# Stackup TUI Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Stackup's current 5-line-header single-panel TUI with a full-screen 3-panel layout (sidebar + detail + log tail) inspired by lazydocker and k9s, adding 9 new features.

**Architecture:** Keep all code in `package tui`. Add new files for layout, sidebar, detail, tabs, and progress. Modify existing tui.go (model wiring), header.go (slim header), services.go (key remaps), logs.go (filter), toast.go (stacking), and stats.go (longer sparklines). Keep `ServicesModel` in `Model` for data polling and filter state; `SidebarModel` is display-only, receiving filtered `[]ServiceInfo` from `ServicesModel`. The Docker client `dc *dockerclient.Client` already exists in `ServicesModel` and is passed to `fetchInspect`.

**Tech Stack:** Go 1.21, Bubble Tea (`github.com/charmbracelet/bubbletea`), Lipgloss (`github.com/charmbracelet/lipgloss`), Bubbles viewport (`github.com/charmbracelet/bubbles/viewport`), Docker SDK (`github.com/docker/docker`).

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `internal/tui/header.go` | Modify | Replace 5-line header with 1-line slim header |
| `internal/tui/tabs.go` | Create | `TabType` enum + tab bar renderer |
| `internal/tui/layout.go` | Create | `PanelLayout` struct + `ComputeLayout()` |
| `internal/tui/sidebar.go` | Create | Left panel: service list with tier groupings |
| `internal/tui/detail.go` | Create | Center panel: overview + sub-tabs (env/ports/volumes/log viewer) |
| `internal/tui/progress.go` | Create | Right panel bottom: startup progress bars |
| `internal/tui/tui.go` | Modify | Wire new models; replace viewStack; update tests |
| `internal/tui/services.go` | Modify | Remap keys (s→stop, x→shell), add u→start |
| `internal/tui/logs.go` | Modify | Add `/` live filter |
| `internal/tui/toast.go` | Modify | Stacking multi-message toasts |
| `internal/tui/stats.go` | Modify | Dynamic sparkline length |
| `internal/tui/messages.go` | Modify | Add new message types |
| `internal/tui/confirm.go` | Modify | Add `ConfirmStackDown` action |
| `internal/tui/tui_test.go` | Modify | Update tests for new tab model |

---

## Task 1: Slim Header

Replace the 5-line header + ASCII logo with a single-line header showing project name, status badges, compose file, and uptime.

**Files:**
- Modify: `internal/tui/header.go`

- [ ] **Step 1: Update `HeaderModel.View()` to render 1 line**

Replace the entire `View` method body (lines 59–91) with:

```go
func (m HeaderModel) View(width int, active TabType) string {
	uptime := time.Since(m.startTime).Truncate(time.Second)

	logo := styleInfo.Bold(true).Render("STACKUP")
	sep := styleDim.Render(" │ ")
	project := styleBold.Render(m.stack)

	var badges []string
	if m.healthy > 0 {
		badges = append(badges, styleHealthy.Render(fmt.Sprintf("● %d healthy", m.healthy)))
	}
	starting := m.total - m.healthy - m.failed
	if starting > 0 {
		badges = append(badges, styleWarning.Render(fmt.Sprintf("◐ %d starting", starting)))
	}
	if m.failed > 0 {
		badges = append(badges, styleFailed.Render(fmt.Sprintf("✗ %d failed", m.failed)))
	}

	meta := styleDim.Render(fmt.Sprintf("%s  tiers:%s  uptime:%s",
		m.compose, tierStr(m.tiers), formatUptime(uptime)))

	left := logo + sep + project
	if len(badges) > 0 {
		left += sep + strings.Join(badges, "  ")
	}
	left += sep + meta

	return styleHeader.Width(width).Render(left)
}
```

- [ ] **Step 2: Add `failed int` field and `tierStr` helper to `HeaderModel`**

In `header.go`, add `failed int` to the `HeaderModel` struct and add the helper:

```go
type HeaderModel struct {
	stack     string
	compose   string
	tiers     int
	healthy   int
	total     int
	failed    int       // NEW
	startTime time.Time
}

func tierStr(n int) string {
	if n == 0 {
		return "—"
	}
	return fmt.Sprintf("%d", n)
}
```

Update the `Update` method to count failed services:

```go
case ServiceUpdateMsg:
	if msg.Err == nil {
		m.total = len(msg.Services)
		healthy, failed := 0, 0
		for _, s := range msg.Services {
			switch {
			case s.Health == "healthy":
				healthy++
			case s.State == "exited" || s.State == "restarting":
				failed++
			}
		}
		m.healthy = healthy
		m.failed = failed
	}
```

- [ ] **Step 3: Update `View` signature** — change `active ViewType` parameter to `active TabType`

```go
func (m HeaderModel) View(width int, active TabType) string {
```

(The `active` parameter is unused in the slim header but kept for future use.)

- [ ] **Step 4: Build to verify compilation**

```bash
go build ./internal/tui/...
```

Expected: compile error about `TabType` not yet defined. That's OK — proceed to Task 2.

- [ ] **Step 5: Commit once Task 2 is done (combined commit)**

---

## Task 2: Tab Bar

Add `TabType` enum and a tab bar renderer. This replaces the `ViewType`-based navigation.

**Files:**
- Create: `internal/tui/tabs.go`

- [ ] **Step 1: Create `internal/tui/tabs.go`**

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type TabType int

const (
	TabServices TabType = iota
	TabLogs
	TabStats
	TabDoctor
	TabGraph
)

var tabLabels = []string{"Services", "Logs", "Stats", "Doctor", "Graph"}

func renderTabBar(width int, active TabType) string {
	var parts []string
	for i, label := range tabLabels {
		num := styleDim.Render(fmt.Sprintf("%d", i+1))
		if TabType(i) == active {
			tab := lipgloss.NewStyle().
				Background(lipgloss.Color("#161b22")).
				Foreground(colorBlue).
				Bold(true).
				BorderBottom(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(colorBlue).
				Padding(0, 1).
				Render(num + " " + label)
			parts = append(parts, tab)
		} else {
			tab := lipgloss.NewStyle().
				Foreground(colorDim).
				Padding(0, 1).
				Render(num + " " + label)
			parts = append(parts, tab)
		}
	}
	bar := strings.Join(parts, "")
	return lipgloss.NewStyle().
		Background(lipgloss.Color("#0d1117")).
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorBorder).
		Width(width).
		Render(bar)
}
```

- [ ] **Step 2: Build to verify `TabType` is now defined**

```bash
go build ./internal/tui/...
```

Expected: compiles (header.go uses `TabType` now), or remaining errors from tui.go using old `ViewType`. Those will be fixed in Task 7.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/header.go internal/tui/tabs.go
git commit -m "feat(tui): slim 1-line header + tab bar"
```

---

## Task 3: Layout Engine

Compute 3-panel dimensions from terminal width/height.

**Files:**
- Create: `internal/tui/layout.go`

- [ ] **Step 1: Create `internal/tui/layout.go`**

```go
package tui

const (
	headerLines  = 2  // slim header (1) + tab bar (1)
	footerLines  = 1  // footer hint bar
	sidebarWidth = 22 // chars, fixed
	rightWidth   = 36 // chars, fixed
	minWidth     = 80
	minHeight    = 24
)

// PanelLayout holds the computed dimensions for all panels.
type PanelLayout struct {
	Width  int
	Height int

	HeaderHeight  int
	FooterHeight  int
	ContentHeight int // Height - HeaderHeight - FooterHeight

	HasSidebar   bool
	SidebarWidth int

	HasRight   bool
	RightWidth int

	CenterWidth int // Width - SidebarWidth (if shown) - RightWidth (if shown)
}

// ComputeLayout returns panel dimensions for a given terminal size.
func ComputeLayout(width, height int) PanelLayout {
	l := PanelLayout{
		Width:         width,
		Height:        height,
		HeaderHeight:  headerLines,
		FooterHeight:  footerLines,
		ContentHeight: height - headerLines - footerLines,
	}
	// Sidebar visible when terminal is wide enough
	if width >= 100 {
		l.HasSidebar = true
		l.SidebarWidth = sidebarWidth
	}
	// Right panel visible when terminal is wide enough
	if width >= 140 {
		l.HasRight = true
		l.RightWidth = rightWidth
	}
	used := 0
	if l.HasSidebar {
		used += l.SidebarWidth
	}
	if l.HasRight {
		used += l.RightWidth
	}
	l.CenterWidth = width - used
	if l.CenterWidth < 20 {
		l.CenterWidth = 20
	}
	return l
}
```

