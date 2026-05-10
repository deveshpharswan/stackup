# Stackup TUI v2 — 2-Panel Redesign

## Goal

Replace the current single-panel services table with a 2-panel lazydocker-style layout. Left panel: navigable service list grouped by tier. Right panel: selected service detail (state, health, CPU/memory sparklines, ports, dependencies) plus a live log tail. Preserve all existing views (Logs, Doctor, Graph, Describe) and key bindings. No features removed.

---

## Layout

```
┌─ STACKUP │ project │ ● N healthy  ◐ N starting  ✗ N failed │ compose.yaml  uptime:Xm ──┐
├──────────────────────┬───────────────────────────────────────────────────────────────────┤
│  Services  N/N       │  service-name — state (health)                                    │
│ ── tier 1 ────────── │                                                                   │
│  ● db        2m12s   │  State:    running (healthy)                                      │
│  ● redis     2m11s   │  Health:   http GET /health (14ms)                                │
│ ── tier 2 ────────── │  CPU:      ▁▂▃▂▁▃▂▁▃▂▃▄▃▂▁▃  0.4%                              │
│  ● api  ◀    2m08s   │  Memory:   ▃▃▄▃▃▄▄▃▃▄▃▃▄▃▃▄  86MB                               │
│ ── tier 3 ────────── │  Ports:    0.0.0.0:8080 → 8080/tcp                               │
│  ● nginx     2m01s   │  Uptime:   2m08s                                                 │
│  ◐ worker    14s     │  Depends:  db, redis                                             │
│                       │                                                                   │
│                       │  ─── Logs ───────────────────────────────────────────────────     │
│                       │  09:41 GET /health → 200                                         │
│                       │  09:41 POST /api/orders → 201                                    │
│                       │  09:41 GET /api/v1/users → 200                                   │
├──────────────────────┴───────────────────────────────────────────────────────────────────┤
│  j/k:nav  enter/l:logs  r:restart  s:shell  x:stop  d:doctor  g:graph  /:filter  ?:help │
└──────────────────────────────────────────────────────────────────────────────────────────┘
```

### Responsive Breakpoints

| Terminal Width | Left Panel | Right Panel |
|---|---|---|
| < 80 cols | "Terminal too small" message | — |
| 80–99 cols | hidden | Full-width services table (current behavior) |
| ≥ 100 cols | 24 chars | Remaining width (detail + log tail) |

When the left panel is hidden (80–99 cols), the Services view falls back to the existing table rendering in `ServicesModel.View()`.

---

## Header (1 line, already implemented)

```
STACKUP │ project-name │ ● N healthy  ◐ N starting  ✗ N failed │ compose.yaml  tiers:N  uptime:Xm
```

Reuse existing `HeaderModel` as-is.

---

## Left Panel — Service List

Reuse existing `SidebarModel` with modifications:

- Panel header: `Services  N/N` (blue, bold)
- Services grouped by tier with `── tier N ──` dim dividers
- Each row: status dot + name + uptime
- Selected row: green left border + highlight background
- Scrollable when services exceed panel height
- Navigate: `j`/`k` or `↑`/`↓`

### Status Symbols (unchanged)
```
●   running + healthy       (green)
◐   running + starting/none (yellow)
✗   exited / restarting     (red)
◌   not started             (dim)
```

---

## Right Panel — Service Detail

Split into two sections:

### Top: Service Overview

For the selected service, display:
- **Name + state + health** — header line with status-colored name
- **Health check** — type + endpoint/pattern from stackup.yaml config
- **CPU** — sparkline (16 samples) + current percentage
- **Memory** — sparkline (16 samples) + current percentage
- **Ports** — host:port → container:port mappings (from `docker compose ps` output)
- **Uptime** — formatted duration
- **Depends on** — service names from compose `depends_on`

### Bottom: Live Log Tail

- Last ~8-12 lines from `docker compose logs -f --tail 10 <service>`
- Updates in real-time as new lines arrive
- Timestamps stripped for compactness (full logs accessible via `enter`/`l`)
- Color-coded: errors=red, warnings=yellow, debug=blue

---

## Footer Hint Bar (1 line)

Context-sensitive keyboard hints:

**Services view:**
```
j/k:nav  enter/l:logs  r:restart  s:shell  x:stop  e:errors  d:doctor  g:graph  /:filter  :cmd  ?:help  q:quit
```

**Full-screen views (Logs, Doctor, Graph, Describe):**
```
esc:back  [view-specific hints from help.go]
```

---

## View Navigation (unchanged model)

Keep the existing view-stack (`pushView`/`popView`) approach:

| From | Key | To |
|---|---|---|
| Services | `enter` or `l` | Logs (full-screen for selected service) |
| Services | `d` | Doctor (full-screen) |
| Services | `g` | Graph (full-screen) |
| Any | `esc` | Pop back to Services |
| Any | `?` | Help overlay |
| Services | `q` | Quit |

Full-screen views (Logs, Doctor, Graph, Describe) render exactly as they do today — no changes to those view models.

---

## Key Bindings (preserved from current)

