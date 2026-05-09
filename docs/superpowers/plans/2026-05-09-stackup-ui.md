# Stackup UI (TUI) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a k9s-style interactive terminal UI (`stackup ui`) that provides real-time service monitoring, log streaming, diagnostics, and dependency visualization for Docker Compose stacks.

**Architecture:** Single bubbletea Program with a root model that delegates to sub-models per view (services table, log viewport, doctor table, graph renderer, describe panel). Data flows via tea.Msg — a Docker poller emits ServiceUpdateMsg every 2s, log streamer emits LogLineMsg, doctor runner emits DoctorResultMsg. Views are swapped via command mode (`:services`, `:logs <name>`, etc.) maintaining a view history stack for `Esc`-back navigation.

**Tech Stack:** bubbletea (TUI framework), lipgloss (styling), bubbles (table/viewport/textinput components), existing internal packages (docker, config, orchestrator, doctor)

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `internal/tui/styles.go` | Shared lipgloss color palette, border styles, layout helpers |
| `internal/tui/messages.go` | All `tea.Msg` type definitions for inter-component communication |
| `internal/tui/keys.go` | `key.Binding` definitions per view, help text |
| `internal/tui/tui.go` | Root model: Init/Update/View, view switching, overlay management |
| `internal/tui/header.go` | Header block: stack metadata, keybinding hints, ASCII logo |
| `internal/tui/command.go` | Command bar: `:` command mode, `/` filter mode, tab completion |
| `internal/tui/toast.go` | Toast notification: auto-dismiss messages |
| `internal/tui/services.go` | Services table view: Docker polling, row rendering, selection |
| `internal/tui/logs.go` | Logs view: streaming viewport with search/scroll |
| `internal/tui/doctorview.go` | Doctor diagnostics view: findings table, re-scan |
| `internal/tui/graph.go` | Dependency graph: ASCII DAG renderer |
| `internal/tui/describe.go` | Service describe: detail panel from config + Docker inspect |
| `internal/tui/help.go` | Help overlay: context-sensitive keybinding reference |
| `internal/tui/confirm.go` | Confirmation modal: y/n for destructive actions |
| `cmd/ui.go` | Cobra command registration for `stackup ui` |
| `internal/tui/tui_test.go` | Unit tests for root model transitions |
| `internal/tui/services_test.go` | Unit tests for services view update logic |
| `internal/tui/command_test.go` | Unit tests for command parsing and completion |

### Modified Files

| File | Change |
|------|--------|
| `go.mod` | Add bubbletea, lipgloss, bubbles dependencies |
| `cmd/root.go` | Register `newUICmd()` in `AddCommand` |
| `CHANGELOG.md` | Add TUI entry under [Unreleased] |

---

## Task 1: Foundation — Dependencies, Styles, Messages, Root Skeleton

**Files:**
- Modify: `go.mod`
- Create: `internal/tui/styles.go`
- Create: `internal/tui/messages.go`
- Create: `internal/tui/keys.go`
- Create: `internal/tui/tui.go`
- Create: `cmd/ui.go`
- Modify: `cmd/root.go`

- [ ] **Step 1: Add bubbletea ecosystem dependencies**

Run:
```bash
cd c:/Users/I758128/Documents/Projects/Stackup && go get github.com/charmbracelet/bubbletea@latest github.com/charmbracelet/lipgloss@latest github.com/charmbracelet/bubbles@latest
```

- [ ] **Step 2: Create `internal/tui/styles.go`**

```go
package tui

import "github.com/charmbracelet/lipgloss"

var (
	colorGreen  = lipgloss.Color("#7ee787")
	colorYellow = lipgloss.Color("#d29922")
	colorRed    = lipgloss.Color("#f85149")
	colorBlue   = lipgloss.Color("#58a6ff")
	colorDim    = lipgloss.Color("#484f58")
	colorWhite  = lipgloss.Color("#c9d1d9")
	colorBg     = lipgloss.Color("#0d1117")
	colorBgAlt  = lipgloss.Color("#161b22")
	colorBorder = lipgloss.Color("#30363d")

	styleHeader = lipgloss.NewStyle().
			Background(lipgloss.Color("#161b22")).
			Foreground(colorWhite).
			Padding(0, 1)

	styleTitleBar = lipgloss.NewStyle().
			Background(lipgloss.Color("#1c2128")).
			Foreground(colorBlue).
			Bold(true).
			Padding(0, 1)

	styleStatusBar = lipgloss.NewStyle().
			Background(lipgloss.Color("#161b22")).
			Foreground(colorDim).
			Padding(0, 1)

	styleSelected = lipgloss.NewStyle().
			Background(lipgloss.Color("#1f2937")).
			Foreground(colorGreen)

	styleHealthy = lipgloss.NewStyle().Foreground(colorGreen)
	styleWarning = lipgloss.NewStyle().Foreground(colorYellow)
	styleFailed  = lipgloss.NewStyle().Foreground(colorRed)
	styleInfo    = lipgloss.NewStyle().Foreground(colorBlue)
	styleDim     = lipgloss.NewStyle().Foreground(colorDim)
	styleBold    = lipgloss.NewStyle().Bold(true).Foreground(colorWhite)

	styleModal = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2).
			Width(36)
)
```

- [ ] **Step 3: Create `internal/tui/messages.go`**

```go
package tui

import (
	"time"

	"github.com/deveshpharswan/stackup/internal/doctor"
)

type ServiceInfo struct {
	Name   string
	State  string
	Health string
	Ports  string
	Tier   int
	Uptime time.Duration
}

type ServiceUpdateMsg struct {
	Services []ServiceInfo
	Err      error
}

type LogLineMsg struct {
	Line string
}

type LogErrMsg struct {
	Err error
}

type DoctorResultMsg struct {
	Findings []doctor.Finding
}

type DoctorRunningMsg struct{}

type ToastMsg struct {
	Text string
}

type ToastExpiredMsg struct{}

type TickMsg time.Time

type ConfirmAction int

const (
	ConfirmRestart ConfirmAction = iota
	ConfirmDelete
)

type ConfirmRequestMsg struct {
	Action  ConfirmAction
	Service string
}

type ConfirmYesMsg struct {
	Action  ConfirmAction
	Service string
}

type ActionResultMsg struct {
	Text string
	Err  error
}
```

- [ ] **Step 4: Create `internal/tui/keys.go`**

```go
package tui

import "github.com/charmbracelet/bubbles/key"

type ServicesKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Restart key.Binding
	Shell   key.Binding
	Logs    key.Binding
	Delete  key.Binding
	Desc    key.Binding
	Doctor  key.Binding
	Graph   key.Binding
	Filter  key.Binding
	Command key.Binding
	Help    key.Binding
	Quit    key.Binding
}

var ServicesKeys = ServicesKeyMap{
	Up:      key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
	Down:    key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
	Restart: key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "restart")),
	Shell:   key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "shell")),
	Logs:    key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "logs")),
	Delete:  key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "stop")),
	Desc:    key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "describe")),
	Doctor:  key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "doctor")),
	Graph:   key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "graph")),
	Filter:  key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "filter")),
	Command: key.NewBinding(key.WithKeys(":"), key.WithHelp(":", "command")),
	Help:    key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
	Quit:    key.NewBinding(key.WithKeys("q"), key.WithHelp("q", "quit")),
}

type LogsKeyMap struct {
	Back       key.Binding
	Timestamps key.Binding
	Wrap       key.Binding
	Search     key.Binding
	Clear      key.Binding
	Top        key.Binding
	Bottom     key.Binding
}

var LogsKeys = LogsKeyMap{
	Back:       key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Timestamps: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "timestamps")),
	Wrap:       key.NewBinding(key.WithKeys("w"), key.WithHelp("w", "wrap")),
	Search:     key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "search")),
	Clear:      key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "clear")),
	Top:        key.NewBinding(key.WithKeys("g"), key.WithHelp("g", "top")),
	Bottom:     key.NewBinding(key.WithKeys("G"), key.WithHelp("G", "bottom")),
}

type DoctorKeyMap struct {
	Back   key.Binding
	Expand key.Binding
	Rescan key.Binding
}

var DoctorKeys = DoctorKeyMap{
	Back:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Expand: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "details")),
	Rescan: key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "re-scan")),
}

type GraphKeyMap struct {
	Back   key.Binding
	Select key.Binding
}

var GraphKeys = GraphKeyMap{
	Back:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	Select: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select")),
}
```

- [ ] **Step 5: Create `internal/tui/tui.go` — root model skeleton**