- [ ] **Step 2: Build**

```bash
go build ./internal/tui/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/layout.go
git commit -m "feat(tui): layout engine for 3-panel dimensions"
```

---

## Task 4: Services Sidebar

Left panel: scrollable service list with tier-dividers and status dots.

**Files:**
- Create: `internal/tui/sidebar.go`

- [ ] **Step 1: Create `internal/tui/sidebar.go`**

```go
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SidebarModel struct {
	services []ServiceInfo
	cursor   int
	offset   int
}

func NewSidebarModel() SidebarModel {
	return SidebarModel{}
}

func (m SidebarModel) SetServices(services []ServiceInfo) SidebarModel {
	m.services = services
	if m.cursor >= len(services) && len(services) > 0 {
		m.cursor = len(services) - 1
	}
	return m
}

func (m SidebarModel) Selected() string {
	if m.cursor < len(m.services) {
		return m.services[m.cursor].Name
	}
	return ""
}

func (m SidebarModel) SelectedInfo() *ServiceInfo {
	if m.cursor < len(m.services) {
		s := m.services[m.cursor]
		return &s
	}
	return nil
}

func (m SidebarModel) Update(msg tea.Msg) (SidebarModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.services)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		}
	case ServiceUpdateMsg:
		if msg.Err == nil {
			prev := m.Selected()
			m.services = msg.Services
			// Restore cursor to same service name if possible
			for i, s := range m.services {
				if s.Name == prev {
					m.cursor = i
					return m, nil
				}
			}
			if m.cursor >= len(m.services) && len(m.services) > 0 {
				m.cursor = len(m.services) - 1
			}
		}
	}
	return m, nil
}

func (m SidebarModel) View(width, height int) string {
	if len(m.services) == 0 {
		return styleDim.Render("  No services")
	}

	// Scroll window
	visible := height - 2 // subtract header line
	if visible < 1 {
		visible = 1
	}
	// Adjust offset to keep cursor visible
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
	}

	var b strings.Builder
	// Panel header
	b.WriteString(lipgloss.NewStyle().
		Background(lipgloss.Color("#0f1117")).
		Foreground(colorBlue).
		Bold(true).
		Width(width).
		Padding(0, 1).
		Render(fmt.Sprintf("Services  %d/%d", len(m.services), len(m.services))) + "\n")

	currentTier := -1
	rendered := 0
	for i, svc := range m.services {
		// Tier divider
		if svc.Tier != currentTier && svc.Tier > 0 {
			currentTier = svc.Tier
			divider := styleDim.Width(width).Render(fmt.Sprintf(" tier %d ", svc.Tier))
			b.WriteString(divider + "\n")
		}
		if i < m.offset || rendered >= visible {
			continue
		}
		rendered++

		dot, nameStyle, uptimeStr := svcStatusParts(svc)
		name := nameStyle.Render(truncate(svc.Name, width-10))
		uptime := styleDim.Render(truncate(uptimeStr, 7))

		row := fmt.Sprintf(" %s %-*s %s", dot, width-12, name, uptime)

		if i == m.cursor {
			b.WriteString(lipgloss.NewStyle().
				Background(lipgloss.Color("#161b22")).
				BorderLeft(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(colorGreen).
				Width(width).
				Render(row) + "\n")
		} else {
			b.WriteString(lipgloss.NewStyle().Width(width).Render(row) + "\n")
		}
	}
	return b.String()
}

// svcStatusParts returns dot symbol, name style, and uptime string for a service.
func svcStatusParts(svc ServiceInfo) (string, lipgloss.Style, string) {
	uptime := ""
	if svc.Uptime > 0 {
		uptime = formatUptime(svc.Uptime)
	}
	switch {
	case svc.State == "running" && svc.Health == "healthy":
		return styleHealthy.Render("●"), styleHealthy, uptime
	case svc.State == "running":
		return styleWarning.Render("◐"), styleWarning, uptime
	case svc.State == "exited" || svc.State == "restarting":
		return styleFailed.Render("✗"), styleFailed, uptime
	default:
		return styleDim.Render("◌"), styleDim, uptime
	}
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
```

- [ ] **Step 2: Build**

```bash
go build ./internal/tui/...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/sidebar.go
git commit -m "feat(tui): services sidebar panel"
```

---

## Task 5: Detail Overview Panel

Center panel: selected service details (sparklines, port, health check) + mini services table.

**Files:**
- Create: `internal/tui/detail.go`

- [ ] **Step 1: Create `internal/tui/detail.go` with `DetailModel` and `DetailTab` types**

