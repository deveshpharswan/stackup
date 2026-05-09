# Stackup UI — Terminal User Interface Design Spec

## Overview

`stackup ui` is a k9s-style interactive terminal UI for managing Docker Compose development stacks. It replaces switching between `stackup status`, `stackup logs`, `stackup doctor`, and `docker compose` commands with a single, persistent dashboard that shows real-time service state and lets you act on it.

**Command:** `stackup ui`

**Library:** bubbletea + lipgloss + bubbles (Charm stack)

**Style:** k9s — full-width table, `:` command mode, `/` regex filter, vim-style navigation, breadcrumb view history, header info block with context metadata.

---

## UI Structure

The interface has four persistent regions:

### 1. Header Block (top, 5 lines)

Left column shows stack metadata:
```
Stack:    myproject
Compose:  docker-compose.yml
Tiers:    3
Health:   4/4 ✓
Uptime:   12m 34s
```

Center column shows available keybindings for the active view.

Right column shows an ASCII logo.

### 2. Title Bar (1 line, below header)

Shows current view name, filter state, and item count:
```
Services(all)[4]
```

When a filter is active:
```
Services(/api)[1]
```

### 3. Content Area (fills remaining height)

Renders the active view: table, log viewport, graph, or describe panel.

### 4. Command/Status Bar (bottom, 1 line)

Shows either:
- Command input: `:` prefix for commands, `/` prefix for filter
- Toast messages: brief feedback that auto-dismisses after 3 seconds
- Idle state: shows contextual hint (e.g., "Press ? for help")

---

## Views

### Services View (default)

Full-width table showing all Docker Compose services.

**Columns:** NAME, STATE, HEALTH, PORT, TIER, UPTIME

**Row colors:**
- Green: running + healthy
- Yellow: running + starting (health check pending)
- Red: exited / restarting / unhealthy
- Default (dim white): running + no healthcheck configured

**Selection:** Highlighted row (background color change). Moves with `j/k` or arrow keys.

**Data source:** Docker client polled every 2 seconds via `tea.Tick`. Each tick calls `docker compose ps --format` for state/ports and the Docker SDK for health status.

**Actions on selected service:**
- `r` — restart (with toast confirmation)
- `s` — shell into container (suspends TUI, returns on exit)
- `l` — switch to Logs view for this service
- `x` — stop/delete (shows confirmation modal)
- `Enter` — switch to Describe view for this service

### Logs View

Streaming log viewer for a single service.

**Layout:** Full-height viewport with auto-scroll (follows tail). Timestamps shown in dim gray, log level color-coded (INFO=green, WARN=yellow, ERROR=red, DEBUG=blue).

**Data source:** `docker logs --follow` via goroutine streaming into tea.Cmd channel.

**Controls:**
- `Esc` — back to Services
- `t` — toggle timestamp display
- `w` — toggle line wrap
- `/` — search within logs (highlights matches)
- `c` — clear viewport
- `G` — jump to bottom (latest)
- `g` — jump to top (oldest)
- `PgUp/PgDn` — scroll by page

### Doctor View

Table of diagnostic findings from `stackup doctor` checks.

**Columns:** SEVERITY, CHECK, SERVICE, FINDING, FIX

**Row colors:**
- Red: error severity
- Yellow: warning severity
- Green: ok/pass

**Data source:** Runs all registered doctor checks on view entry. Re-runs on `R` key.

**Controls:**
- `Esc` — back to Services
- `Enter` — expand finding detail (shows full Fix text in a sub-panel)
- `R` — re-run all checks (shows spinner in title bar while running)

### Graph View

ASCII dependency DAG showing tier structure and service relationships.

**Layout:** Left-to-right flow. Tier 1 (leftmost) → Tier N (rightmost). Each service is a box with name and health check type. Arrows show dependency direction.

**Node colors:** Same as Services view (green=healthy, yellow=starting, red=failed).

**Data source:** Computed from `orchestrator.BuildTiers()` and config service dependencies. Static — only re-renders on resize or explicit refresh.

**Controls:**
- `Esc` — back to Services
- `1-9` — highlight specific tier
- `Enter` — select highlighted service, switch to Describe view

### Describe View

Detail panel for a single service (k9s equivalent of `kubectl describe`).

**Sections displayed:**
```
Service: api
Image:   myapp:latest
State:   running (healthy)
Uptime:  12m 34s
Ports:   0.0.0.0:8080 → 8080/tcp

Health Check:
  Type:     http
  URL:      http://localhost:8080/health
  Interval: 2s
  Timeout:  30s

Environment:
  DATABASE_URL=postgres://localhost:5432/mydb
  REDIS_URL=redis://localhost:6379
  LOG_LEVEL=debug

Volumes:
  ./src:/app/src
  pgdata:/var/lib/postgresql/data

Depends On:
  postgres (tier 1)
  redis (tier 1)

Hooks:
  after_start: ["npm run migrate"]
```

