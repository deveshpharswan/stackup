# Stackup TUI Redesign — Design Spec

## Goal

Replace the current 5-line-header, single-panel TUI with a full-screen 3-panel layout inspired by lazydocker and k9s. Fix all four identified pain points (density, confusing navigation, dated appearance, missing information) and add nine new features.

## Pain Points Addressed

| Pain Point | Current | Fix |
|---|---|---|
| Header overhead | 5-line header + ASCII logo | 1-line slim header with status badges |
| Confusing navigation | Shortcuts buried in header | Numbered tabs (1–5) + footer hint bar |
| Dated appearance | Plain text table, no visual hierarchy | Tier separators, color badges, panels |
| Missing information | No logs/env/ports/volumes in main view | 3-panel layout with always-visible detail |

---

## Layout: Option C — Hybrid 3-Panel

```
┌─ STACKUP │ my-project │ ● 4 healthy  ◐ 1 starting  │ compose.yaml  tiers:3  uptime:2m34s ── master ─┐  (1 line)
├─ [1] Services  [2] Logs  [3] Stats  [4] Doctor  [5] Graph ─────────────────────────────────────────────┤  (tab bar)
│                                                                                                          │
│  ┌─ Services ──────────┐  ┌─ api — details ● healthy · http ──────────────────┐  ┌─ Live Logs  api ──┐ │
│  │ ── tier 1 ──────── │  │  CPU  ▁▁▂▁▃▂▁▂  0.4%                              │  │ 09:41 GET /health │ │
│  │ ● db       2m12s   │  │  MEM  ▃▃▄▃▃▄▄▃  86MB / 512MB                      │  │ 09:41 GET /api/v1 │ │
│  │ ● redis    2m11s   │  │  Port  0.0.0.0:8080 → 8080/tcp                     │  │ 09:41 POST /order │ │
│  │ ── tier 2 ──────── │  │  Health  ✓ http 200 (14ms)                         │  ├───────────────────┤ │
│  │ ● api  ◀   2m08s   │  │                                                     │  │ Startup Progress  │ │
│  │ ── tier 3 ──────── │  │  NAME     STATE    HEALTH    CPU   MEM   UPTIME     │  │ db   ████████ ✓   │ │
│  │ ● nginx    2m01s   │  │  ● db     running  ✓ tcp    1.2%  110M  2m12s      │  │ api  ████████ ✓   │ │
│  │ ◐ worker   14s     │  │  ● api ▶  running  ✓ http   0.4%   86M  2m08s      │  │ worker ████░░ …   │ │
│  └─────────────────────┘  └────────────────────────────────────────────────────┘  └───────────────────┘ │
│                                                                                                          │
└─ ↑↓:navigate  enter:focus  1–5:tabs  r:restart  s:stop  u:up  x:shell  D:stack-down  /:filter  ?:help  q:quit ─┘
```

### Panel Widths (responsive)

| Terminal width | Left panel | Center panel | Right panel |
|---|---|---|---|
| < 100 cols | hidden (collapsed) | full width | hidden |
| 100–139 cols | 22 chars | remaining | hidden |
| ≥ 140 cols | 22 chars | remaining | 36 chars |

When the right panel is hidden, the startup progress section moves to the bottom of the center panel.

### Panel Descriptions

**Left — Services sidebar** (22 chars wide)
- Lists all services grouped by tier with `── tier N ──` dividers
- Each row: status dot · name · uptime
- Selected row: green left border + bold name
- Starting service: yellow dot and text
- Scrollable if services exceed panel height

**Center — Detail + table** (fills remaining space)
- Top section: selected service detail (CPU sparkline, MEM sparkline, port, health check result, container image, restart policy)
- Bottom section: mini services table (NAME, STATE, HEALTH, CPU, MEM, UPTIME, ACTIVITY) — same data as current table but embedded in the panel
- Selected row highlighted in mini table

**Right — Live logs + startup progress** (36 chars wide)
- Top: live log tail for the selected service, newest lines at bottom
- Bottom: startup progress bars (shown during boot; hidden once all services healthy)
- When all services healthy, the startup section collapses and logs take full right panel height

---

## Header Redesign

### Current (5 lines)
```
Stack:   my-project          [shortcuts col]       ╔═══════╗
Compose: compose.yaml                              ║STACKUP║
Tiers:   3                                         ╚═══════╝
Health:  4/5 [✓]
Uptime:  2m34s
```