```go
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	"github.com/deveshpharswan/stackup/internal/config"
	"github.com/deveshpharswan/stackup/internal/constants"
)

type DetailTab int

const (
	DetailTabOverview DetailTab = iota
	DetailTabLogViewer
	DetailTabEnv
	DetailTabPorts
	DetailTabVolumes
	DetailTabConfig
)

var detailTabLabels = []string{"overview", "logs", "env", "ports", "volumes", "config"}

type InspectData struct {
	Env   []string // "KEY=value" format
	Binds []string // "/host:/container:rw" format
	Ports []PortMapping
}

type PortMapping struct {
	HostPort      string
	ContainerPort string
	Proto         string
	Conflict      bool
}

type DetailModel struct {
	service      ServiceInfo
	services     []ServiceInfo
	statsHistory map[string]*StatsHistory
	tab          DetailTab
	inspect      *InspectData
	envMasked    bool
	viewport     viewport.Model
	ready        bool
}

func NewDetailModel() DetailModel {
	return DetailModel{envMasked: true}
}

func (m DetailModel) SetService(svc ServiceInfo, services []ServiceInfo, history map[string]*StatsHistory) DetailModel {
	m.service = svc
	m.services = services
	m.statsHistory = history
	m.ready = true
	m.inspect = nil // reset inspect; will re-fetch
	return m
}

func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.tab = DetailTab((int(m.tab) + 1) % len(detailTabLabels))
		case "shift+tab":
			m.tab = DetailTab((int(m.tab) - 1 + len(detailTabLabels)) % len(detailTabLabels))
		case "v":
			m.envMasked = !m.envMasked
		}
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	case ServiceUpdateMsg:
		if msg.Err == nil {
			for _, s := range msg.Services {
				if s.Name == m.service.Name {
					m.service = s
					break
				}
			}
			m.services = msg.Services
		}
	case StatsUpdateMsg:
		// statsHistory is a pointer to the shared map in ServicesModel; updates are automatic
	case InspectResultMsg:
		if msg.Service == m.service.Name {
			m.inspect = &msg.Data
		}
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height
	}
	return m, nil
}

func (m DetailModel) View(width, height int) string {
	if !m.ready {
		return styleDim.Render("  Select a service")
	}
	switch m.tab {
	case DetailTabOverview:
		return m.viewOverview(width, height)
	case DetailTabLogViewer:
		return m.viewLogPlaceholder(width, height)
	case DetailTabEnv:
		return m.viewEnv(width, height)
	case DetailTabPorts:
		return m.viewPorts(width, height)
	case DetailTabVolumes:
		return m.viewVolumes(width, height)
	case DetailTabConfig:
		return m.viewConfig(width, height)
	}
	return ""
}

func (m DetailModel) renderSubTabBar(width int) string {
	var parts []string
	for i, label := range detailTabLabels {
		if DetailTab(i) == m.tab {
			parts = append(parts, styleInfo.Bold(true).Render("["+label+"]"))
		} else {
			parts = append(parts, styleDim.Render(" "+label+" "))
		}
	}
	bar := strings.Join(parts, styleDim.Render("│"))
	return lipgloss.NewStyle().
		Background(lipgloss.Color("#0d1117")).
		Width(width).
		Padding(0, 1).
		Render(bar)
}

func (m DetailModel) viewOverview(width, height int) string {
	var b strings.Builder

	// Panel header
	dot, nameStyle, _ := svcStatusParts(m.service)
	healthLabel := m.service.Health
	if healthLabel == "" || healthLabel == "(none)" {
		healthLabel = m.service.State
	}
	hdr := fmt.Sprintf("%s %s — %s",
		dot,
		nameStyle.Bold(true).Render(m.service.Name),
		nameStyle.Render(healthLabel))
	if m.service.Ports != "" {
		hdr += styleDim.Render("  ·  ") + styleInfo.Render(m.service.Ports)
	}
	b.WriteString(lipgloss.NewStyle().
		Background(lipgloss.Color("#0f1117")).
		Width(width).
		Padding(0, 1).
		Render(hdr) + "\n")
	b.WriteString(m.renderSubTabBar(width) + "\n")

	// Resource rows
	h := m.statsHistory
	cpuSpark, memSpark := "—", "—"
	cpuVal, memVal := "—", "—"
	if h != nil {
		if hist, ok := h[m.service.Name]; ok && len(hist.cpu) > 0 {
			cpuSpark = styleInfo.Render(renderSparkline(hist.cpu, 100))
			memSpark = styleInfo.Render(renderSparkline(hist.mem, 100))
			cpuVal = styleHealthy.Render(fmt.Sprintf("%.1f%%", m.service.CPU))
			memVal = styleInfo.Render(fmt.Sprintf("%.1f%%", m.service.Memory))
		}
	}

	row := func(key, val string) string {
		return fmt.Sprintf("  %-10s %s\n", styleDim.Render(key), val)
	}

	b.WriteString(row("cpu", cpuSpark+" "+cpuVal))
	b.WriteString(row("memory", memSpark+" "+memVal))
	b.WriteString(row("uptime", styleBold.Render(formatUptime(m.service.Uptime))))

	// Health check from config
	cfg, _ := config.LoadOrEmpty(constants.DefaultConfigFile)
	if cfg != nil {
		if svcCfg, ok := cfg.Services[m.service.Name]; ok && svcCfg.Health != nil {
			hc := svcCfg.Health
			hcDesc := hc.Type
			if hc.URL != "" {
				hcDesc += " " + hc.URL
			} else if hc.Host != "" {
				hcDesc += fmt.Sprintf(" %s:%d", hc.Host, hc.Port)
			} else if hc.Pattern != "" {
				hcDesc += " \"" + hc.Pattern + "\""
			}
			b.WriteString(row("health", styleInfo.Render(hcDesc)))
		}
	}

	b.WriteString("\n")
	b.WriteString(m.renderMiniTable(width) + "\n")
	return b.String()
}

func (m DetailModel) renderMiniTable(width int) string {
	var b strings.Builder
	hdr := fmt.Sprintf("  %-14s %-10s %-12s %-7s %-7s %s",
		"NAME", "STATE", "HEALTH", "CPU", "MEM", "UPTIME")
	b.WriteString(styleInfo.Bold(true).Render(hdr) + "\n")
	b.WriteString(styleDim.Render("  "+strings.Repeat("─", width-4)) + "\n")

	for _, svc := range m.services {
		cpuStr := fmt.Sprintf("%.1f%%", svc.CPU)
		memStr := fmt.Sprintf("%.1f%%", svc.Memory)
		row := fmt.Sprintf("  %-14s %-10s %-12s %-7s %-7s %s",
			truncate(svc.Name, 14), svc.State, svc.Health,
			cpuStr, memStr, formatUptime(svc.Uptime))
		dot, rowStyle, _ := svcStatusParts(svc)
		prefix := dot + " "
		_ = prefix
		if svc.Name == m.service.Name {
			b.WriteString(styleSelected.Width(width).Render(row) + "\n")
		} else {
			b.WriteString(rowStyle.Render(row) + "\n")
		}
	}
	return b.String()
}

func (m DetailModel) viewLogPlaceholder(width, height int) string {
	var b strings.Builder
	b.WriteString(m.renderSubTabBar(width) + "\n")
	b.WriteString(styleDim.Render("  Switch to Logs tab (2) for full-screen log view.\n"))
	b.WriteString(styleDim.Render("  The right panel shows live log tail for this service.\n"))
	return b.String()
}

func (m DetailModel) viewEnv(width, height int) string {
	var b strings.Builder
	b.WriteString(m.renderSubTabBar(width) + "\n")
	if m.inspect == nil {
		b.WriteString(styleDim.Render("  Loading… (fetching container inspect)\n"))
		return b.String()
	}
	hint := styleDim.Render("  v") + styleDim.Render(" to reveal/mask all values") + "\n"
	b.WriteString(hint)
	b.WriteString(styleDim.Render("  "+strings.Repeat("─", width-4)) + "\n")
	for _, env := range m.inspect.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := styleInfo.Render(parts[0])
		val := parts[1]
		if m.envMasked {
			val = styleDim.Render("****")
		} else {
			val = styleBold.Render(val)
		}
		b.WriteString(fmt.Sprintf("  %-30s %s\n", key, val))
	}
	return b.String()
}

func (m DetailModel) viewPorts(width, height int) string {
	var b strings.Builder
	b.WriteString(m.renderSubTabBar(width) + "\n")
	if m.inspect == nil {
		b.WriteString(styleDim.Render("  Loading…\n"))
		return b.String()
	}
	if len(m.inspect.Ports) == 0 {
		b.WriteString(styleDim.Render("  No port bindings.\n"))
		return b.String()
	}
	b.WriteString(styleDim.Render("  "+strings.Repeat("─", width-4)) + "\n")
	for _, p := range m.inspect.Ports {
		binding := fmt.Sprintf("  0.0.0.0:%-6s → %s/%s", p.HostPort, p.ContainerPort, p.Proto)
		if p.Conflict {
			b.WriteString(styleWarning.Render(binding) + "  " + styleFailed.Render("⚠ conflict") + "\n")
		} else {
			b.WriteString(styleInfo.Render(binding) + "\n")
		}
	}
	return b.String()
}

func (m DetailModel) viewVolumes(width, height int) string {	var b strings.Builder
	b.WriteString(m.renderSubTabBar(width) + "\n")
	if m.inspect == nil {
		b.WriteString(styleDim.Render("  Loading…\n"))
		return b.String()
	}
	if len(m.inspect.Binds) == 0 {
		b.WriteString(styleDim.Render("  No volume binds.\n"))
		return b.String()
	}
	b.WriteString(styleDim.Render("  "+strings.Repeat("─", width-4)) + "\n")
	for _, bind := range m.inspect.Binds {
		parts := strings.SplitN(bind, ":", 3)
		mode := "rw"
		if len(parts) == 3 {
			mode = parts[2]
		}
		src := ""
		dst := ""
		if len(parts) >= 2 {
			src = parts[0]
			dst = parts[1]
		}
		modeStyle := styleDim
		if mode == "ro" {
			modeStyle = styleWarning
		}
		b.WriteString(fmt.Sprintf("  %s → %s  %s\n",
			styleDim.Render(truncate(src, 30)),
			styleInfo.Render(truncate(dst, 30)),
			modeStyle.Render(mode)))
	}
	return b.String()
}
```

- [ ] **Step 2: Add `InspectResultMsg` to `messages.go`**

```go
// InspectResultMsg carries Docker inspect data for a service.
type InspectResultMsg struct {
	Service string
	Data    InspectData
	Err     error
}
```

Also add the `viewConfig` method to `detail.go`. This method shows compose `depends_on` info (reuses `scaffold.ParseServices` already imported in `describe.go`). Add these imports to `detail.go`: `"github.com/deveshpharswan/stackup/internal/scaffold"`, `"github.com/deveshpharswan/stackup/internal/constants"`.