```go
package tui

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ViewType int

const (
	ViewServices ViewType = iota
	ViewLogs
	ViewDoctor
	ViewGraph
	ViewDescribe
)

type Model struct {
	width  int
	height int

	activeView ViewType
	viewStack  []ViewType

	services ServicesModel
	logs     LogsModel
	doctor   DoctorViewModel
	graph    GraphModel
	describe DescribeModel

	header  HeaderModel
	command CommandModel
	toast   ToastModel
	help    HelpModel
	confirm ConfirmModel

	showHelp    bool
	showConfirm bool

	serviceNames []string
	quitting     bool
}

func NewModel() Model {
	return Model{
		activeView: ViewServices,
		viewStack:  []ViewType{ViewServices},
		services:   NewServicesModel(),
		logs:       NewLogsModel(),
		doctor:     NewDoctorViewModel(),
		graph:      NewGraphModel(),
		describe:   NewDescribeModel(),
		header:     NewHeaderModel(),
		command:    NewCommandModel(),
		toast:      NewToastModel(),
		help:       NewHelpModel(),
		confirm:    NewConfirmModel(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.services.Init(), m.header.Init())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.showConfirm {
			newConfirm, cmd := m.confirm.Update(msg)
			m.confirm = newConfirm
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
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}
		switch msg.String() {
		case "?":
			m.showHelp = true
			return m, nil
		case ":":
			m.command.Activate(ModeCommand)
			return m, nil
		case "q":
			if m.activeView == ViewServices {
				m.quitting = true
				return m, tea.Quit
			}
		case "esc":
			m = m.popView()
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case ToastMsg:
		m.toast = m.toast.Show(msg.Text)
		cmds = append(cmds, m.toast.Tick())

	case ToastExpiredMsg:
		m.toast = m.toast.Hide()

	case ConfirmRequestMsg:
		m.confirm = m.confirm.Request(msg.Action, msg.Service)
		m.showConfirm = true

	case ConfirmYesMsg:
		m.showConfirm = false
	}

	switch m.activeView {
	case ViewServices:
		newSvc, cmd := m.services.Update(msg)
		m.services = newSvc
		cmds = append(cmds, cmd)
	case ViewLogs:
		newLogs, cmd := m.logs.Update(msg)
		m.logs = newLogs
		cmds = append(cmds, cmd)
	case ViewDoctor:
		newDoc, cmd := m.doctor.Update(msg)
		m.doctor = newDoc
		cmds = append(cmds, cmd)
	case ViewGraph:
		newGraph, cmd := m.graph.Update(msg)
		m.graph = newGraph
		cmds = append(cmds, cmd)
	case ViewDescribe:
		newDesc, cmd := m.describe.Update(msg)
		m.describe = newDesc
		cmds = append(cmds, cmd)
	}

	newHeader, cmd := m.header.Update(msg)
	m.header = newHeader
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.width < 80 || m.height < 24 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			styleBold.Render("Terminal too small")+"\n"+
				styleDim.Render(fmt.Sprintf("Need 80x24, got %dx%d", m.width, m.height)))
	}

	contentHeight := m.height - 7 // header(5) + title(1) + statusbar(1)

	var content string
	switch m.activeView {
	case ViewServices:
		content = m.services.View(m.width, contentHeight)
	case ViewLogs:
		content = m.logs.View(m.width, contentHeight)
	case ViewDoctor:
		content = m.doctor.View(m.width, contentHeight)
	case ViewGraph:
		content = m.graph.View(m.width, contentHeight)
	case ViewDescribe:
		content = m.describe.View(m.width, contentHeight)
	}

	view := lipgloss.JoinVertical(lipgloss.Left,
		m.header.View(m.width, m.activeView),
		m.titleBar(),
		content,
		m.statusBar(),
	)

	if m.showHelp {
		view = m.help.View(m.width, m.height, m.activeView)
	}
	if m.showConfirm {
		view = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			m.confirm.View())
	}

	return view
}

func (m Model) titleBar() string {
	var title string
	switch m.activeView {
	case ViewServices:
		filter := "all"
		if f := m.command.Filter(); f != "" {
			filter = "/" + f
		}
		title = fmt.Sprintf("  Services(%s)[%d]", filter, m.services.Count())
	case ViewLogs:
		title = fmt.Sprintf("  Logs: %s", m.logs.ServiceName())
	case ViewDoctor:
		title = "  Doctor"
	case ViewGraph:
		title = "  Graph"
	case ViewDescribe:
		title = fmt.Sprintf("  Describe: %s", m.describe.ServiceName())
	}
	return styleTitleBar.Width(m.width).Render(title)
}

func (m Model) statusBar() string {
	if m.command.Active() {
		return m.command.View(m.width)
	}
	if msg := m.toast.Message(); msg != "" {
		return styleStatusBar.Width(m.width).Render("  " + msg)
	}
	return styleStatusBar.Width(m.width).Render("  Press ? for help")
}

func (m Model) pushView(v ViewType) Model {
	m.viewStack = append(m.viewStack, v)
	m.activeView = v
	return m
}

func (m Model) popView() Model {
	if len(m.viewStack) > 1 {
		m.viewStack = m.viewStack[:len(m.viewStack)-1]
		m.activeView = m.viewStack[len(m.viewStack)-1]
	}
	return m
}

func Run() error {
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
```

- [ ] **Step 6: Create `cmd/ui.go` — Cobra command**

```go
package cmd

import (
	"github.com/spf13/cobra"
	"github.com/deveshpharswan/stackup/internal/tui"
)

func newUICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Interactive terminal dashboard for managing your stack",
		Long:  "Launch a k9s-style TUI with real-time service status, log streaming, diagnostics, and dependency visualization.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run()
		},
	}
	return cmd
}
```

- [ ] **Step 7: Register command in `cmd/root.go`**

Add `newUICmd()` to the `root.AddCommand(...)` call in `cmd/root.go`.

- [ ] **Step 8: Create stub models for compilation**

Create minimal stubs for `ServicesModel`, `LogsModel`, `DoctorViewModel`, `GraphModel`, `DescribeModel`, `HeaderModel`, `CommandModel`, `ToastModel`, `HelpModel`, `ConfirmModel` so that `tui.go` compiles. Each stub implements the interface needed by `tui.go` — the real implementations come in subsequent tasks.

Create `internal/tui/header.go`:
```go
package tui

import tea "github.com/charmbracelet/bubbletea"

type HeaderModel struct{}

func NewHeaderModel() HeaderModel { return HeaderModel{} }
func (m HeaderModel) Init() tea.Cmd { return nil }
func (m HeaderModel) Update(msg tea.Msg) (HeaderModel, tea.Cmd) { return m, nil }
func (m HeaderModel) View(width int, active ViewType) string { return "" }
```

Create `internal/tui/command.go`:
```go
package tui

import tea "github.com/charmbracelet/bubbletea"

type InputMode int

const (
	ModeCommand InputMode = iota
	ModeFilter
)

type CommandModel struct {
	active bool
	mode   InputMode
	filter string
}

func NewCommandModel() CommandModel { return CommandModel{} }
func (m CommandModel) Active() bool  { return m.active }
func (m CommandModel) Filter() string { return m.filter }
func (m *CommandModel) Activate(mode InputMode) { m.active = true; m.mode = mode }
func (m CommandModel) Update(msg tea.Msg) (CommandModel, tea.Cmd) { return m, nil }
func (m CommandModel) View(width int) string { return "" }
```

Create `internal/tui/toast.go`:
```go
package tui

import tea "github.com/charmbracelet/bubbletea"

type ToastModel struct{ message string }

func NewToastModel() ToastModel { return ToastModel{} }
func (m ToastModel) Show(text string) ToastModel { m.message = text; return m }
func (m ToastModel) Hide() ToastModel { m.message = ""; return m }
func (m ToastModel) Message() string { return m.message }
func (m ToastModel) Tick() tea.Cmd { return nil }
```

Create `internal/tui/services.go`:
```go
package tui

import tea "github.com/charmbracelet/bubbletea"

type ServicesModel struct{ count int }

func NewServicesModel() ServicesModel { return ServicesModel{} }
func (m ServicesModel) Init() tea.Cmd { return nil }
func (m ServicesModel) Update(msg tea.Msg) (ServicesModel, tea.Cmd) { return m, nil }
func (m ServicesModel) View(width, height int) string { return "  No services loaded" }
func (m ServicesModel) Count() int { return m.count }
func (m ServicesModel) Selected() string { return "" }
```

Create `internal/tui/logs.go`:
```go
package tui

import tea "github.com/charmbracelet/bubbletea"

type LogsModel struct{ service string }

func NewLogsModel() LogsModel { return LogsModel{} }
func (m LogsModel) Update(msg tea.Msg) (LogsModel, tea.Cmd) { return m, nil }
func (m LogsModel) View(width, height int) string { return "" }
func (m LogsModel) ServiceName() string { return m.service }
```

Create `internal/tui/doctorview.go`:
```go
package tui

import tea "github.com/charmbracelet/bubbletea"

type DoctorViewModel struct{}

func NewDoctorViewModel() DoctorViewModel { return DoctorViewModel{} }
func (m DoctorViewModel) Update(msg tea.Msg) (DoctorViewModel, tea.Cmd) { return m, nil }
func (m DoctorViewModel) View(width, height int) string { return "" }
```