### New (1 line)
```
STACKUP │ my-project │ ● 4 healthy  ◐ 1 starting  │ compose.yaml  tiers:3  uptime:2m34s ─── master
```

Elements:
- `STACKUP` — bold blue, fixed brand mark
- Project name from working directory
- Status badges: `● N healthy` (green), `◐ N starting` (yellow), `✗ N failed` (red) — only shown when count > 0
- Compose filename, tier count, uptime — dim gray
- Git branch (if detectable) — right-aligned

---

## Tab Bar

Replaces the view stack navigation. Fixed 5 tabs:

| Key | Tab | Content |
|---|---|---|
| `1` | Services | 3-panel layout (default view) |
| `2` | Logs | Full-screen log viewer for selected service |
| `3` | Stats | CPU/memory graphs over time |
| `4` | Doctor | Diagnostic checks (existing) |
| `5` | Graph | Dependency graph (existing) |

Active tab: blue text + bottom border highlight. Inactive tabs: dim gray.

---

## Detail Panel Tabs

Within the center panel, `tab` / `shift+tab` cycle through sub-panels for the selected service:

| Sub-panel | Content |
|---|---|
| **Overview** (default) | CPU sparkline, MEM sparkline, port mapping, health check, image, restart |
| **Log Viewer** | Full scrollable log history for the selected service with `/` filter. Distinct from Tab 2 (Logs), which is the full-screen log view without side panels. |
| **Env** | Environment variables; values masked with `****`, press `v` to reveal |
| **Ports** | Host→container port mappings; conflict indicator if host port is bound elsewhere |
| **Volumes** | Volume binds: host path, container path, mode (rw/ro) |
| **Config** | Raw compose service definition (read-only) |

---

## New Features

### 1. Shell Exec — `x`
- Press `x` on any running service
- Opens `docker exec -it <container> sh` (falls back to `bash` if `sh` unavailable)
- Suspends Bubble Tea, hands terminal to exec, resumes TUI on exit
- Only available when service state is `running`; shows error toast otherwise

### 2. Start Stopped Service — `u`
- Press `u` on a stopped/exited service
- Runs `docker compose up -d <service>`
- Shows spinner toast until service is running

### 3. Live Log Filter — `/` in Logs view
- Press `/` to open an inline filter input at the bottom of the log panel
- Lines not matching the filter are hidden; matches are highlighted (yellow background)
- `esc` clears filter and returns to unfiltered view
- Filter persists when switching services until cleared

### 4. Environment Variable Inspector
- Accessible via detail panel `Env` sub-tab
- All values rendered as `****` by default
- Press `v` while cursor is on a row to reveal that value
- Press `V` to reveal/mask all values

### 5. Port Mapping Viewer
- Accessible via detail panel `Ports` sub-tab
- Shows each port binding: `host:port → container:port/proto`
- If the host port is already bound by another process, shows a red `⚠ conflict` indicator
- Conflict detection: attempt `net.DialTimeout("tcp", "localhost:PORT", 200ms)`; if it succeeds the port is in use by something other than this container

### 6. Volume Mounts Panel
- Accessible via detail panel `Volumes` sub-tab
- Table: Source (host path or named volume), Target (container path), Mode (rw/ro)
- Named volumes shown in blue; bind mounts shown in white

### 7. Animated Startup Sequence
- During `stackup up`, the Services tab shows a dedicated startup progress view
- Each service row: spinner animation → progress bar filling → `✓ done` or `✗ failed`
- Health check type shown: `http`, `tcp`, `log pattern`
- Elapsed time shown per service
- On completion, transitions to normal 3-panel layout

### 8. Toast Notifications
- Positioned in the bottom-right corner, above the footer
- Triggers: service goes down, service recovers, health check fails, restart loop detected
- Colors: red (down/failed), yellow (warning/restarting), green (recovered)
- Auto-dismiss after 3 seconds; multiple toasts stack vertically
- Implementation: extend existing `toast.go`

### 9. Bulk Stack Down — `D` (capital)
- Press `D` from any view
- Confirmation modal: "Bring down all 5 services?" with list of service names
- `enter` confirms, `esc` cancels
- Runs `docker compose down`
- Existing confirmation modal in `tui.go` can be reused

---

## Keyboard Map

### Global (all views)
| Key | Action |
|---|---|
| `1`–`5` | Switch to numbered tab |
| `?` | Toggle help overlay |
| `q` | Quit |
| `D` | Stack down (confirm) |