**Data source:** Config file (`stackup.yml`) + Docker inspect.

**Controls:**
- `Esc` — back to previous view
- `l` — switch to Logs for this service
- `r` — restart this service

---

## Navigation Model

### Command Mode (`:` prefix)

Typing `:` activates the command input. Available commands:

| Command | Aliases | Action |
|---------|---------|--------|
| `:services` | `:svc` | Switch to Services view |
| `:logs <name>` | `:l <name>` | Switch to Logs view for named service |
| `:doctor` | `:doc` | Switch to Doctor view |
| `:graph` | `:g` | Switch to Graph view |
| `:describe <name>` | `:desc <name>` | Switch to Describe view |
| `:quit` | `:q` | Exit TUI |

Tab completion cycles through matching service names when typing arguments after `:logs` or `:describe`. Partial input narrows candidates (e.g., `:logs ap<Tab>` completes to `:logs api`).

### Filter Mode (`/` prefix)

Typing `/` activates regex filter on the current table view. Filters NAME column. Shows match count in title bar. `Esc` clears filter.

### View History

Views maintain a stack. `Esc` pops back to previous view. Order is always: Services → [Logs|Doctor|Graph|Describe] → back to Services.

---

## Overlays

### Help Overlay (`?`)

Full-screen overlay showing all keybindings for the current view. Organized in two columns. Dismissed with `Esc` or `?`.

### Confirmation Modal

Shown for destructive actions (stop/delete). Centered box:
```
┌─────────────────────────────────┐
│  Stop service "api"?            │
│                                 │
│  This will stop the container.  │
│                                 │
│      [y] Confirm   [n] Cancel   │
└─────────────────────────────────┘
```

Dismissed with `y` (execute) or `n`/`Esc` (cancel).

---

## Data Layer

### Docker Poller

- Runs as a `tea.Tick` command every 2 seconds
- Calls `docker compose ps --format "{{.Service}}\t{{.State}}\t{{.Status}}\t{{.Ports}}"` for state
- Sends `ServiceUpdateMsg` to the model
- Only active when Services view is visible (paused in Logs/Doctor/Graph to reduce load)

### Log Streamer

- Starts when Logs view activates for a service
- Uses existing `docker.Client.Logs(ctx, containerID, follow=true, w)` with a pipe reader
- Sends `LogLineMsg` for each new line
- Cancelled when leaving Logs view (context cancellation)

### Doctor Runner

- Triggered on-demand (entering Doctor view or pressing `R`)
- Runs `doctor.New().Run(ctx, opts)` in a goroutine
- Sends `DoctorResultMsg` with findings
- Shows spinner in title bar while running

---

## Package Structure

```
internal/
  tui/
    tui.go           — Program entry point, root model, top-level Update/View
    header.go        — Header block model (context info, shortcuts, logo)
    command.go       — Command input model (: and / modes, tab completion)
    services.go      — Services table view model
    logs.go          — Logs viewport view model
    doctor.go        — Doctor findings table view model
    graph.go         — Dependency graph renderer
    describe.go      — Service describe panel model
    help.go          — Help overlay model
    confirm.go       — Confirmation modal model
    toast.go         — Toast notification model
    styles.go        — Shared lipgloss styles (colors, borders, layout)
    keys.go          — Keybinding definitions per view
    messages.go      — All tea.Msg types (ServiceUpdateMsg, LogLineMsg, etc.)
cmd/
  ui.go             — Cobra command wiring (`stackup ui`)
```

Each file is one focused model following bubbletea's Model interface: `Init()`, `Update(tea.Msg)`, `View() string`.

---

## Shell Action

When the user presses `s` for shell:

1. TUI suspends (bubbletea's `tea.ExecProcess`)
2. Runs `docker compose exec -it <service> sh` (universal POSIX shell — works in alpine, debian, etc.)
3. On shell exit, TUI resumes automatically

This is bubbletea's built-in mechanism for handing control to a child process.

---

## Terminal Requirements

- Minimum terminal size: 80 columns x 24 rows
- If terminal is too small, shows a "Terminal too small" message with required dimensions
- Responds to resize events (bubbletea `tea.WindowSizeMsg`)
- Respects `NO_COLOR` environment variable (disables all styling)
- Works in Windows Terminal, iTerm2, standard Linux terminals

---

## Error Handling

- If Docker daemon is unreachable: show error in content area with fix suggestion ("Is Docker running?")
- If a service disappears between polls: remove from table, show toast "Service 'x' removed"
- If log streaming disconnects: show "Connection lost, reconnecting..." in viewport, auto-retry
- If doctor check panics: catch, show "Check failed" in findings, don't crash TUI

---

## Not in Scope (v1)

- CPU/MEM resource metrics (requires Docker stats streaming — fast-follow)
- Mouse support (keyboard-only for v1)
- Custom themes/skins
- Plugin system
- Multi-compose file support
- Network view
- Image management