Create `internal/tui/graph.go`:
```go
package tui

import tea "github.com/charmbracelet/bubbletea"

type GraphModel struct{}

func NewGraphModel() GraphModel { return GraphModel{} }
func (m GraphModel) Update(msg tea.Msg) (GraphModel, tea.Cmd) { return m, nil }
func (m GraphModel) View(width, height int) string { return "" }
```

Create `internal/tui/describe.go`:
```go
package tui

import tea "github.com/charmbracelet/bubbletea"

type DescribeModel struct{ service string }

func NewDescribeModel() DescribeModel { return DescribeModel{} }
func (m DescribeModel) Update(msg tea.Msg) (DescribeModel, tea.Cmd) { return m, nil }
func (m DescribeModel) View(width, height int) string { return "" }
func (m DescribeModel) ServiceName() string { return m.service }
```

Create `internal/tui/help.go`:
```go
package tui

type HelpModel struct{}

func NewHelpModel() HelpModel { return HelpModel{} }
func (m HelpModel) View(width, height int, active ViewType) string { return "" }
```

Create `internal/tui/confirm.go`:
```go
package tui

import tea "github.com/charmbracelet/bubbletea"

type ConfirmModel struct {
	action  ConfirmAction
	service string
}

func NewConfirmModel() ConfirmModel { return ConfirmModel{} }
func (m ConfirmModel) Request(action ConfirmAction, service string) ConfirmModel {
	m.action = action
	m.service = service
	return m
}
func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) { return m, nil }
func (m ConfirmModel) View() string { return "" }
```

- [ ] **Step 9: Verify build compiles**

Run:
```bash
go vet ./... && go build ./...
```
Expected: clean compilation, no errors.

- [ ] **Step 10: Run existing tests to verify no regressions**

Run:
```bash
go test ./internal/config/... ./internal/orchestrator/... ./internal/env/... ./internal/scaffold/... ./cmd/...
```
Expected: all existing tests pass.

- [ ] **Step 11: Commit**

```bash
git add internal/tui/ cmd/ui.go cmd/root.go go.mod go.sum
git commit -m "feat(tui): scaffold stackup ui with bubbletea foundation

Add bubbletea/lipgloss/bubbles dependencies. Create internal/tui package
with root model, styles, messages, keybindings, and stub sub-models.
Register 'stackup ui' cobra command. All existing tests pass."
```

---

## Task 2: Header Component

**Files:**
- Modify: `internal/tui/header.go`

- [ ] **Step 1: Implement HeaderModel with stack metadata**

Replace the stub in `internal/tui/header.go`:

```go
package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type HeaderModel struct {
	stack      string
	compose    string
	tiers      int
	healthy    int
	total      int
	startTime  time.Time
}

func NewHeaderModel() HeaderModel {
	return HeaderModel{
		stack:     currentDirName(),
		compose:   "docker-compose.yml",
		startTime: time.Now(),
	}
}

func (m HeaderModel) Init() tea.Cmd { return nil }

func (m HeaderModel) Update(msg tea.Msg) (HeaderModel, tea.Cmd) {
	switch msg := msg.(type) {
	case ServiceUpdateMsg:
		if msg.Err == nil {
			m.total = len(msg.Services)
			healthy := 0
			for _, s := range msg.Services {
				if s.Health == "healthy" {
					healthy++
				}
			}
			m.healthy = healthy
		}
	}
	return m, nil
}

func (m HeaderModel) View(width int, active ViewType) string {
	uptime := time.Since(m.startTime).Truncate(time.Second)

	healthStr := fmt.Sprintf("%d/%d", m.healthy, m.total)
	if m.healthy == m.total && m.total > 0 {
		healthStr = styleHealthy.Render(healthStr + " ✓")
	} else {
		healthStr = styleWarning.Render(healthStr)
	}

	left := strings.Join([]string{
		styleHealthy.Render("Stack:") + "   " + m.stack,
		styleHealthy.Render("Compose:") + " " + m.compose,
		styleHealthy.Render("Tiers:") + "   " + fmt.Sprintf("%d", m.tiers),
		styleHealthy.Render("Health:") + "  " + healthStr,
		styleHealthy.Render("Uptime:") + "  " + formatUptime(uptime),
	}, "\n")

	shortcuts := shortcutsForView(active)

	logo := styleDim.Render("╔═══════╗\n║STACKUP║\n╚═══════╝")

	leftCol := lipgloss.NewStyle().Width(22).Render(left)
	midCol := lipgloss.NewStyle().Width(width - 22 - 14).Render(shortcuts)
	rightCol := lipgloss.NewStyle().Width(12).Align(lipgloss.Right).Render(logo)

	row := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, midCol, rightCol)
	return styleHeader.Width(width).Render(row)
}

func shortcutsForView(v ViewType) string {
	switch v {
	case ViewServices:
		return strings.Join([]string{
			styleInfo.Render("<?>") + " Help     " + styleInfo.Render("<r>") + " Restart",
			styleInfo.Render("<l>") + " Logs     " + styleInfo.Render("<s>") + " Shell",
			styleInfo.Render("<d>") + " Doctor   " + styleInfo.Render("<x>") + " Stop",
			styleInfo.Render("<g>") + " Graph    " + styleInfo.Render("</>") + " Filter",
			styleInfo.Render("<:>") + " Command  " + styleInfo.Render("<q>") + " Quit",
		}, "\n")
	case ViewLogs:
		return strings.Join([]string{
			styleInfo.Render("<esc>") + " Back   " + styleInfo.Render("<t>") + " Timestamps",
			styleInfo.Render("<w>") + " Wrap     " + styleInfo.Render("</>") + " Search",
			styleInfo.Render("<c>") + " Clear    " + styleInfo.Render("<G>") + " Bottom",
		}, "\n")
	case ViewDoctor:
		return strings.Join([]string{
			styleInfo.Render("<esc>") + " Back     " + styleInfo.Render("<enter>") + " Details",
			styleInfo.Render("<R>") + " Re-scan",
		}, "\n")
	case ViewGraph:
		return strings.Join([]string{
			styleInfo.Render("<esc>") + " Back     " + styleInfo.Render("<1-9>") + " Focus tier",
			styleInfo.Render("<enter>") + " Select",
		}, "\n")
	case ViewDescribe:
		return strings.Join([]string{
			styleInfo.Render("<esc>") + " Back   " + styleInfo.Render("<l>") + " Logs",
			styleInfo.Render("<r>") + " Restart",
		}, "\n")
	}
	return ""
}

func formatUptime(d time.Duration) string {
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm %02ds", m, s)
}

func currentDirName() string {
	dir, _ := os.Getwd()
	parts := strings.Split(strings.ReplaceAll(dir, "\\", "/"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "stackup"
}
```

Note: add `"os"` to the import list.

- [ ] **Step 2: Verify compilation**

Run:
```bash
go vet ./internal/tui/... && go build ./...
```
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/header.go
git commit -m "feat(tui): implement header component with stack metadata and shortcuts"
```

---

## Task 3: Services Table View with Docker Polling

**Files:**
- Modify: `internal/tui/services.go`
- Create: `internal/tui/services_test.go`

- [ ] **Step 1: Write test for services model update**

Create `internal/tui/services_test.go`:

```go
package tui

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestServicesModel_UpdateWithServices(t *testing.T) {
	m := NewServicesModel()
	msg := ServiceUpdateMsg{
		Services: []ServiceInfo{
			{Name: "api", State: "running", Health: "healthy", Ports: "8080/tcp", Tier: 2, Uptime: 5 * time.Minute},
			{Name: "postgres", State: "running", Health: "healthy", Ports: "5432/tcp", Tier: 1, Uptime: 6 * time.Minute},
			{Name: "worker", State: "running", Health: "starting", Tier: 3},
		},
	}
	m, _ = m.Update(msg)
	assert.Equal(t, 3, m.Count())
	assert.Equal(t, "api", m.Selected())
}

func TestServicesModel_Navigation(t *testing.T) {
	m := NewServicesModel()
	msg := ServiceUpdateMsg{
		Services: []ServiceInfo{
			{Name: "api", State: "running", Health: "healthy"},
			{Name: "postgres", State: "running", Health: "healthy"},
		},
	}
	m, _ = m.Update(msg)

	// Move down
	m, _ = m.Update(keyMsg("j"))
	assert.Equal(t, "postgres", m.Selected())

	// Move up
	m, _ = m.Update(keyMsg("k"))
	assert.Equal(t, "api", m.Selected())
}

func TestServicesModel_Filter(t *testing.T) {
	m := NewServicesModel()
	msg := ServiceUpdateMsg{
		Services: []ServiceInfo{
			{Name: "api", State: "running", Health: "healthy"},
			{Name: "postgres", State: "running", Health: "healthy"},
			{Name: "redis", State: "running", Health: "healthy"},
		},
	}
	m, _ = m.Update(msg)
	m = m.SetFilter("post")
	assert.Equal(t, 1, m.Count())
}

func keyMsg(k string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
}
```

Add `tea "github.com/charmbracelet/bubbletea"` to imports.

- [ ] **Step 2: Implement ServicesModel**

Replace `internal/tui/services.go`:

```go
package tui

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type ServicesModel struct {
	services []ServiceInfo
	filtered []ServiceInfo
	cursor   int
	filter   string
	err      error
}