```go
func (m DetailModel) viewConfig(width, height int) string {
	var b strings.Builder
	b.WriteString(m.renderSubTabBar(width) + "\n")
	composePath := constants.FindComposeFile(".")
	if composePath == "" {
		composePath = constants.DefaultComposeFile
	}
	composeSvcs, err := scaffold.ParseServices(composePath)
	if err != nil {
		b.WriteString(styleFailed.Render("  Error reading compose file: "+err.Error()) + "\n")
		return b.String()
	}
	deps := composeSvcs[m.service.Name]
	if len(deps) == 0 {
		b.WriteString(styleDim.Render("  No dependencies defined.\n"))
	} else {
		b.WriteString(styleBold.Render("  depends_on:") + "\n")
		for _, dep := range deps {
			b.WriteString(styleInfo.Render("    - "+dep) + "\n")
		}
	}
	return b.String()
}
```

- [ ] **Step 3: Build**

```bash
go build ./internal/tui/...
```

Expected: no errors (InspectResultMsg and InspectData are defined).

- [ ] **Step 4: Commit**

```bash
git add internal/tui/detail.go internal/tui/messages.go
git commit -m "feat(tui): detail panel with overview and sub-tab skeleton"
```

---

## Task 6: Startup Progress Panel

Right panel bottom section: per-service startup progress bars, hidden once all services healthy.

**Files:**
- Create: `internal/tui/progress.go`

- [ ] **Step 1: Create `internal/tui/progress.go`**

```go
package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ProgressModel renders startup progress in the right panel.
// It disappears once all services are healthy.
type ProgressModel struct {
	services  []ServiceInfo
	firstSeen map[string]time.Time
}

func NewProgressModel() ProgressModel {
	return ProgressModel{firstSeen: make(map[string]time.Time)}
}

// AllDone returns true when every service is healthy or exited (no pending startup).
func (m ProgressModel) AllDone() bool {
	for _, s := range m.services {
		if s.State == "running" && s.Health != "healthy" {
			return false
		}
	}
	return len(m.services) > 0
}

func (m ProgressModel) Update(services []ServiceInfo) ProgressModel {
	m.services = services
	now := time.Now()
	for _, s := range services {
		if _, seen := m.firstSeen[s.Name]; !seen {
			m.firstSeen[s.Name] = now
		}
	}
	return m
}

func (m ProgressModel) View(width int) string {
	if m.AllDone() {
		return ""
	}
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorDim).
		Width(width).
		Padding(0, 1).
		Render("Startup") + "\n")

	barWidth := width - 12
	if barWidth < 4 {
		barWidth = 4
	}

	for _, s := range m.services {
		var fillStyle lipgloss.Style
		var filled int
		var statusChar string

		switch {
		case s.State == "running" && s.Health == "healthy":
			filled = barWidth
			fillStyle = lipgloss.NewStyle().Foreground(colorGreen)
			statusChar = styleHealthy.Render("✓")
		case s.State == "exited":
			filled = barWidth
			fillStyle = lipgloss.NewStyle().Foreground(colorRed)
			statusChar = styleFailed.Render("✗")
		default:
			// Estimate progress from elapsed time (max out at 90%)
			elapsed := time.Since(m.firstSeen[s.Name])
			pct := int(elapsed.Seconds() / 30 * float64(barWidth))
			if pct > barWidth*9/10 {
				pct = barWidth * 9 / 10
			}
			filled = pct
			fillStyle = lipgloss.NewStyle().Foreground(colorYellow)
			statusChar = styleWarning.Render("…")
		}

		bar := fillStyle.Render(strings.Repeat("█", filled)) +
			styleDim.Render(strings.Repeat("░", barWidth-filled))

		name := truncate(s.Name, 7)
		b.WriteString(fmt.Sprintf("  %-7s %s %s\n", name, bar, statusChar))
	}
	return b.String()
}
```

- [ ] **Step 2: Build**

```bash
go build ./internal/tui/...
```

- [ ] **Step 3: Commit**

```bash
git add internal/tui/progress.go
git commit -m "feat(tui): startup progress panel"
```

---

## Task 7: Wire 3-Panel Layout into tui.go

Replace the old `viewStack`/`ViewType` model with the new tab + 3-panel layout. Update tests.

**Files:**
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/tui_test.go`

- [ ] **Step 1: Replace `Model` struct in `tui.go`**

Replace the entire `Model` struct and `NewModel` function:

```go
type Model struct {
	width  int
	height int

	activeTab TabType

	services ServicesModel // data source: polling + filter state
	sidebar  SidebarModel
	detail   DetailModel
	logTail  LogsModel // right panel live log tail
	logs     LogsModel // tab 2 full-screen logs
	doctor   DoctorViewModel
	graph    GraphModel
	progress ProgressModel

	header  HeaderModel
	command CommandModel
	toast   ToastModel
	help    HelpModel
	confirm ConfirmModel

	showHelp    bool
	showConfirm bool
	quitting    bool
}

func NewModel(dc *dockerclient.Client) Model {
	return Model{
		activeTab: TabServices,
		services:  NewServicesModel(dc),
		sidebar:   NewSidebarModel(),
		detail:    NewDetailModel(),
		doctor:    NewDoctorViewModel(),
		graph:     NewGraphModel(),
		progress:  NewProgressModel(),
		header:    NewHeaderModel(),
		command:   NewCommandModel(),
		toast:     NewToastModel(),
		help:      NewHelpModel(),
		confirm:   NewConfirmModel(),
	}
}
```

- [ ] **Step 2: Replace `Update()` key handling in `tui.go`**

Replace the `tea.KeyMsg` switch inside `Update()`:

```go
case tea.KeyMsg:
	if m.showConfirm {
		newConfirm, cmd := m.confirm.Update(msg)
		m.confirm = newConfirm
		if !m.confirm.active {
			m.showConfirm = false
		}
		return m, cmd
	}
	if m.showHelp {
		if msg.String() == "esc" || msg.String() == "?" {
			m.showHelp = false
			return m, nil
		}
		return m, nil
	}
	if m.command.Active() {
		newCmd, cmd := m.command.Update(msg)
		m.command = newCmd
		m.services = m.services.SetFilter(m.command.Filter())
		m.sidebar = m.sidebar.SetServices(m.services.filtered)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)
	}
	switch msg.String() {
	case "?":
		m.showHelp = true
		return m, nil
	case "/":
		if m.activeTab == TabLogs {
			newLogs, cmd := m.logs.ActivateFilter()
			m.logs = newLogs
			return m, cmd
		}
		m.command.Activate(ModeFilter)
		return m, nil
	case ":":
		m.command.Activate(ModeCommand)
		return m, nil
	case "q":
		if m.activeTab == TabServices {
			m.quitting = true
			return m, tea.Quit
		}
	case "esc":
		if m.activeTab == TabLogs {
			m.logs.Stop()
		}
		m.activeTab = TabServices
		return m, nil
	case "1":
		m.activeTab = TabServices
		return m, nil
	case "2":
		m.activeTab = TabLogs
		if svc := m.sidebar.Selected(); svc != "" {
			m.logs.Stop()
			newLogs, cmd := m.logs.Start(svc, m.width, m.height-headerLines-footerLines)
			m.logs = newLogs
			return m, cmd
		}
		return m, nil
	case "3":
		m.activeTab = TabStats
		return m, nil
	case "4":
		m.activeTab = TabDoctor
		return m, m.doctor.Init()
	case "5":
		m.activeTab = TabGraph
		return m, m.graph.Init()
	case "d":
		m.activeTab = TabDoctor
		return m, m.doctor.Init()
	case "g":
		m.activeTab = TabGraph
		return m, m.graph.Init()
	case "D":
		m.confirm = m.confirm.Request(ConfirmStackDown, "")
		m.showConfirm = true
		return m, nil
	}