### Services View
| Key | Action | Status |
|---|---|---|
| `↑`/`k` | Move cursor up | Existing |
| `↓`/`j` | Move cursor down | Existing |
| `enter` | Describe service | Existing |
| `l` | View logs for service | Existing |
| `r` | Restart service (confirm) | Existing |
| `s` | Shell into container | Existing |
| `x` | Stop service (confirm) | Existing |
| `e` | Error zoom | Existing |
| `d` | Doctor view | Existing |
| `g` | Graph view | Existing |
| `/` | Filter services | Existing |
| `:` | Command mode | Existing |
| `?` | Help overlay | Existing |
| `q` | Quit | Existing |

### Other views — unchanged from current `help.go` definitions.

---

## Architecture

### Modified Files
| File | Change |
|---|---|
| `internal/tui/tui.go` | Add `SidebarModel` + `logTail LogsModel` to `Model`; update `View()` to render 2-panel when width ≥ 100; delegate j/k to sidebar when active; start log tail on selection change |
| `internal/tui/layout.go` | Simplify to 2-panel (remove 3-panel right panel logic); adjust constants |
| `internal/tui/sidebar.go` | Already implemented. Minor: emit `SidebarSelectionMsg` on cursor change so tui.go can update detail + log tail |
| `internal/tui/services.go` | Keep as-is for narrow-terminal fallback. Remove j/k handling (sidebar handles it now when wide) |
| `internal/tui/stats.go` | Increase `sparklineLen` from 8 to 16 |
| `internal/tui/header.go` | Already implemented (slim 1-line). No changes. |
| `internal/tui/tabs.go` | Remove (not needed for 2-panel; replaced by view-stack) |

### New Files
| File | Responsibility |
|---|---|
| `internal/tui/detail.go` | Right panel: renders service overview + log tail. Receives `ServiceInfo`, `StatsHistory`, config data. |

### Removed Files
| File | Reason |
|---|---|
| `internal/tui/tabs.go` | Tab bar not used in 2-panel layout; view navigation uses existing view-stack |

### State Model Changes

```go
type Model struct {
    width  int
    height int

    activeView ViewType    // keep existing view-stack
    viewStack  []ViewType  // keep existing

    services ServicesModel // data source (polling, filter, stats)
    sidebar  SidebarModel  // left panel navigation (wide terminals)
    detail   DetailModel   // right panel rendering
    logTail  LogsModel     // small log preview in right panel

    logs     LogsModel       // full-screen logs (existing)
    doctor   DoctorViewModel // existing
    graph    GraphModel      // existing
    describe DescribeModel   // existing

    header  HeaderModel   // existing (slim)
    command CommandModel   // existing
    toast   ToastModel     // existing
    help    HelpModel      // existing
    confirm ConfirmModel   // existing

    showHelp    bool
    showConfirm bool
    quitting    bool
}
```

---

## Detail Panel Internals

```go
type DetailModel struct {
    service      ServiceInfo
    statsHistory map[string]*StatsHistory
    deps         []string // from compose file
    healthDesc   string   // from stackup.yaml
}

func (m DetailModel) View(width, height int, logTailView string) string
```

The detail panel is purely a renderer — no tea.Cmd, no own Update logic. It receives data from `tui.go` and formats it.

---

## Log Tail Behavior

- Uses existing `LogsModel` infrastructure (reuse `startLogStream`)
- Starts streaming for the selected service when cursor moves
- On sidebar selection change: stop old stream, start new one
- Displays last N lines that fit in the available height (bottom half of right panel)
- No scroll support (that's what full-screen Logs is for)
- Timestamps hidden for compactness

---

## What Does NOT Change

- `LogsModel` — full-screen log viewer, all keys (t, w, c, g, G, PgUp, PgDn)
- `DoctorViewModel` — full-screen diagnostics, all keys (j, k, enter, R)
- `GraphModel` — full-screen dependency graph, all keys (0-9)
- `DescribeModel` — full-screen service description
- `CommandModel` — `:` command mode with aliases
- `ConfirmModel` — confirmation modals (restart, stop)
- `ToastModel` — toast notifications
- `HelpModel` — help overlay (will add sidebar-specific hints)
- All message types in `messages.go`
- Stats polling (`pollStats`, `StatsHistory`)
- Service polling (`pollServices`)
- `Run()` function — entry point unchanged
- Color palette and style definitions

---

## Out of Scope

- Mouse support
- Numbered tabs (1–5 switching)
- Third panel (right log panel from old spec)
- Environment variable inspector
- Volume/port sub-tabs
- Bulk stack down (`D`)
- Start service (`u`)
- These can be added later as incremental features

---

## Testing

1. Build compiles: `go build ./...`
2. Existing tests pass: `go test ./internal/tui/...`
3. Manual verification at 80×24 (narrow fallback), 120×30 (2-panel), 160×40 (wide 2-panel)
4. Verify all existing key bindings still work
5. Verify full-screen views (Logs, Doctor, Graph, Describe) render correctly
6. Verify log tail updates when cursor moves between services