func NewServicesModel() ServicesModel {
	return ServicesModel{}
}

func (m ServicesModel) Init() tea.Cmd {
	return tea.Batch(pollServices(), tickEvery(2*time.Second))
}

func (m ServicesModel) Update(msg tea.Msg) (ServicesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case ServiceUpdateMsg:
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.services = msg.Services
		m.err = nil
		m.applyFilter()
	case TickMsg:
		return m, tea.Batch(pollServices(), tickEvery(2*time.Second))
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		}
	}
	return m, nil
}

func (m ServicesModel) View(width, height int) string {
	if m.err != nil {
		return styleWarning.Render(fmt.Sprintf("  Docker error: %v\n\n", m.err)) +
			styleDim.Render("  Is Docker running?")
	}
	if len(m.filtered) == 0 {
		return styleDim.Render("  No services found")
	}

	var b strings.Builder
	// Column headers
	header := fmt.Sprintf("  %-16s %-12s %-12s %-20s %-6s %s",
		"NAME", "STATE", "HEALTH", "PORT", "TIER", "UPTIME")
	b.WriteString(styleInfo.Bold(true).Render(header) + "\n")
	b.WriteString(styleDim.Render("  " + strings.Repeat("─", width-4)) + "\n")

	for i, svc := range m.filtered {
		if i >= height-3 {
			break
		}
		row := m.renderRow(svc, i == m.cursor, width)
		b.WriteString(row + "\n")
	}

	// Footer summary
	running, healthy, failed := 0, 0, 0
	for _, s := range m.services {
		switch {
		case s.State == "running" && s.Health == "healthy":
			running++
			healthy++
		case s.State == "running":
			running++
		default:
			failed++
		}
	}
	b.WriteString("\n" + styleDim.Render("  "+strings.Repeat("─", width-4)) + "\n")
	summary := fmt.Sprintf("  %s   %s   %s",
		styleHealthy.Render(fmt.Sprintf("✓ %d healthy", healthy)),
		styleWarning.Render(fmt.Sprintf("⠋ %d starting", running-healthy)),
		styleFailed.Render(fmt.Sprintf("✗ %d failed", failed)))
	b.WriteString(summary)

	return b.String()
}

func (m ServicesModel) renderRow(svc ServiceInfo, selected bool, width int) string {
	tier := fmt.Sprintf("%d", svc.Tier)
	ports := svc.Ports
	if ports == "" {
		ports = "—"
	}
	uptime := formatUptime(svc.Uptime)

	row := fmt.Sprintf("  %-16s %-12s %-12s %-20s %-6s %s",
		svc.Name, svc.State, svc.Health, ports, tier, uptime)

	if selected {
		return styleSelected.Width(width).Render(row)
	}

	switch {
	case svc.State == "running" && svc.Health == "healthy":
		return styleHealthy.Render(row)
	case svc.State == "running" && (svc.Health == "starting" || svc.Health == ""):
		return styleWarning.Render(row)
	case svc.State == "exited" || svc.State == "restarting":
		return styleFailed.Render(row)
	default:
		return styleDim.Render(row)
	}
}

func (m ServicesModel) Count() int {
	return len(m.filtered)
}

func (m ServicesModel) Selected() string {
	if m.cursor < len(m.filtered) {
		return m.filtered[m.cursor].Name
	}
	return ""
}

func (m *ServicesModel) SetFilter(f string) ServicesModel {
	m.filter = f
	m.applyFilter()
	m.cursor = 0
	return *m
}

func (m *ServicesModel) applyFilter() {
	if m.filter == "" {
		m.filtered = m.services
		return
	}
	re, err := regexp.Compile("(?i)" + m.filter)
	if err != nil {
		m.filtered = m.services
		return
	}
	var filtered []ServiceInfo
	for _, s := range m.services {
		if re.MatchString(s.Name) {
			filtered = append(filtered, s)
		}
	}
	m.filtered = filtered
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func pollServices() tea.Cmd {
	return func() tea.Msg {
		c := exec.Command("docker", "compose", "ps", "--format",
			"{{.Service}}\t{{.State}}\t{{.Status}}\t{{.Ports}}")
		out, err := c.Output()
		if err != nil {
			return ServiceUpdateMsg{Err: fmt.Errorf("docker compose ps: %w", err)}
		}

		var services []ServiceInfo
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "\t", 4)
			if len(parts) < 2 {
				continue
			}
			svc := ServiceInfo{
				Name:  strings.TrimSpace(parts[0]),
				State: strings.TrimSpace(parts[1]),
			}
			if len(parts) >= 3 {
				svc.Health = parseHealthStatus(parts[2])
			}
			if len(parts) >= 4 {
				svc.Ports = strings.TrimSpace(parts[3])
			}
			services = append(services, svc)
		}
		return ServiceUpdateMsg{Services: services}
	}
}

func parseHealthStatus(status string) string {
	lower := strings.ToLower(status)
	switch {
	case strings.Contains(lower, "healthy"):
		return "healthy"
	case strings.Contains(lower, "unhealthy"):
		return "unhealthy"
	case strings.Contains(lower, "starting"):
		return "starting"
	default:
		return "(none)"
	}
}

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
```

- [ ] **Step 3: Run tests**

```bash
go test ./internal/tui/... -v
```
Expected: tests pass.

- [ ] **Step 4: Verify full build**

```bash
go vet ./... && go build ./...
```

- [ ] **Step 5: Commit**

```bash
git add internal/tui/services.go internal/tui/services_test.go
git commit -m "feat(tui): implement services table view with Docker polling and filtering"
```

---

## Task 4: Command Input with View Switching

**Files:**
- Modify: `internal/tui/command.go`
- Create: `internal/tui/command_test.go`
- Modify: `internal/tui/tui.go` (wire command results)

- [ ] **Step 1: Write tests for command parsing**

Create `internal/tui/command_test.go`:

```go
package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCommand_Services(t *testing.T) {
	cmd, arg := parseCommand("services")
	assert.Equal(t, "services", cmd)
	assert.Equal(t, "", arg)
}

func TestParseCommand_LogsWithArg(t *testing.T) {
	cmd, arg := parseCommand("logs api")
	assert.Equal(t, "logs", cmd)
	assert.Equal(t, "api", arg)
}

func TestParseCommand_Aliases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"svc", "services"},
		{"l api", "logs"},
		{"doc", "doctor"},
		{"g", "graph"},
		{"desc api", "describe"},
		{"q", "quit"},
	}
	for _, tt := range tests {
		cmd, _ := parseCommand(tt.input)
		resolved := resolveAlias(cmd)
		assert.Equal(t, tt.want, resolved, "input: %s", tt.input)
	}
}

func TestTabComplete(t *testing.T) {
	names := []string{"api", "api-worker", "postgres", "redis"}
	result := tabComplete("ap", names)
	assert.Equal(t, "api", result)

	result = tabComplete("api-", names)
	assert.Equal(t, "api-worker", result)

	result = tabComplete("post", names)
	assert.Equal(t, "postgres", result)
}
```

- [ ] **Step 2: Implement CommandModel**

Replace `internal/tui/command.go`:

```go
package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type InputMode int

const (
	ModeCommand InputMode = iota
	ModeFilter
)

type CommandResult struct {
	View    ViewType
	Arg     string
	IsQuit  bool
	Filter  string
}

type CommandModel struct {
	active       bool
	mode         InputMode
	input        string
	filter       string
	serviceNames []string
}

func NewCommandModel() CommandModel { return CommandModel{} }

func (m CommandModel) Active() bool   { return m.active }
func (m CommandModel) Filter() string { return m.filter }

func (m *CommandModel) Activate(mode InputMode) {
	m.active = true
	m.mode = mode
	m.input = ""
}

func (m *CommandModel) SetServiceNames(names []string) {
	m.serviceNames = names
}

func (m CommandModel) Update(msg tea.Msg) (CommandModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEscape:
			m.active = false
			m.input = ""
			if m.mode == ModeFilter {
				m.filter = ""
			}
			return m, nil
		case tea.KeyEnter:
			m.active = false
			if m.mode == ModeFilter {
				m.filter = m.input
				m.input = ""
				return m, nil
			}
			result := m.execute()
			m.input = ""
			return m, func() tea.Msg { return result }
		case tea.KeyBackspace:
			if len(m.input) > 0 {
				m.input = m.input[:len(m.input)-1]
			}
			if m.mode == ModeFilter {
				m.filter = m.input
			}
			return m, nil
		case tea.KeyTab:
			if m.mode == ModeCommand {
				m.tryComplete()
			}
			return m, nil
		default:
			if msg.Type == tea.KeyRunes {
				m.input += string(msg.Runes)
				if m.mode == ModeFilter {
					m.filter = m.input
				}
			}
		}
	}
	return m, nil
}