```

- [ ] **Step 3: Replace `View()` in `tui.go`**

```go
func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.width < minWidth || m.height < minHeight {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			styleBold.Render("Terminal too small")+"\n"+
				styleDim.Render(fmt.Sprintf("Need %dx%d, got %dx%d", minWidth, minHeight, m.width, m.height)))
	}

	layout := ComputeLayout(m.width, m.height)

	var view string
	hdr := m.header.View(m.width, m.activeTab)
	tabBar := renderTabBar(m.width, m.activeTab)
	footer := m.renderFooter()

	switch m.activeTab {
	case TabServices:
		view = m.renderServicesTab(layout)
	case TabLogs:
		view = m.logs.View(m.width, layout.ContentHeight)
	case TabStats:
		view = styleDim.Render("  Stats view — coming soon")
	case TabDoctor:
		view = m.doctor.View(m.width, layout.ContentHeight)
	case TabGraph:
		view = m.graph.View(m.width, layout.ContentHeight)
	}

	full := lipgloss.JoinVertical(lipgloss.Left, hdr, tabBar, view, footer)

	if m.showHelp {
		full = m.help.View(m.width, m.height, ViewServices) // ViewServices kept for help compat
	}
	if m.showConfirm {
		full = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			m.confirm.View())
	}
	return full
}

func (m Model) renderServicesTab(layout PanelLayout) string {
	contentH := layout.ContentHeight

	// Left sidebar
	var leftPanel string
	if layout.HasSidebar {
		leftPanel = lipgloss.NewStyle().
			Width(layout.SidebarWidth).
			Height(contentH).
			BorderRight(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorBorder).
			Render(m.sidebar.View(layout.SidebarWidth-1, contentH))
	}

	// Right panel (log tail + progress)
	var rightPanel string
	if layout.HasRight {
		logHeight := contentH
		progressStr := m.progress.View(layout.RightWidth)
		if progressStr != "" {
			progressLines := strings.Count(progressStr, "\n") + 1
			logHeight = contentH - progressLines
			if logHeight < 2 {
				logHeight = 2
			}
		}
		logContent := m.logTail.View(layout.RightWidth, logHeight)
		logHdr := lipgloss.NewStyle().
			Background(lipgloss.Color("#0f1117")).
			Foreground(colorBlue).
			Bold(true).
			Width(layout.RightWidth).
			Padding(0, 1).
			Render("Live Logs  " + styleDim.Render(m.sidebar.Selected()))
		logSection := logHdr + "\n" + logContent

		rightPanel = lipgloss.NewStyle().
			Width(layout.RightWidth).
			Height(contentH).
			BorderLeft(true).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(colorBorder).
			Render(lipgloss.JoinVertical(lipgloss.Left, logSection, progressStr))
	}

	// Center detail panel
	centerContent := m.detail.View(layout.CenterWidth, contentH)
	centerPanel := lipgloss.NewStyle().
		Width(layout.CenterWidth).
		Height(contentH).
		Render(centerContent)

	if layout.HasSidebar && layout.HasRight {
		return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, centerPanel, rightPanel)
	}
	if layout.HasSidebar {
		return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, centerPanel)
	}
	return centerPanel
}

func (m Model) renderFooter() string {
	if m.command.Active() {
		return m.command.View(m.width)
	}
	if msg := m.toast.Message(); msg != "" {
		return styleStatusBar.Width(m.width).Render("  " + msg)
	}

	var hints []string
	switch m.activeTab {
	case TabServices:
		hints = []string{"↑↓:nav", "enter:focus", "1–5:tabs", "r:restart", "s:stop", "u:start", "x:shell", "D:down", "/:filter", "?:help", "q:quit"}
	case TabLogs:
		hints = []string{"↑↓:scroll", "g/G:top/bot", "/:filter", "t:timestamps", "1–5:tabs", "esc:back"}
	default:
		hints = []string{"1–5:tabs", "esc:back", "?:help", "q:quit"}
	}

	var parts []string
	for _, h := range hints {
		idx := strings.Index(h, ":")
		if idx >= 0 {
			parts = append(parts, styleInfo.Render(h[:idx+1])+styleDim.Render(h[idx+1:]))
		} else {
			parts = append(parts, styleDim.Render(h))
		}
	}
	return styleStatusBar.Width(m.width).Render("  " + strings.Join(parts, styleDim.Render("  ")))
}
```

- [ ] **Step 4: Update the `ServiceUpdateMsg` and `StatsUpdateMsg` handlers in `Update()`**

Add sidebar + detail + progress updates:

```go
case ServiceUpdateMsg:
	if msg.Err == nil {
		names := make([]string, len(msg.Services))
		for i, s := range msg.Services {
			names[i] = s.Name
		}
		m.command.SetServiceNames(names)
		newSidebar, cmd := m.sidebar.Update(msg)
		m.sidebar = newSidebar
		cmds = append(cmds, cmd)
		// Update detail with new service data
		if svc := m.sidebar.SelectedInfo(); svc != nil {
			m.detail = m.detail.SetService(*svc, msg.Services, m.services_statsHistory())
		}
		m.progress = m.progress.Update(msg.Services)
	}
```

Note: `m.services_statsHistory()` is a helper — add it:

```go
// services_statsHistory returns the stats history from the services polling goroutine.
// We keep a reference copy on Model so detail and progress can use it.
func (m *Model) services_statsHistory() map[string]*StatsHistory {
	return m.servicesStatsHistory
}
```

Actually, simpler: add `servicesStatsHistory map[string]*StatsHistory` to `Model` and update it whenever `StatsUpdateMsg` arrives. Or pass it through a message. The cleanest approach for the plan: add a field to Model:

```go
type Model struct {
	// ... existing fields ...
	statsHistory map[string]*StatsHistory // shared, updated from StatsUpdateMsg
}
```

Initialize it in `NewModel`:
```go
statsHistory: make(map[string]*StatsHistory),
```

In `Update()` for `StatsUpdateMsg`:
```go
case StatsUpdateMsg:
	if msg.Stats != nil {
		for name, s := range msg.Stats {
			h, ok := m.statsHistory[name]
			if !ok {
				h = &StatsHistory{}
				m.statsHistory[name] = h
			}
			h.Push(s.CPU, s.Memory)
		}
		// Update detail panel sparklines
		if svc := m.sidebar.SelectedInfo(); svc != nil {
			for _, ss := range m.sidebar.services {
				if ss.Name == svc.Name {
					svc2 := ss
					m.detail = m.detail.SetService(svc2, m.sidebar.services, m.statsHistory)
					break
				}
			}
		}
	}
```

- [ ] **Step 5: Start log tail when sidebar selection changes**

In the sidebar `Update`, add a new message type for selection change. Add to `messages.go`:

```go
type SidebarSelectionMsg struct {
	Service string
}
```

In `sidebar.go`, emit `SidebarSelectionMsg` when cursor changes:

```go
case "j", "down":
	if m.cursor < len(m.services)-1 {
		m.cursor++
		return m, func() tea.Msg { return SidebarSelectionMsg{Service: m.Selected()} }
	}
case "k", "up":
	if m.cursor > 0 {
		m.cursor--
		return m, func() tea.Msg { return SidebarSelectionMsg{Service: m.Selected()} }
	}
```

In `tui.go` `Update()`, handle `SidebarSelectionMsg`:

```go
case SidebarSelectionMsg:
	// Update log tail for new selection
	m.logTail.Stop()
	newLogTail, cmd := m.logTail.Start(msg.Service, rightWidth, 20)
	m.logTail = newLogTail
	cmds = append(cmds, cmd)
	// Trigger inspect fetch
	if m.sidebar.SelectedInfo() != nil {
		cmds = append(cmds, fetchInspect(msg.Service))
	}