### Services tab
| Key | Action |
|---|---|
| `↑` / `↓` | Navigate service list |
| `enter` | Focus selected service detail |
| `r` | Restart selected service |
| `s` | Stop selected service |
| `u` | Start selected service |
| `x` | Shell exec into selected service |
| `/` | Filter services by name |
| `e` | Toggle error zoom (show only unhealthy) |
| `tab` | Cycle detail sub-panels |
| `v` | Reveal env values (when Env sub-panel active) |

### Logs tab / Logs sub-panel
| Key | Action |
|---|---|
| `↑` / `↓` | Scroll |
| `g` / `G` | Jump to top / bottom |
| `/` | Open log filter |
| `esc` | Clear filter / exit sub-panel |

### All modals
| Key | Action |
|---|---|
| `enter` | Confirm |
| `esc` | Cancel |

---

## Visual Design

### Color Palette (unchanged from current)
```
Green:   #7ee787   running, healthy, success
Yellow:  #d29922   starting, warning
Red:     #f85149   failed, stopped, error
Blue:    #58a6ff   selected, info, shortcuts
Dim:     #484f58   secondary text, metadata
White:   #c9d1d9   primary text
Border:  #30363d   panel borders
Dark bg: #161b22   panel backgrounds, selected rows
```

### Status Symbols
```
●   running + healthy       (green)
◐   running + starting      (yellow)
✗   exited / failed         (red)
⏸   paused                  (yellow)
◌   not started             (dim)
```

### Tier Separators
```
── tier 1 ─────────────
```
Rendered in dim gray (`#484f58`). Separate logical tiers visually in both the sidebar and the mini table.

### Sparklines
Extended from 8 chars to 16 chars minimum (scales with panel width). One sample per second. Both CPU and MEM tracked.

---

## Architecture

### New Files
| File | Responsibility |
|---|---|
| `internal/tui/layout.go` | Computes 3-panel layout dimensions from terminal size; handles responsive breakpoints |
| `internal/tui/sidebar.go` | Left panel: service list with tier grouping, selection, scrolling |
| `internal/tui/detail.go` | Center panel: service detail header + mini table + sub-panel tabs |
| `internal/tui/tabs.go` | Top tab bar rendering and tab-switch key handling |
| `internal/tui/progress.go` | Startup progress animation: per-service bars, spinners, health check display |
| `internal/tui/exec.go` | Shell exec: suspend TUI, run docker exec, resume |

### Modified Files
| File | Change |
|---|---|
| `internal/tui/tui.go` | Wire new panels into model; replace view stack with tab model; handle `WindowSizeMsg` for responsive layout |
| `internal/tui/header.go` | Replace 5-line header with 1-line slim header |
| `internal/tui/styles.go` | Add new styles: tab bar, tier divider, badge, progress bar, detail panel |
| `internal/tui/services.go` | Refactor into sidebar (list) and mini-table (embedded in detail panel) |
| `internal/tui/logs.go` | Add live filter (`/`), scroll-to-bottom behavior, keep log tail for right panel |
| `internal/tui/keys.go` | Add new key bindings: `u`, `x`, `D`, `1`–`5`, `v`, `V` |
| `internal/tui/toast.go` | Add stacking toasts, event-driven triggers |

### State Model Changes
```go
// Current: view stack
type Model struct {
    viewStack []ViewType
    ...
}

// New: tab + panel model
type Model struct {
    activeTab    TabType          // services, logs, stats, doctor, graph
    detailTab    DetailTabType    // overview, logs, env, ports, volumes, config
    sidebarModel sidebar.Model
    detailModel  detail.Model
    logModel     logs.Model
    startupModel progress.Model
    ...
}
```

---

## Out of Scope

- Mouse support (keyboard-only, consistent with current)
- Light/dark theme switching (GitHub dark stays as-is)
- Config file editing within the TUI
- Port forwarding
- Docker Compose file editing
- Multi-compose-file support changes (unrelated)

---

## Testing

The existing E2E tests cover CLI behavior (not TUI rendering). No TUI-specific tests exist. After implementation:

1. Manual test checklist: all 9 features exercised on the `tests/e2e/testdata/multi-tier` fixture
2. Responsive layout: test at 80×24, 120×30, 160×40
3. Startup animation: test with `multi-tier` fixture (three tiers, different health check types)
4. Error states: test with a service that fails health check, a stopped service
5. Shell exec: test `x` on running nginx container (`sh` available)