func (m CommandModel) View(width int) string {
	prefix := ":"
	if m.mode == ModeFilter {
		prefix = "/"
	}
	prompt := styleWarning.Render(prefix)
	input := lipgloss.NewStyle().
		Background(lipgloss.Color("#21262d")).
		Foreground(colorWhite).
		Width(width - 4).
		Render(m.input + "_")
	return styleStatusBar.Width(width).Render("  " + prompt + input)
}

func (m CommandModel) execute() CommandResult {
	cmd, arg := parseCommand(m.input)
	cmd = resolveAlias(cmd)
	switch cmd {
	case "services":
		return CommandResult{View: ViewServices}
	case "logs":
		return CommandResult{View: ViewLogs, Arg: arg}
	case "doctor":
		return CommandResult{View: ViewDoctor}
	case "graph":
		return CommandResult{View: ViewGraph}
	case "describe":
		return CommandResult{View: ViewDescribe, Arg: arg}
	case "quit":
		return CommandResult{IsQuit: true}
	}
	return CommandResult{View: ViewServices}
}

func (m *CommandModel) tryComplete() {
	_, arg := parseCommand(m.input)
	if arg == "" {
		return
	}
	completed := tabComplete(arg, m.serviceNames)
	if completed != "" {
		parts := strings.SplitN(m.input, " ", 2)
		m.input = parts[0] + " " + completed
	}
}

func parseCommand(input string) (string, string) {
	input = strings.TrimSpace(input)
	parts := strings.SplitN(input, " ", 2)
	cmd := strings.ToLower(parts[0])
	arg := ""
	if len(parts) > 1 {
		arg = strings.TrimSpace(parts[1])
	}
	return cmd, arg
}

func resolveAlias(cmd string) string {
	aliases := map[string]string{
		"svc":  "services",
		"l":    "logs",
		"doc":  "doctor",
		"g":    "graph",
		"desc": "describe",
		"q":    "quit",
	}
	if resolved, ok := aliases[cmd]; ok {
		return resolved
	}
	return cmd
}

func tabComplete(prefix string, candidates []string) string {
	prefix = strings.ToLower(prefix)
	for _, c := range candidates {
		if strings.HasPrefix(strings.ToLower(c), prefix) {
			return c
		}
	}
	return ""
}
```

- [ ] **Step 3: Wire command results in `tui.go`**

In the `Update` method of `Model`, add handling for `CommandResult`:

```go
case CommandResult:
    if msg.IsQuit {
        m.quitting = true
        return m, tea.Quit
    }
    m = m.pushView(msg.View)
    // Additional initialization based on view type
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/tui/... -v
```

- [ ] **Step 5: Commit**

```bash
git add internal/tui/command.go internal/tui/command_test.go internal/tui/tui.go
git commit -m "feat(tui): implement command input with : commands, / filter, tab completion"
```

---

## Task 5: Toast Notifications

**Files:**
- Modify: `internal/tui/toast.go`

- [ ] **Step 1: Implement ToastModel with auto-dismiss**

Replace `internal/tui/toast.go`:

```go
package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type ToastModel struct {
	message string
	visible bool
}

func NewToastModel() ToastModel { return ToastModel{} }

func (m ToastModel) Show(text string) ToastModel {
	m.message = text
	m.visible = true
	return m
}

func (m ToastModel) Hide() ToastModel {
	m.message = ""
	m.visible = false
	return m
}

func (m ToastModel) Message() string {
	if m.visible {
		return m.message
	}
	return ""
}

func (m ToastModel) Tick() tea.Cmd {
	return tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return ToastExpiredMsg{}
	})
}
```

- [ ] **Step 2: Verify build**

```bash
go vet ./internal/tui/... && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/tui/toast.go
git commit -m "feat(tui): implement toast notifications with 3s auto-dismiss"
```

---

## Task 6: Confirmation Modal

**Files:**
- Modify: `internal/tui/confirm.go`

- [ ] **Step 1: Implement ConfirmModel**

Replace `internal/tui/confirm.go`:

```go
package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

type ConfirmModel struct {
	action  ConfirmAction
	service string
	active  bool
}

func NewConfirmModel() ConfirmModel { return ConfirmModel{} }

func (m ConfirmModel) Request(action ConfirmAction, service string) ConfirmModel {
	m.action = action
	m.service = service
	m.active = true
	return m
}

func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			m.active = false
			return m, func() tea.Msg {
				return ConfirmYesMsg{Action: m.action, Service: m.service}
			}
		case "n", "N", "esc":
			m.active = false
			return m, nil
		}
	}
	return m, nil
}

func (m ConfirmModel) View() string {
	var title, desc string
	switch m.action {
	case ConfirmRestart:
		title = fmt.Sprintf("Restart service %q?", m.service)
		desc = "This will restart the container."
	case ConfirmDelete:
		title = fmt.Sprintf("Stop service %q?", m.service)
		desc = "This will stop the container."
	}

	content := styleBold.Render(title) + "\n\n" +
		styleDim.Render(desc) + "\n\n" +
		styleInfo.Render("[y]") + " Confirm   " + styleDim.Render("[n]") + " Cancel"

	return styleModal.Render(content)
}
```

- [ ] **Step 2: Verify build**

```bash
go vet ./internal/tui/... && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/tui/confirm.go
git commit -m "feat(tui): implement confirmation modal for destructive actions"
```

---

## Task 7: Service Actions (Restart, Shell, Delete)

**Files:**
- Modify: `internal/tui/tui.go` (action handlers)
- Modify: `internal/tui/services.go` (action key handling)

- [ ] **Step 1: Add action commands to services Update**

In `internal/tui/services.go`, add key handling in the `Update` switch for `tea.KeyMsg`:

```go
case "r":
    if svc := m.Selected(); svc != "" {
        return m, func() tea.Msg {
            return ConfirmRequestMsg{Action: ConfirmRestart, Service: svc}
        }
    }
case "x":
    if svc := m.Selected(); svc != "" {
        return m, func() tea.Msg {
            return ConfirmRequestMsg{Action: ConfirmDelete, Service: svc}
        }
    }
```

- [ ] **Step 2: Handle ConfirmYesMsg in root model**

In `internal/tui/tui.go`, add to the `Update` switch:

```go
case ConfirmYesMsg:
    m.showConfirm = false
    switch msg.Action {
    case ConfirmRestart:
        return m, restartService(msg.Service)
    case ConfirmDelete:
        return m, stopService(msg.Service)
    }

case ActionResultMsg:
    if msg.Err != nil {
        m.toast = m.toast.Show(styleFailed.Render("Error: " + msg.Err.Error()))
    } else {
        m.toast = m.toast.Show(msg.Text)
    }
    return m, m.toast.Tick()
```

- [ ] **Step 3: Add action helper functions**

Add to `internal/tui/tui.go`:

```go
func restartService(name string) tea.Cmd {
	return func() tea.Msg {
		c := exec.Command("docker", "compose", "restart", name)
		err := c.Run()
		if err != nil {
			return ActionResultMsg{Err: fmt.Errorf("restart %s: %w", name, err)}
		}
		return ActionResultMsg{Text: fmt.Sprintf("Service %q restarted", name)}
	}
}

func stopService(name string) tea.Cmd {
	return func() tea.Msg {
		c := exec.Command("docker", "compose", "stop", name)
		err := c.Run()
		if err != nil {
			return ActionResultMsg{Err: fmt.Errorf("stop %s: %w", name, err)}
		}
		return ActionResultMsg{Text: fmt.Sprintf("Service %q stopped", name)}
	}
}