```

Add `fetchInspect` to `tui.go`:

```go
func fetchInspect(service string) tea.Cmd {
	return func() tea.Msg {
		// Get container ID
		out, err := exec.Command("docker", "compose", "ps", "-q", service).Output()
		if err != nil || len(strings.TrimSpace(string(out))) == 0 {
			return InspectResultMsg{Service: service, Err: err}
		}
		containerID := strings.TrimSpace(string(out))

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		dc, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
		if err != nil {
			return InspectResultMsg{Service: service, Err: err}
		}
		defer dc.Close()

		info, err := dc.ContainerInspect(ctx, containerID)
		if err != nil {
			return InspectResultMsg{Service: service, Err: err}
		}

		data := InspectData{
			Env:   info.Config.Env,
			Binds: info.HostConfig.Binds,
		}

		// Parse port mappings
		for cPort, bindings := range info.NetworkSettings.Ports {
			for _, b := range bindings {
				pm := PortMapping{
					HostPort:      b.HostPort,
					ContainerPort: cPort.Port(),
					Proto:         cPort.Proto(),
				}
				// Check for port conflict (another process using the host port)
				pm.Conflict = isPortInUse(b.HostPort)
				data.Ports = append(data.Ports, pm)
			}
		}

		return InspectResultMsg{Service: service, Data: data}
	}
}

func isPortInUse(port string) bool {
	if port == "" {
		return false
	}
	conn, err := net.DialTimeout("tcp", "localhost:"+port, 200*time.Millisecond)
	if err == nil {
		conn.Close()
		return true
	}
	return false
}
```

Add `"context"`, `"net"`, `"time"` to imports in tui.go.

- [ ] **Step 6: Remove `titleBar()` method and old ViewType-based code from tui.go**

Delete:
- `func (m Model) titleBar() string`
- `func (m Model) statusBar() string` (replaced by `renderFooter`)
- `func (m Model) pushView(v ViewType) Model`
- `func (m Model) popView() Model`

Keep `ViewServices` etc. for backwards compat with `help.go` (help.go still uses `ViewType`). Will refactor help in a later task if needed — for now, pass `ViewServices` as dummy.

- [ ] **Step 7: Update `tui_test.go` for new model**

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestModel_InitialTab(t *testing.T) {
	m := NewModel(nil)
	assert.Equal(t, TabServices, m.activeTab)
}

func TestModel_TabSwitch(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 30
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("4")})
	model := newModel.(Model)
	assert.Equal(t, TabDoctor, model.activeTab)
}

func TestModel_EscReturnsToServices(t *testing.T) {
	m := NewModel(nil)
	m.width = 120
	m.height = 30
	m.activeTab = TabLogs
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model := newModel.(Model)
	assert.Equal(t, TabServices, model.activeTab)
}

func TestModel_TerminalTooSmall(t *testing.T) {
	m := NewModel(nil)
	m.width = 40
	m.height = 10
	view := m.View()
	assert.Contains(t, view, "Terminal too small")
}

func TestModel_QuitOnQ(t *testing.T) {
	m := NewModel(nil)
	m.width = 100
	m.height = 30
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	model := newModel.(Model)
	assert.True(t, model.quitting)
	assert.NotNil(t, cmd)
}

func TestModel_HelpToggle(t *testing.T) {
	m := NewModel(nil)
	m.width = 100
	m.height = 30
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	model := newModel.(Model)
	assert.True(t, model.showHelp)

	newModel, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	model = newModel.(Model)
	assert.False(t, model.showHelp)
}
```

- [ ] **Step 8: Run tests**

```bash
go test ./internal/tui/... -v -run TestModel
```

Expected: all TestModel_* tests pass.

- [ ] **Step 9: Build the full binary**

```bash
go build ./...
```

Expected: binary compiles. Run `./stackup` (or `stackup.exe`) and verify 3-panel layout appears.

- [ ] **Step 10: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_test.go internal/tui/messages.go internal/tui/sidebar.go
git commit -m "feat(tui): wire 3-panel layout into main model"
```

---

## Task 8: Tier Data in Sidebar

Populate `ServiceInfo.Tier` so sidebar shows `tier 1`, `tier 2`, etc. dividers.

**Files:**
- Modify: `internal/tui/tui.go` (add tier computation on startup)

- [ ] **Step 1: Compute tiers when graph data arrives and stamp `ServiceInfo.Tier`**

In `tui.go` `Update()`, handle `graphDataMsg` (already handled by graph model):

```go
case graphDataMsg:
	if msg.err == nil {
		// Build name→tier map
		tierMap := make(map[string]int)
		for i, tier := range msg.tiers {
			for _, name := range tier {
				tierMap[name] = i + 1
			}
		}
		// Stamp tiers onto sidebar services
		services := make([]ServiceInfo, len(m.sidebar.services))
		copy(services, m.sidebar.services)
		for i, s := range services {
			services[i].Tier = tierMap[s.Name]
		}
		m.sidebar = m.sidebar.SetServices(services)
	}
	// Forward to graph model
	newGraph, cmd := m.graph.Update(msg)
	m.graph = newGraph
	cmds = append(cmds, cmd)
```

Also trigger tier loading on startup. In `Model.Init()`:

```go
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		pollServices(),
		tickEvery(2*time.Second),
		m.graph.Init(), // loads tier data via graphDataMsg
	)
}
```

- [ ] **Step 2: Build and verify**

```bash
go build ./...
```

Run `stackup` and verify tier dividers appear in the sidebar for multi-tier stacks.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/tui.go
git commit -m "feat(tui): populate tier data for sidebar grouping"
```

---

## Task 9: Action Key Remapping + `u` Start + `s` Stop

Current: `s`=shell, `x`=stop. New: `s`=stop, `x`=shell (already in tui.go via `shellIntoService`), `u`=start.

**Files:**
- Modify: `internal/tui/services.go`
- Modify: `internal/tui/tui.go`

- [ ] **Step 1: Remap keys in `services.go`**

Replace the key cases in `ServicesModel.Update()`:

```go
case "s":
	if svc := m.Selected(); svc != "" {
		return m, func() tea.Msg {
			return ConfirmRequestMsg{Action: ConfirmDelete, Service: svc}
		}
	}
case "x":
	if svc := m.Selected(); svc != "" {
		return m, func() tea.Msg {
			return shellRequestMsg{Service: svc}
		}
	}
case "u":
	if svc := m.Selected(); svc != "" {
		return m, func() tea.Msg {
			return startServiceMsg{Service: svc}
		}
	}
```

- [ ] **Step 2: Add `startServiceMsg` to `messages.go`**

```go
type startServiceMsg struct {
	Service string
}
```

- [ ] **Step 3: Handle `startServiceMsg` in `tui.go` Update()**

```go
case startServiceMsg:
	return m, startService(msg.Service)
```

Add `startService` function near `stopService` in `tui.go`:

```go
func startService(name string) tea.Cmd {
	return func() tea.Msg {
		c := exec.Command("docker", "compose", "up", "-d", name)
		err := c.Run()
		if err != nil {
			return ActionResultMsg{Err: fmt.Errorf("start %s: %w", name, err)}
		}
		return ActionResultMsg{Text: fmt.Sprintf("Service %q started", name)}
	}
}
```

- [ ] **Step 4: Add bash fallback to `shellIntoService` in `tui.go`**

Replace the existing `shellIntoService` function:

```go
func shellIntoService(name string) tea.Cmd {
	// Try sh first; if the container has no sh, fall back to bash.
	c := exec.Command("docker", "compose", "exec", name, "sh", "-c", "sh || bash")
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return tea.ExecProcess(c, func(err error) tea.Msg {
		if err != nil {
			return ActionResultMsg{Err: fmt.Errorf("shell %s: %w", name, err)}
		}
		return ActionResultMsg{Text: fmt.Sprintf("Exited shell for %q", name)}
	})
}
```

- [ ] **Step 5: Update ConfirmModel for "stop" label (cosmetic)**

In `confirm.go`, the `ConfirmDelete` case label already says "Stop service". No change needed.

- [ ] **Step 6: Update `services_test.go`** — verify key remapping

```go
func TestServicesModel_KeyX_EmitsShellRequest(t *testing.T) {
	m := NewServicesModel(nil)
	m.services = []ServiceInfo{{Name: "api", State: "running"}}
	m.filtered = m.services

	newM, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	_ = newM
	require.NotNil(t, cmd)
	msg := cmd()
	assert.IsType(t, shellRequestMsg{}, msg)
}

func TestServicesModel_KeyU_EmitsStartMsg(t *testing.T) {
	m := NewServicesModel(nil)
	m.services = []ServiceInfo{{Name: "api", State: "exited"}}
	m.filtered = m.services

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	require.NotNil(t, cmd)
	msg := cmd()
	assert.IsType(t, startServiceMsg{}, msg)
}
```

- [ ] **Step 7: Run tests**

```bash
go test ./internal/tui/... -v -run TestServicesModel
```

Expected: all pass.

- [ ] **Step 8: Commit**

```bash
git add internal/tui/services.go internal/tui/tui.go internal/tui/messages.go
git commit -m "feat(tui): remap s→stop x→shell, add u→start, sh→bash fallback"
```

---

## Task 10: Bulk Stack Down — `D`

Capital `D` brings down the full stack with a confirmation modal.

**Files:**
- Modify: `internal/tui/messages.go`
- Modify: `internal/tui/confirm.go`
- Modify: `internal/tui/tui.go`

- [ ] **Step 1: Add `ConfirmStackDown` to `ConfirmAction` enum in `messages.go`**

```go
const (
	ConfirmRestart   ConfirmAction = iota
	ConfirmDelete
	ConfirmStackDown // NEW
)
```

- [ ] **Step 2: Update `confirm.go` View() for stack down case**

```go
case ConfirmStackDown:
	title = "Bring down the full stack?"
	desc = "This will stop and remove all containers."
```

- [ ] **Step 3: Handle `ConfirmYesMsg` for `ConfirmStackDown` in `tui.go`**

```go
case ConfirmYesMsg:
	m.showConfirm = false
	switch msg.Action {
	case ConfirmRestart:
		return m, restartService(msg.Service)
	case ConfirmDelete:
		return m, stopService(msg.Service)
	case ConfirmStackDown:
		return m, downStack()
	}
```

Add `downStack` function in `tui.go`:

```go
func downStack() tea.Cmd {
	return func() tea.Msg {
		c := exec.Command("docker", "compose", "down")
		err := c.Run()
		if err != nil {
			return ActionResultMsg{Err: fmt.Errorf("stack down: %w", err)}
		}
		return ActionResultMsg{Text: "Stack brought down"}
	}
}
```

- [ ] **Step 4: Build and verify**

```bash
go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/tui/messages.go internal/tui/confirm.go internal/tui/tui.go
git commit -m "feat(tui): bulk stack down with D key + confirmation"
```

---

## Task 11: Live Log Filter

Press `/` in the Logs tab to filter log lines; `esc` clears.

**Files:**
- Modify: `internal/tui/logs.go`

- [ ] **Step 1: Add filter state to `LogsModel`**

```go
type LogsModel struct {
	service    string
	viewport   viewport.Model
	lines      []string
	cancel     context.CancelFunc
	logCh      <-chan string
	timestamps bool
	wrap       bool
	ready      bool
	filter     string      // NEW
	filtering  bool        // NEW — true when filter input active
}
```

- [ ] **Step 2: Add `ActivateFilter()` method**

```go
func (m LogsModel) ActivateFilter() (LogsModel, tea.Cmd) {
	m.filtering = true
	return m, nil
}
```

- [ ] **Step 3: Handle filter input in `LogsModel.Update()`**

Add to the `tea.KeyMsg` section of `Update()`:

```go
case tea.KeyMsg:
	if m.filtering {
		switch msg.String() {
		case "esc", "enter":
			m.filtering = false
		case "backspace":
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.viewport.SetContent(m.renderLines())
			}
		default:
			if len(msg.Runes) > 0 {
				m.filter += string(msg.Runes)
				m.viewport.SetContent(m.renderLines())
			}
		}
		return m, nil
	}
	// existing key handling continues below...
```

- [ ] **Step 4: Update `renderLines()` to apply filter**

```go
func (m LogsModel) renderLines() string {
	var b strings.Builder
	for _, line := range m.lines {
		// Filter: skip non-matching lines
		if m.filter != "" && !strings.Contains(strings.ToLower(line), strings.ToLower(m.filter)) {
			continue
		}
		display := line
		if !m.timestamps {
			display = stripTimestamp(display)
		}
		rendered := m.colorLogLine(display)
		// Highlight filter match
		if m.filter != "" {
			idx := strings.Index(strings.ToLower(rendered), strings.ToLower(m.filter))
			if idx >= 0 {
				rendered = rendered[:idx] +
					lipgloss.NewStyle().Background(colorYellow).Foreground(lipgloss.Color("#000000")).Render(rendered[idx:idx+len(m.filter)]) +
					rendered[idx+len(m.filter):]
			}
		}
		if m.wrap && m.viewport.Width > 0 {
			rendered = lipgloss.NewStyle().Width(m.viewport.Width).Render(rendered)
		}
		b.WriteString(rendered + "\n")
	}
	return b.String()
}
```

- [ ] **Step 5: Show filter input in `View()`**

Add a filter indicator line when filtering:

```go
func (m LogsModel) View(width, height int) string {
	if !m.ready {
		return styleDim.Render("  No service selected")
	}
	m.viewport.Width = width
	vpHeight := height
	if m.filtering || m.filter != "" {
		vpHeight = height - 1
	}
	m.viewport.Height = vpHeight

	view := m.viewport.View()

	if m.filtering {
		filterBar := styleStatusBar.Width(width).Render("  Filter: " + m.filter + "█")
		return lipgloss.JoinVertical(lipgloss.Left, view, filterBar)
	}
	if m.filter != "" {
		filterBar := styleDim.Width(width).Render(fmt.Sprintf("  Filter: %s  (esc to clear)", m.filter))
		return lipgloss.JoinVertical(lipgloss.Left, view, filterBar)
	}
	return view
}
```

- [ ] **Step 6: Build**

```bash
go build ./...
```

- [ ] **Step 7: Commit**

```bash
git add internal/tui/logs.go
git commit -m "feat(tui): live log filter with / key"
```

---

## Task 12: Docker Inspect Fetcher

Fetch container env, ports, and volumes for the detail panel.

This task validates the `fetchInspect` function added in Task 7 actually works and the detail panel renders the data.

**Files:**
- No new files — `fetchInspect` is already in `tui.go` from Task 7.

- [ ] **Step 1: Verify `InspectResultMsg` is handled in `tui.go` Update()**

Add the handler (if not already present from Task 7):

```go
case InspectResultMsg:
	if msg.Err == nil {
		newDetail, cmd := m.detail.Update(msg)
		m.detail = newDetail
		cmds = append(cmds, cmd)
	}
```

- [ ] **Step 2: Ensure `fetchInspect` is called when sidebar selection changes**

In Task 7 Step 5, we added:
```go
cmds = append(cmds, fetchInspect(msg.Service))
```

Verify this is present in the `SidebarSelectionMsg` handler.

- [ ] **Step 3: Build and manual test**

```bash
go build ./...
```

Run `stackup` on the `tests/e2e/testdata/simple-stack` fixture:
```bash
cd tests/e2e/testdata/simple-stack && docker compose up -d && stackup
```

Select a service, press `tab` until Ports or Env sub-tab is visible. Verify data appears.

- [ ] **Step 4: Commit (if any code changes were needed)**

```bash
git add internal/tui/tui.go
git commit -m "feat(tui): wire inspect fetcher to detail sub-tabs"
```

---

## Task 13: Stacking Toasts

Replace single-message toast with a stack of up to 3 messages that auto-dismiss independently.

**Files:**
- Modify: `internal/tui/toast.go`
- Modify: `internal/tui/messages.go`

- [ ] **Step 1: Rewrite `ToastModel` in `toast.go`**