func shellIntoService(name string) tea.Cmd {
	c := exec.Command("docker", "compose", "exec", name, "sh")
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

Add `"os/exec"` to imports.

- [ ] **Step 4: Handle shell key `s` in services**

In the services `Update`, add:

```go
case "s":
    if svc := m.Selected(); svc != "" {
        return m, func() tea.Msg {
            return shellRequestMsg{Service: svc}
        }
    }
```

Add a new message type in `messages.go`:
```go
type shellRequestMsg struct {
	Service string
}
```

Handle in root model:
```go
case shellRequestMsg:
    return m, shellIntoService(msg.Service)
```

- [ ] **Step 5: Verify build and tests**

```bash
go vet ./... && go build ./... && go test ./internal/tui/...
```

- [ ] **Step 6: Commit**

```bash
git add internal/tui/tui.go internal/tui/services.go internal/tui/messages.go
git commit -m "feat(tui): add service actions — restart, stop, shell with confirmation"
```

---

## Task 8: Logs View with Streaming

**Files:**
- Modify: `internal/tui/logs.go`

- [ ] **Step 1: Implement LogsModel with viewport and streaming**

Replace `internal/tui/logs.go`:

```go
package tui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
)

type LogsModel struct {
	service    string
	viewport   viewport.Model
	lines      []string
	cancel     context.CancelFunc
	timestamps bool
	wrap       bool
	ready      bool
}

func NewLogsModel() LogsModel {
	return LogsModel{}
}

func (m LogsModel) Start(service string, width, height int) (LogsModel, tea.Cmd) {
	vp := viewport.New(width, height)
	vp.SetContent("")
	m.service = service
	m.viewport = vp
	m.lines = nil
	m.ready = true
	m.timestamps = true
	return m, m.streamLogs()
}

func (m LogsModel) Update(msg tea.Msg) (LogsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case LogLineMsg:
		m.lines = append(m.lines, msg.Line)
		if len(m.lines) > 1000 {
			m.lines = m.lines[len(m.lines)-1000:]
		}
		m.viewport.SetContent(m.renderLines())
		m.viewport.GotoBottom()
		return m, nil
	case LogErrMsg:
		m.lines = append(m.lines, styleFailed.Render("Stream error: "+msg.Err.Error()))
		m.viewport.SetContent(m.renderLines())
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "t":
			m.timestamps = !m.timestamps
			m.viewport.SetContent(m.renderLines())
		case "w":
			m.wrap = !m.wrap
		case "c":
			m.lines = nil
			m.viewport.SetContent("")
		case "G":
			m.viewport.GotoBottom()
		case "g":
			m.viewport.GotoTop()
		default:
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 7
	}
	return m, nil
}

func (m LogsModel) View(width, height int) string {
	if !m.ready {
		return styleDim.Render("  No service selected")
	}
	m.viewport.Width = width
	m.viewport.Height = height
	return m.viewport.View()
}

func (m LogsModel) ServiceName() string { return m.service }

func (m *LogsModel) Stop() {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
}

func (m LogsModel) streamLogs() tea.Cmd {
	service := m.service
	return func() tea.Msg {
		ctx, cancel := context.WithCancel(context.Background())
		_ = cancel // stored elsewhere for cleanup

		c := exec.CommandContext(ctx, "docker", "compose", "logs", "-f", "--tail", "100", service)
		stdout, err := c.StdoutPipe()
		if err != nil {
			return LogErrMsg{Err: err}
		}
		if err := c.Start(); err != nil {
			return LogErrMsg{Err: err}
		}

		go func() {
			scanner := bufio.NewScanner(stdout)
			for scanner.Scan() {
				// We'll need a channel approach here — simplified for now
			}
		}()
		return nil
	}
}

func (m LogsModel) renderLines() string {
	var b strings.Builder
	for _, line := range m.lines {
		rendered := m.colorLogLine(line)
		b.WriteString(rendered + "\n")
	}
	return b.String()
}

func (m LogsModel) colorLogLine(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "error") || strings.Contains(lower, "err "):
		return styleFailed.Render(line)
	case strings.Contains(lower, "warn"):
		return styleWarning.Render(line)
	case strings.Contains(lower, "debug"):
		return styleInfo.Render(line)
	default:
		return line
	}
}
```

Note: The streaming implementation here uses a simplified approach. The full implementation will use a `tea.Cmd` that reads from a pipe and sends `LogLineMsg` back via a channel subscription pattern. For the initial commit this establishes the viewport and rendering; log streaming will be refined with a proper `io.Reader` → `tea.Msg` bridge using bubbletea's `tea.Program.Send()`.

- [ ] **Step 2: Wire logs view entry from services**

In `internal/tui/tui.go`, add to the `case "l"` handling or to the `CommandResult` handler:

```go
case ViewLogs:
    newLogs, cmd := m.logs.Start(msg.Arg, m.width, m.height-7)
    m.logs = newLogs
    cmds = append(cmds, cmd)
```

Also handle the `l` key in services view (in root model Update for KeyMsg when activeView is ViewServices):

```go
case "l":
    if svc := m.services.Selected(); svc != "" {
        m = m.pushView(ViewLogs)
        newLogs, cmd := m.logs.Start(svc, m.width, m.height-7)
        m.logs = newLogs
        return m, cmd
    }
```

- [ ] **Step 3: Verify build**

```bash
go vet ./... && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/tui/logs.go internal/tui/tui.go
git commit -m "feat(tui): implement logs view with viewport scrolling and color-coded output"
```

---

## Task 9: Doctor Diagnostics View

**Files:**
- Modify: `internal/tui/doctorview.go`

- [ ] **Step 1: Implement DoctorViewModel**

Replace `internal/tui/doctorview.go`:

```go
package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/deveshpharswan/stackup/internal/constants"
	"github.com/deveshpharswan/stackup/internal/doctor"
)

type DoctorViewModel struct {
	findings []doctor.Finding
	cursor   int
	expanded int
	loading  bool
}

func NewDoctorViewModel() DoctorViewModel {
	return DoctorViewModel{expanded: -1}
}

func (m DoctorViewModel) Init() tea.Cmd {
	return m.runChecks()
}

func (m DoctorViewModel) Update(msg tea.Msg) (DoctorViewModel, tea.Cmd) {
	switch msg := msg.(type) {
	case DoctorResultMsg:
		m.findings = msg.Findings
		m.loading = false
		m.cursor = 0
	case DoctorRunningMsg:
		m.loading = true
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.findings)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "enter":
			if m.expanded == m.cursor {
				m.expanded = -1
			} else {
				m.expanded = m.cursor
			}
		case "R":
			m.loading = true
			return m, m.runChecks()
		}
	}
	return m, nil
}

func (m DoctorViewModel) View(width, height int) string {
	if m.loading {
		return styleWarning.Render("  ⠋ Running diagnostics...")
	}
	if len(m.findings) == 0 {
		return styleHealthy.Render("  ✓ No issues found")
	}

	var b strings.Builder

	// Column header
	header := fmt.Sprintf("  %-4s %-18s %-12s %-30s %s",
		"SEV", "CHECK", "SERVICE", "FINDING", "FIX")
	b.WriteString(styleInfo.Bold(true).Render(header) + "\n")
	b.WriteString(styleDim.Render("  " + strings.Repeat("─", width-4)) + "\n")

	for i, f := range m.findings {
		if i >= height-4 {
			break
		}
		row := m.renderFinding(f, i == m.cursor, width)
		b.WriteString(row + "\n")
		if i == m.expanded {
			if f.Detail != "" {
				b.WriteString(styleDim.Render("    Detail: "+f.Detail) + "\n")
			}
			if f.Fix != "" {
				b.WriteString(styleInfo.Render("    Fix: "+f.Fix) + "\n")
			}
		}
	}

	// Summary
	errors, warnings, oks := 0, 0, 0
	for _, f := range m.findings {
		switch f.Severity {
		case doctor.SeverityError:
			errors++
		case doctor.SeverityWarning:
			warnings++
		case doctor.SeverityOK:
			oks++
		}
	}
	b.WriteString("\n" + styleDim.Render("  "+strings.Repeat("─", width-4)) + "\n")
	b.WriteString(fmt.Sprintf("  %s  %s  %s",
		styleFailed.Render(fmt.Sprintf("%d errors", errors)),
		styleWarning.Render(fmt.Sprintf("%d warnings", warnings)),
		styleHealthy.Render(fmt.Sprintf("%d ok", oks))))

	return b.String()
}

func (m DoctorViewModel) renderFinding(f doctor.Finding, selected bool, width int) string {
	var sevIcon string
	var sevStyle lipgloss.Style
	switch f.Severity {
	case doctor.SeverityError:
		sevIcon = "✗"
		sevStyle = styleFailed
	case doctor.SeverityWarning:
		sevIcon = "!"
		sevStyle = styleWarning
	case doctor.SeverityOK:
		sevIcon = "✓"
		sevStyle = styleHealthy
	}

	svc := f.Service
	if svc == "" {
		svc = "—"
	}
	fix := f.Fix
	if len(fix) > 30 {
		fix = fix[:27] + "..."
	}
	title := f.Title
	if len(title) > 30 {
		title = title[:27] + "..."
	}

	row := fmt.Sprintf("  %-4s %-18s %-12s %-30s %s",
		sevIcon, "", svc, title, fix)

	if selected {
		return styleSelected.Width(width).Render(row)
	}
	return sevStyle.Render(row)
}

func (m DoctorViewModel) runChecks() tea.Cmd {
	return func() tea.Msg {
		d := doctor.New()
		opts := &doctor.Options{
			ComposeFile: constants.DefaultComposeFile,
			EnvFile:     constants.DefaultEnvFile,
			ExampleFile: constants.DefaultExampleFile,
			ConfigFile:  constants.DefaultConfigFile,
		}
		findings := d.Run(context.Background(), opts)
		return DoctorResultMsg{Findings: findings}
	}
}
```

Add `"github.com/charmbracelet/lipgloss"` to imports.

- [ ] **Step 2: Wire doctor view entry in root model**

In `tui.go`, when switching to ViewDoctor, call Init:

```go
case ViewDoctor:
    m = m.pushView(ViewDoctor)
    return m, m.doctor.Init()
```

- [ ] **Step 3: Verify build**

```bash
go vet ./... && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/tui/doctorview.go internal/tui/tui.go
git commit -m "feat(tui): implement doctor diagnostics view with re-scan and detail expand"
```

---

## Task 10: Dependency Graph View

**Files:**
- Modify: `internal/tui/graph.go`

- [ ] **Step 1: Implement GraphModel with ASCII DAG rendering**

Replace `internal/tui/graph.go`:

```go
package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/deveshpharswan/stackup/internal/config"
	"github.com/deveshpharswan/stackup/internal/constants"
	"github.com/deveshpharswan/stackup/internal/orchestrator"
	"github.com/deveshpharswan/stackup/internal/scaffold"
)

type GraphModel struct {
	tiers       []orchestrator.Tier
	deps        map[string][]string
	services    []ServiceInfo
	focusTier   int
	rendered    string
}

func NewGraphModel() GraphModel {
	return GraphModel{focusTier: -1}
}

func (m GraphModel) Init() tea.Cmd {
	return func() tea.Msg {
		composeSvcs, err := scaffold.ParseServices(constants.DefaultComposeFile)
		if err != nil {
			return graphDataMsg{err: err}
		}
		tiers, err := orchestrator.BuildTiers(composeSvcs)
		if err != nil {
			return graphDataMsg{err: err}
		}
		return graphDataMsg{tiers: tiers, deps: composeSvcs}
	}
}

type graphDataMsg struct {
	tiers []orchestrator.Tier
	deps  map[string][]string
	err   error
}

func (m GraphModel) Update(msg tea.Msg) (GraphModel, tea.Cmd) {
	switch msg := msg.(type) {
	case graphDataMsg:
		if msg.err == nil {
			m.tiers = msg.tiers
			m.deps = msg.deps
		}
	case ServiceUpdateMsg:
		m.services = msg.Services
	case tea.KeyMsg:
		switch msg.String() {
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			tier := int(msg.Runes[0] - '0')
			if tier <= len(m.tiers) {
				m.focusTier = tier
			}
		case "0":
			m.focusTier = -1
		}
	}
	return m, nil
}

func (m GraphModel) View(width, height int) string {
	if len(m.tiers) == 0 {
		return styleDim.Render("  No dependency data available")
	}

	var b strings.Builder

	// Render tiers left to right
	b.WriteString("\n")
	for i, tier := range m.tiers {
		tierNum := i + 1
		focused := m.focusTier == tierNum || m.focusTier == -1
		label := fmt.Sprintf("Tier %d", tierNum)
		if focused {
			b.WriteString(styleDim.Render(fmt.Sprintf("        %-20s", label)))
		} else {
			b.WriteString(styleDim.Render(fmt.Sprintf("        %-20s", label)))
		}
	}
	b.WriteString("\n")
	for range m.tiers {
		b.WriteString(styleDim.Render("       ──────────────      "))
	}
	b.WriteString("\n\n")

	// Render service boxes per tier
	maxServices := 0
	for _, tier := range m.tiers {
		if len(tier) > maxServices {
			maxServices = len(tier)
		}
	}

	for row := 0; row < maxServices; row++ {
		line1 := ""
		line2 := ""
		line3 := ""
		for i, tier := range m.tiers {
			focused := m.focusTier == i+1 || m.focusTier == -1
			if row < len(tier) {
				svc := tier[row]
				health := m.healthFor(svc)
				icon := m.iconFor(health)
				boxStyle := m.styleFor(health, focused)

				top := "  ┌──────────────┐  "
				mid := fmt.Sprintf("  │ %s %-10s │  ", icon, svc)
				bot := "  └──────────────┘  "

				if !focused {
					top = styleDim.Render(top)
					mid = styleDim.Render(mid)
					bot = styleDim.Render(bot)
				} else {
					top = boxStyle.Render(top)
					mid = boxStyle.Render(mid)
					bot = boxStyle.Render(bot)
				}
				line1 += top
				line2 += mid
				line3 += bot
			} else {
				pad := strings.Repeat(" ", 20)
				line1 += pad
				line2 += pad
				line3 += pad
			}
		}
		b.WriteString(line1 + "\n")
		b.WriteString(line2 + "\n")
		b.WriteString(line3 + "\n")

		// Draw arrows between tiers
		if row == 0 && len(m.tiers) > 1 {
			// simplified arrow rendering
		}
	}

	// Legend
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  Legend: "))
	b.WriteString(styleHealthy.Render("◉ healthy") + "  ")
	b.WriteString(styleWarning.Render("◎ starting") + "  ")
	b.WriteString(styleFailed.Render("◈ failed") + "  ")
	b.WriteString(styleDim.Render("──▸ depends on"))
	b.WriteString("\n\n")

	// Startup order
	var order []string
	for _, tier := range m.tiers {
		order = append(order, strings.Join(tier, ", "))
	}
	b.WriteString(styleDim.Render("  Startup order: " + strings.Join(order, " → ")))

	return b.String()
}

func (m GraphModel) healthFor(name string) string {
	for _, s := range m.services {
		if s.Name == name {
			return s.Health
		}
	}
	return "(none)"
}

func (m GraphModel) iconFor(health string) string {
	switch health {
	case "healthy":
		return styleHealthy.Render("◉")
	case "starting":
		return styleWarning.Render("◎")
	case "unhealthy":
		return styleFailed.Render("◈")
	default:
		return styleDim.Render("○")
	}
}

func (m GraphModel) styleFor(health string, focused bool) lipgloss.Style {
	if !focused {
		return styleDim
	}
	switch health {
	case "healthy":
		return styleHealthy
	case "starting":
		return styleWarning
	case "unhealthy":
		return styleFailed
	default:
		return styleDim
	}
}
```

Add `"github.com/charmbracelet/lipgloss"` to imports.

- [ ] **Step 2: Wire graph view entry in root model**

When switching to ViewGraph:
```go
case ViewGraph:
    m = m.pushView(ViewGraph)
    return m, m.graph.Init()
```

- [ ] **Step 3: Verify build**

```bash
go vet ./... && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/tui/graph.go internal/tui/tui.go
git commit -m "feat(tui): implement dependency graph view with tier-based ASCII DAG"
```

---

## Task 11: Service Describe View

**Files:**
- Modify: `internal/tui/describe.go`

- [ ] **Step 1: Implement DescribeModel**

Replace `internal/tui/describe.go`:

```go
package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/deveshpharswan/stackup/internal/config"
	"github.com/deveshpharswan/stackup/internal/constants"
)

type DescribeModel struct {
	service  string
	viewport viewport.Model
	content  string
	ready    bool
}

func NewDescribeModel() DescribeModel {
	return DescribeModel{}
}

func (m DescribeModel) Start(service string, services []ServiceInfo, width, height int) (DescribeModel, tea.Cmd) {
	vp := viewport.New(width, height)
	m.service = service
	m.viewport = vp
	m.ready = true

	cfg := config.LoadOrEmpty(constants.DefaultConfigFile)
	m.content = m.buildContent(service, services, cfg)
	m.viewport.SetContent(m.content)
	return m, nil
}

func (m DescribeModel) Update(msg tea.Msg) (DescribeModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - 7
	}
	return m, nil
}

func (m DescribeModel) View(width, height int) string {
	if !m.ready {
		return styleDim.Render("  No service selected")
	}
	m.viewport.Width = width
	m.viewport.Height = height
	return m.viewport.View()
}

func (m DescribeModel) ServiceName() string { return m.service }

func (m DescribeModel) buildContent(name string, services []ServiceInfo, cfg *config.Config) string {
	var b strings.Builder

	// Find service info
	var info ServiceInfo
	for _, s := range services {
		if s.Name == name {
			info = s
			break
		}
	}

	b.WriteString(styleBold.Render("  Service: ") + styleHealthy.Render(name) + "\n")
	b.WriteString(styleBold.Render("  State:   ") + m.stateStr(info) + "\n")
	if info.Ports != "" {
		b.WriteString(styleBold.Render("  Ports:   ") + info.Ports + "\n")
	}
	b.WriteString(styleBold.Render("  Uptime:  ") + formatUptime(info.Uptime) + "\n")
	b.WriteString("\n")

	// Health check config
	if svcCfg, ok := cfg.Services[name]; ok && svcCfg.Health != nil {
		hc := svcCfg.Health
		b.WriteString(styleBold.Render("  Health Check:") + "\n")
		b.WriteString(fmt.Sprintf("    Type:     %s\n", hc.Type))
		if hc.URL != "" {
			b.WriteString(fmt.Sprintf("    URL:      %s\n", hc.URL))
		}
		if hc.Host != "" {
			b.WriteString(fmt.Sprintf("    Host:     %s:%d\n", hc.Host, hc.Port))
		}
		if hc.Pattern != "" {
			b.WriteString(fmt.Sprintf("    Pattern:  %s\n", hc.Pattern))
		}
		if hc.Interval != "" {
			b.WriteString(fmt.Sprintf("    Interval: %s\n", hc.Interval))
		}
		if hc.Timeout != "" {
			b.WriteString(fmt.Sprintf("    Timeout:  %s\n", hc.Timeout))
		}
		b.WriteString("\n")

		// Hooks
		if svcCfg.Hooks != nil && len(svcCfg.Hooks.AfterStart) > 0 {
			b.WriteString(styleBold.Render("  Hooks:") + "\n")
			b.WriteString(fmt.Sprintf("    after_start: %v\n", svcCfg.Hooks.AfterStart))
			b.WriteString("\n")
		}
	}

	// Dependencies (from compose)
	composeSvcs, err := scaffold.ParseServices(constants.DefaultComposeFile)
	if err == nil {
		if deps, ok := composeSvcs[name]; ok && len(deps) > 0 {
			b.WriteString(styleBold.Render("  Depends On:") + "\n")
			for _, dep := range deps {
				b.WriteString(fmt.Sprintf("    %s\n", dep))
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (m DescribeModel) stateStr(info ServiceInfo) string {
	state := info.State
	if info.Health != "" && info.Health != "(none)" {
		state += " (" + info.Health + ")"
	}
	switch {
	case info.Health == "healthy":
		return styleHealthy.Render(state)
	case info.State == "running":
		return styleWarning.Render(state)
	default:
		return styleFailed.Render(state)
	}
}
```

Add `"github.com/deveshpharswan/stackup/internal/scaffold"` to imports.

- [ ] **Step 2: Wire describe view entry**

In root model, handle `Enter` key in services and the `:describe` command:

```go
// In services key handling
case "enter":
    if svc := m.services.Selected(); svc != "" {
        m = m.pushView(ViewDescribe)
        newDesc, cmd := m.describe.Start(svc, m.services.services, m.width, m.height-7)
        m.describe = newDesc
        return m, cmd
    }
```

- [ ] **Step 3: Verify build**

```bash
go vet ./... && go build ./...
```

- [ ] **Step 4: Commit**

```bash
git add internal/tui/describe.go internal/tui/tui.go
git commit -m "feat(tui): implement service describe view with config and dependency details"
```

---

## Task 12: Help Overlay

**Files:**
- Modify: `internal/tui/help.go`

- [ ] **Step 1: Implement HelpModel with context-aware keybinding display**

Replace `internal/tui/help.go`:

```go
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type HelpModel struct{}

func NewHelpModel() HelpModel { return HelpModel{} }

func (m HelpModel) View(width, height int, active ViewType) string {
	title := styleBold.Render("Keyboard Shortcuts")
	subtitle := styleDim.Render("Press ? or Esc to close")

	var bindings [][]string

	switch active {
	case ViewServices:
		bindings = [][]string{
			{"↑/k", "Move up"},
			{"↓/j", "Move down"},
			{"enter", "Describe service"},
			{"l", "View logs"},
			{"r", "Restart service"},
			{"s", "Shell into container"},
			{"x", "Stop service"},
			{"d", "Doctor diagnostics"},
			{"g", "Dependency graph"},
			{"/", "Filter services"},
			{":", "Command mode"},
			{"?", "This help"},
			{"q", "Quit"},
		}
	case ViewLogs:
		bindings = [][]string{
			{"esc", "Back to services"},
			{"t", "Toggle timestamps"},
			{"w", "Toggle wrap"},
			{"/", "Search in logs"},
			{"c", "Clear viewport"},
			{"g", "Jump to top"},
			{"G", "Jump to bottom"},
			{"PgUp", "Page up"},
			{"PgDn", "Page down"},
		}
	case ViewDoctor:
		bindings = [][]string{
			{"esc", "Back to services"},
			{"↑/k", "Move up"},
			{"↓/j", "Move down"},
			{"enter", "Expand/collapse detail"},
			{"R", "Re-run diagnostics"},
		}
	case ViewGraph:
		bindings = [][]string{
			{"esc", "Back to services"},
			{"1-9", "Focus tier N"},
			{"0", "Show all tiers"},
			{"enter", "Select service"},
		}
	case ViewDescribe:
		bindings = [][]string{
			{"esc", "Back"},
			{"l", "View logs"},
			{"r", "Restart service"},
		}
	}

	// Render in two columns
	var left, right []string
	mid := (len(bindings) + 1) / 2
	for i, b := range bindings {
		entry := fmt.Sprintf("  %s  %s",
			styleInfo.Render(fmt.Sprintf("%-8s", b[0])),
			b[1])
		if i < mid {
			left = append(left, entry)
		} else {
			right = append(right, entry)
		}
	}

	leftCol := strings.Join(left, "\n")
	rightCol := strings.Join(right, "\n")

	columns := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(width/2).Render(leftCol),
		lipgloss.NewStyle().Width(width/2).Render(rightCol),
	)

	commands := styleDim.Render("\n  Commands: :services :logs <name> :doctor :graph :describe <name> :quit")

	content := title + "\n" + subtitle + "\n\n" + columns + "\n" + commands

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(1, 2).
		Width(width - 4).
		Height(height - 4).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
```

- [ ] **Step 2: Verify build**

```bash
go vet ./... && go build ./...
```

- [ ] **Step 3: Commit**

```bash
git add internal/tui/help.go
git commit -m "feat(tui): implement help overlay with context-aware keybinding display"
```

---

## Task 13: Integration, Error Handling, and Final Polish

**Files:**
- Modify: `internal/tui/tui.go` (final wiring, NO_COLOR, terminal size)
- Modify: `cmd/root.go` (ensure newUICmd registered)
- Create: `internal/tui/tui_test.go`
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Add NO_COLOR support to styles**

In `internal/tui/styles.go`, add at the top of the file:

```go
func init() {
	if os.Getenv("NO_COLOR") != "" {
		lipgloss.SetColorProfile(termenv.Ascii)
	}
}
```

Add `"os"` and `"github.com/muesli/termenv"` to imports.

- [ ] **Step 2: Create root model test**

Create `internal/tui/tui_test.go`:

```go
package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestModel_InitialView(t *testing.T) {
	m := NewModel()
	assert.Equal(t, ViewServices, m.activeView)
}

func TestModel_ViewStack(t *testing.T) {
	m := NewModel()
	m = m.pushView(ViewDoctor)
	assert.Equal(t, ViewDoctor, m.activeView)
	assert.Len(t, m.viewStack, 2)

	m = m.popView()
	assert.Equal(t, ViewServices, m.activeView)
	assert.Len(t, m.viewStack, 1)
}

func TestModel_PopViewAtRoot(t *testing.T) {
	m := NewModel()
	m = m.popView()
	assert.Equal(t, ViewServices, m.activeView)
	assert.Len(t, m.viewStack, 1)
}

func TestModel_TerminalTooSmall(t *testing.T) {
	m := NewModel()
	m.width = 40
	m.height = 10
	view := m.View()
	assert.Contains(t, view, "Terminal too small")
}

func TestModel_QuitOnQ(t *testing.T) {
	m := NewModel()
	m.width = 100
	m.height = 30
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	model := newModel.(Model)
	assert.True(t, model.quitting)
	assert.NotNil(t, cmd)
}

func TestModel_HelpToggle(t *testing.T) {
	m := NewModel()
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

- [ ] **Step 3: Run all tests**

```bash
go test ./internal/tui/... -v
```
Expected: all pass.

- [ ] **Step 4: Run full project verification**

```bash
go vet ./... && go build ./...
```

- [ ] **Step 5: Run existing tests to verify no regressions**

```bash
go test ./internal/config/... ./internal/orchestrator/... ./internal/env/... ./internal/scaffold/... ./cmd/...
```
Expected: all existing tests pass unchanged.

- [ ] **Step 6: Update CHANGELOG.md**

Add under `## [Unreleased]` → `### Added`:

```markdown
- Interactive terminal UI (`stackup ui`) — k9s-style dashboard with bubbletea
- Services view: real-time table with Docker polling, row selection, color-coded health
- Logs view: streaming container logs with viewport scrolling, search, timestamps toggle
- Doctor view: diagnostic findings table with re-scan and detail expansion
- Graph view: ASCII dependency DAG showing tier structure and health status
- Describe view: service detail panel showing config, health checks, dependencies
- Command mode (`:services`, `:logs <name>`, `:doctor`, `:graph`, `:describe <name>`, `:quit`)
- Filter mode (`/regex`) for table views
- Service actions: restart, stop, shell (with confirmation modal for destructive ops)
- Help overlay (`?`) with context-sensitive keybinding reference
- Toast notifications for action feedback (3s auto-dismiss)
```

- [ ] **Step 7: Final commit**

```bash
git add internal/tui/tui_test.go internal/tui/styles.go internal/tui/tui.go CHANGELOG.md
git commit -m "feat(tui): complete stackup ui with tests, NO_COLOR support, and changelog

Interactive k9s-style terminal UI with services table, log streaming,
doctor diagnostics, dependency graph, and service describe views.
Keyboard-driven with : commands, / filter, and vim navigation."
```

---

## Verification Checklist

After all tasks are complete, verify:

1. `go vet ./...` — no issues
2. `go build ./...` — compiles cleanly
3. `go test ./internal/tui/... -v` — all TUI tests pass
4. `go test ./...` — all existing tests still pass (no regressions)
5. `stackup ui` command appears in `stackup --help` output
6. NO_COLOR=1 disables all styling
7. Terminal < 80x24 shows "Terminal too small" message