```go
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const maxToasts = 3

type toastEntry struct {
	text  string
	level toastLevel
}

type toastLevel int

const (
	toastInfo toastLevel = iota
	toastSuccess
	toastWarning
	toastError
)

type ToastModel struct {
	entries []toastEntry
}

func NewToastModel() ToastModel { return ToastModel{} }

func (m ToastModel) Show(text string) ToastModel {
	return m.ShowLevel(text, toastInfo)
}

func (m ToastModel) ShowLevel(text string, level toastLevel) ToastModel {
	m.entries = append(m.entries, toastEntry{text: text, level: level})
	if len(m.entries) > maxToasts {
		m.entries = m.entries[len(m.entries)-maxToasts:]
	}
	return m
}

func (m ToastModel) HideOldest() ToastModel {
	if len(m.entries) > 0 {
		m.entries = m.entries[1:]
	}
	return m
}

// Message returns the most recent toast text (used for status bar compat).
func (m ToastModel) Message() string {
	if len(m.entries) == 0 {
		return ""
	}
	return m.entries[len(m.entries)-1].text
}

func (m ToastModel) Tick() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return ToastExpiredMsg{}
	})
}

// RenderOverlay renders stacked toasts in the bottom-right corner.
// Returns empty string if no toasts.
func (m ToastModel) RenderOverlay(screenWidth, screenHeight int) string {
	if len(m.entries) == 0 {
		return ""
	}
	var lines []string
	for _, e := range m.entries {
		var style lipgloss.Style
		switch e.level {
		case toastSuccess:
			style = styleHealthy.Copy().Background(lipgloss.Color("#0d2818")).Padding(0, 1)
		case toastWarning:
			style = styleWarning.Copy().Background(lipgloss.Color("#2d2005")).Padding(0, 1)
		case toastError:
			style = styleFailed.Copy().Background(lipgloss.Color("#2d0f0f")).Padding(0, 1)
		default:
			style = styleInfo.Copy().Background(lipgloss.Color("#0d1f38")).Padding(0, 1)
		}
		lines = append(lines, style.Render(e.text))
	}
	return lipgloss.JoinVertical(lipgloss.Right, lines...)
}
```

- [ ] **Step 2: Update `ToastExpiredMsg` handler in `tui.go`**

```go
case ToastExpiredMsg:
	m.toast = m.toast.HideOldest()
	if len(m.toast.entries) > 0 {
		// More toasts remain; schedule next expiry
		cmds = append(cmds, m.toast.Tick())
	}
```

- [ ] **Step 3: Add `ToastLevel` variants to ToastMsg**

In `messages.go`:
```go
type ToastMsg struct {
	Text  string
	Level toastLevel
}
```

Update all `ToastMsg{Text: ...}` usages to still work (Level defaults to 0 = toastInfo).

Update `ToastMsg` handling in `tui.go`:
```go
case ToastMsg:
	m.toast = m.toast.ShowLevel(msg.Text, msg.Level)
	cmds = append(cmds, m.toast.Tick())
```

- [ ] **Step 4: Emit `toastWarning` when a service goes down**

In the `ServiceUpdateMsg` handler in `tui.go`, detect state transitions:

```go
case ServiceUpdateMsg:
	if msg.Err == nil {
		// Detect newly failed services for toast notification
		prevByName := make(map[string]ServiceInfo)
		for _, s := range m.sidebar.services {
			prevByName[s.Name] = s
		}
		for _, s := range msg.Services {
			if prev, ok := prevByName[s.Name]; ok {
				if prev.State == "running" && s.State == "exited" {
					cmds = append(cmds, func() tea.Msg {
						return ToastMsg{Text: s.Name + " stopped unexpectedly", Level: toastWarning}
					})
				}
				if prev.Health != "healthy" && s.Health == "healthy" {
					cmds = append(cmds, func() tea.Msg {
						return ToastMsg{Text: s.Name + " is healthy", Level: toastSuccess}
					})
				}
			}
		}
		// ... rest of ServiceUpdateMsg handling
```

- [ ] **Step 5: Build**

```bash
go build ./...
```

- [ ] **Step 6: Run tests**

```bash
go test ./internal/tui/... -v
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add internal/tui/toast.go internal/tui/tui.go internal/tui/messages.go
git commit -m "feat(tui): stacking toasts with service state change notifications"
```

---

## Task 14: Longer Sparklines

Increase sparkline from 8 to 16 samples, scaled to panel width.

**Files:**
- Modify: `internal/tui/stats.go`

- [ ] **Step 1: Change sparkline history length**

In `stats.go`, change `sparklineLen` constant and `StatsHistory.Push`:

```go
const sparklineLen = 16 // was 8
```

- [ ] **Step 2: Update `renderSparkline` to accept explicit length parameter**

```go
func renderSparkline(values []float64, maxVal float64) string {
	return renderSparklineN(values, maxVal, sparklineLen)
}

func renderSparklineN(values []float64, maxVal float64, n int) string {
	if n <= 0 {
		n = sparklineLen
	}
	if len(values) == 0 {
		return strings.Repeat(string(sparkChars[0]), n)
	}
	if maxVal <= 0 {
		maxVal = 100
	}
	var b strings.Builder
	pad := n - len(values)
	if pad < 0 {
		// Use last n values
		values = values[len(values)-n:]
		pad = 0
	}
	for i := 0; i < pad; i++ {
		b.WriteRune(sparkChars[0])
	}
	for _, v := range values {
		idx := int((v / maxVal) * float64(len(sparkChars)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkChars) {
			idx = len(sparkChars) - 1
		}
		b.WriteRune(sparkChars[idx])
	}
	return b.String()
}
```

- [ ] **Step 3: Build and verify**

```bash
go build ./...
go test ./internal/tui/... -v
```

Expected: all pass. Sparklines in the detail panel now show 16 samples.

- [ ] **Step 4: Commit**

```bash
git add internal/tui/stats.go
git commit -m "feat(tui): longer sparklines (8→16 samples)"
```

---

## Task 15: Final Integration Test

Manual end-to-end test against real Docker services.

**Files:** None (manual test only)

- [ ] **Step 1: Build the binary**

```bash
go build -o stackup ./main.go
```

- [ ] **Step 2: Start test services**

```bash
cd tests/e2e/testdata/multi-tier && docker compose up -d
```

- [ ] **Step 3: Launch the TUI and verify all features**

```bash
cd tests/e2e/testdata/multi-tier && ../../../../stackup
```

Verify checklist:
- [ ] 1-line slim header shows project name, healthy/starting badges, uptime
- [ ] Tab bar shows `1 Services` (active, blue) and `2 Logs  3 Stats  4 Doctor  5 Graph` (dim)
- [ ] Left sidebar shows services grouped by tier with `tier 1`, `tier 2` dividers
- [ ] Center panel shows selected service overview (CPU sparkline, memory sparkline, health check)
- [ ] Mini services table in center panel shows all services with status colors
- [ ] Right panel shows live log tail for selected service
- [ ] Right panel bottom shows startup progress bars (may show "done" for all)
- [ ] Footer bar shows key hints: `↑↓:nav enter:focus 1–5:tabs r:restart s:stop u:start x:shell D:down /:filter ?:help q:quit`
- [ ] Press `↑`/`↓` — sidebar cursor moves, log tail updates for new service
- [ ] Press `tab` — detail sub-panel cycles: overview → logs → env → ports → volumes
- [ ] Press `v` on env sub-panel — values toggle masked/revealed
- [ ] Press `2` — switches to full-screen Logs tab
- [ ] Press `/` in Logs tab — filter input activates; type a string, logs filter live
- [ ] Press `esc` — returns to Services tab
- [ ] Press `4` — Doctor tab opens
- [ ] Press `5` — Graph tab opens
- [ ] Press `D` — confirmation modal appears "Bring down the full stack?"
- [ ] Press `n` — modal dismisses
- [ ] Press `?` — help overlay appears

- [ ] **Step 4: Stop test services**

```bash
cd tests/e2e/testdata/multi-tier && docker compose down
```

- [ ] **Step 5: Run full test suite**

```bash
go test ./... -timeout 5m
```

Expected: all unit tests pass. E2E tests may be skipped on Windows (expected per TESTING.md).

- [ ] **Step 6: Final commit**

```bash
git add -A
git commit -m "feat(tui): complete TUI redesign — 3-panel layout, 9 new features"
```
