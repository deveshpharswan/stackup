package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deveshpharswan/stackup/internal/config"
	"github.com/deveshpharswan/stackup/internal/constants"
	"github.com/deveshpharswan/stackup/internal/scaffold"
	dockerclient "github.com/docker/docker/client"
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

	services     ServicesModel
	sidebar      SidebarModel
	detail       DetailModel
	logTail      LogsModel
	statsHistory map[string]*StatsHistory
	startedAt    map[string]time.Time
	tierMap      map[string]int

	// Cached config data (loaded once at startup)
	cachedHealthDescs map[string]string
	cachedDeps        map[string][]string

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

	quitting bool
}

func NewModel(dc *dockerclient.Client) Model {
	healthDescs := make(map[string]string)
	deps := make(map[string][]string)

	cfg, _ := config.LoadOrEmpty(constants.DefaultConfigFile)
	if cfg != nil {
		for name, svcCfg := range cfg.Services {
			if svcCfg.Health != nil {
				hc := svcCfg.Health
				desc := hc.Type
				if hc.URL != "" {
					desc += " " + hc.URL
				} else if hc.Host != "" {
					desc += fmt.Sprintf(" %s:%d", hc.Host, hc.Port)
				} else if hc.Pattern != "" {
					desc += " \"" + hc.Pattern + "\""
				}
				healthDescs[name] = desc
			}
		}
	}

	composePath := constants.FindComposeFile(".")
	if composePath == "" {
		composePath = constants.DefaultComposeFile
	}
	composeSvcs, err := scaffold.ParseServices(composePath)
	if err == nil {
		deps = composeSvcs
	}

	return Model{
		activeView:        ViewServices,
		viewStack:         []ViewType{ViewServices},
		services:          NewServicesModel(dc),
		sidebar:           NewSidebarModel(),
		detail:            NewDetailModel(),
		logTail:           NewLogsModel(),
		statsHistory:      make(map[string]*StatsHistory),
		startedAt:         make(map[string]time.Time),
		tierMap:           make(map[string]int),
		cachedHealthDescs: healthDescs,
		cachedDeps:        deps,
		logs:              NewLogsModel(),
		doctor:            NewDoctorViewModel(),
		graph:             NewGraphModel(),
		describe:          NewDescribeModel(),
		header:            NewHeaderModel(),
		command:           NewCommandModel(),
		toast:             NewToastModel(),
		help:              NewHelpModel(),
		confirm:           NewConfirmModel(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.services.Init(), m.graph.Init(), uiTick())
}

func uiTick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return uiTickMsg(t)
	})
}

func (m Model) isWide() bool {
	return m.width >= minWidthWide
}

func (m Model) updateDetail(svc *ServiceInfo) DetailModel {
	d := m.detail.SetService(svc, m.statsHistory)
	if svc != nil {
		d = d.SetServiceMeta(m.cachedDeps[svc.Name], m.cachedHealthDescs[svc.Name])
	}
	return d
}

func (m *Model) applyTiers() {
	if len(m.tierMap) == 0 {
		return
	}
	for i, s := range m.sidebar.services {
		if tier, ok := m.tierMap[s.Name]; ok {
			m.sidebar.services[i].Tier = tier
		}
	}
}

// selectedService returns the name of the currently selected service,
// using sidebar in wide mode or services model in narrow mode.
func (m Model) selectedService() string {
	if m.isWide() {
		return m.sidebar.Selected()
	}
	return m.services.Selected()
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
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
		case "/":
			m.command.Activate(ModeFilter)
			return m, nil
		case "q":
			if m.activeView == ViewServices {
				m.quitting = true
				return m, tea.Quit
			}
		case "esc":
			if m.activeView == ViewLogs {
				m.logs.Stop()
			}
			m = m.popView()
			return m, nil
		case "d":
			if m.activeView == ViewServices {
				m = m.pushView(ViewDoctor)
				return m, m.doctor.Init()
			}
		case "l":
			if m.activeView == ViewServices {
				if svc := m.selectedService(); svc != "" {
					m.logs.Stop()
					m = m.pushView(ViewLogs)
					newLogs, cmd := m.logs.Start(svc, m.width, m.height-headerLines-footerLines)
					m.logs = newLogs
					return m, cmd
				}
			}
		case "g":
			if m.activeView == ViewServices {
				m = m.pushView(ViewGraph)
				return m, m.graph.Init()
			}
		case "e":
			if m.activeView == ViewServices {
				m.services = m.services.ToggleErrorZoom()
				return m, nil
			}
		case "enter":
			if m.activeView == ViewServices {
				if svc := m.selectedService(); svc != "" {
					m.logs.Stop()
					m = m.pushView(ViewLogs)
					newLogs, cmd := m.logs.Start(svc, m.width, m.height-headerLines-footerLines)
					m.logs = newLogs
					return m, cmd
				}
			}
		case "j", "down":
			if m.activeView == ViewServices && m.isWide() {
				newSidebar, cmd := m.sidebar.Update(msg)
				m.sidebar = newSidebar
				cmds = append(cmds, cmd)
				return m, tea.Batch(cmds...)
			}
		case "k", "up":
			if m.activeView == ViewServices && m.isWide() {
				newSidebar, cmd := m.sidebar.Update(msg)
				m.sidebar = newSidebar
				cmds = append(cmds, cmd)
				return m, tea.Batch(cmds...)
			}
		case "r":
			if m.activeView == ViewServices {
				if svc := m.selectedService(); svc != "" {
					return m, func() tea.Msg {
						return ConfirmRequestMsg{Action: ConfirmRestart, Service: svc}
					}
				}
			}
		case "s":
			if m.activeView == ViewServices {
				if svc := m.selectedService(); svc != "" {
					return m, func() tea.Msg {
						return shellRequestMsg{Service: svc}
					}
				}
			}
		case "x":
			if m.activeView == ViewServices {
				if svc := m.selectedService(); svc != "" {
					return m, func() tea.Msg {
						return ConfirmRequestMsg{Action: ConfirmDelete, Service: svc}
					}
				}
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		layout := ComputeLayout(m.width, m.height)
		m.sidebar = m.sidebar.SetVisibleRows(layout.ContentHeight - 2)

	case ServiceUpdateMsg:
		if msg.Err == nil {
			now := time.Now()
			names := make([]string, len(msg.Services))
			for i, s := range msg.Services {
				names[i] = s.Name
				// Compute startedAt from Docker's reported uptime
				if s.Uptime > 0 {
					m.startedAt[s.Name] = now.Add(-s.Uptime)
				}
			}
			m.command.SetServiceNames(names)
			// Forward to sidebar
			newSidebar, cmd := m.sidebar.Update(msg)
			m.sidebar = newSidebar
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			m.applyTiers()
		}

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
			// Update detail panel with latest stats
			if svc := m.sidebar.SelectedInfo(); svc != nil {
				m.detail = m.updateDetail(svc)
			}
		}

	case uiTickMsg:
		m.sidebar = m.sidebar.UpdateUptimes(m.startedAt)
		m.services = m.services.UpdateUptimes(m.startedAt)
		if m.detail.service != nil {
			if t, ok := m.startedAt[m.detail.service.Name]; ok {
				m.detail.service.Uptime = time.Since(t)
			}
		}
		cmds = append(cmds, uiTick())

	case SidebarSelectionMsg:
		// Stop old log tail and start new one for selected service
		m.logTail.Stop()
		layout := ComputeLayout(m.width, m.height)
		logHeight := layout.ContentHeight / 2
		if logHeight < 5 {
			logHeight = 5
		}
		newLogTail, cmd := m.logTail.Start(msg.Service, layout.DetailWidth, logHeight)
		m.logTail = newLogTail
		cmds = append(cmds, cmd)
		// Update detail model
		if svc := m.sidebar.SelectedInfo(); svc != nil {
			m.detail = m.updateDetail(svc)
		}

	case graphDataMsg:
		if msg.err == nil && len(msg.tiers) > 0 {
			for i, tier := range msg.tiers {
				for _, name := range tier {
					m.tierMap[name] = i + 1
				}
			}
			m.applyTiers()
		}

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
		switch msg.Action {
		case ConfirmRestart:
			return m, restartService(msg.Service)
		case ConfirmDelete:
			return m, stopService(msg.Service)
		}

	case ActionResultMsg:
		if msg.Err != nil {
			m.toast = m.toast.Show("Error: " + msg.Err.Error())
		} else {
			m.toast = m.toast.Show(msg.Text)
		}
		return m, m.toast.Tick()

	case shellRequestMsg:
		return m, shellIntoService(msg.Service)

	case CommandResult:
		if msg.IsQuit {
			m.quitting = true
			return m, tea.Quit
		}
		if msg.View == ViewLogs && msg.Arg != "" {
			m.logs.Stop()
			m = m.pushView(ViewLogs)
			newLogs, cmd := m.logs.Start(msg.Arg, m.width, m.height-headerLines-footerLines)
			m.logs = newLogs
			return m, cmd
		}
		if msg.View == ViewDoctor {
			m = m.pushView(ViewDoctor)
			return m, m.doctor.Init()
		}
		if msg.View == ViewGraph {
			m = m.pushView(ViewGraph)
			return m, m.graph.Init()
		}
		if msg.View == ViewDescribe && msg.Arg != "" {
			m = m.pushView(ViewDescribe)
			newDesc, cmd := m.describe.Start(msg.Arg, m.services.Services(), m.width, m.height-headerLines-footerLines)
			m.describe = newDesc
			return m, cmd
		}
		// ViewServices or unknown — don't push onto stack
	}

	// Forward tick/polling messages to services model (data source)
	switch msg.(type) {
	case TickMsg, ServiceUpdateMsg, StatsUpdateMsg, statsTickMsg:
		newSvc, cmd := m.services.Update(msg)
		m.services = newSvc
		cmds = append(cmds, cmd)
	}

	// Forward messages to active view
	switch m.activeView {
	case ViewServices:
		switch msg.(type) {
		case TickMsg, ServiceUpdateMsg, StatsUpdateMsg, statsTickMsg:
			// Already forwarded above
		default:
			if !m.isWide() {
				// Narrow mode: services model handles its own keys
				newSvc, cmd := m.services.Update(msg)
				m.services = newSvc
				cmds = append(cmds, cmd)
			}
		}
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

	// Forward log tail messages (always, for live updates)
	switch msg.(type) {
	case LogLineMsg, LogErrMsg:
		if m.activeView == ViewServices {
			newLogTail, cmd := m.logTail.Update(msg)
			m.logTail = newLogTail
			cmds = append(cmds, cmd)
		}
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
	if m.width < minWidth || m.height < minHeight {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			styleBold.Render("Terminal too small")+"\n"+
				styleDim.Render(fmt.Sprintf("Need %dx%d, got %dx%d", minWidth, minHeight, m.width, m.height)))
	}

	layout := ComputeLayout(m.width, m.height)

	var content string
	switch m.activeView {
	case ViewServices:
		if m.isWide() {
			content = m.renderTwoPanel(layout)
		} else {
			content = m.services.View(m.width, layout.ContentHeight)
		}
	case ViewLogs:
		content = m.logs.View(m.width, layout.ContentHeight)
	case ViewDoctor:
		content = m.doctor.View(m.width, layout.ContentHeight)
	case ViewGraph:
		content = m.graph.View(m.width, layout.ContentHeight)
	case ViewDescribe:
		content = m.describe.View(m.width, layout.ContentHeight)
	}

	view := lipgloss.JoinVertical(lipgloss.Left,
		m.header.View(m.width, m.activeView),
		content,
		m.footer(),
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

func (m Model) renderTwoPanel(layout PanelLayout) string {
	leftContent := m.sidebar.View(layout.SidebarWidth-1, layout.ContentHeight)
	leftPanel := lipgloss.NewStyle().
		Width(layout.SidebarWidth).
		Height(layout.ContentHeight).
		BorderRight(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorBorder).
		Render(leftContent)

	logTailStr := m.logTail.View(layout.DetailWidth-2, layout.ContentHeight/2)
	rightContent := m.detail.View(layout.DetailWidth-2, layout.ContentHeight, logTailStr)
	rightPanel := lipgloss.NewStyle().
		Width(layout.DetailWidth).
		Height(layout.ContentHeight).
		PaddingLeft(1).
		Render(rightContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
}

func (m Model) footer() string {
	if m.command.Active() {
		return m.command.View(m.width)
	}
	if msg := m.toast.Message(); msg != "" {
		return styleStatusBar.Width(m.width).Render("  " + msg)
	}

	var hints []string
	switch m.activeView {
	case ViewServices:
		hints = []string{"j/k:nav", "enter/l:logs", "r:restart", "s:shell", "x:stop", "e:errors", "d:doctor", "g:graph", "/:filter", "?:help", "q:quit"}
	case ViewLogs:
		hints = []string{"↑↓:scroll", "g/G:top/bot", "t:timestamps", "w:wrap", "c:clear", "esc:back"}
	case ViewDoctor:
		hints = []string{"j/k:nav", "enter:expand", "R:rescan", "esc:back"}
	case ViewGraph:
		hints = []string{"1-9:focus tier", "0:all", "esc:back"}
	case ViewDescribe:
		hints = []string{"↑↓:scroll", "esc:back"}
	}

	var parts []string
	for _, h := range hints {
		idx := strings.Index(h, ":")
		if idx >= 0 {
			parts = append(parts, styleInfo.Render(h[:idx])+styleDim.Render(h[idx:]))
		} else {
			parts = append(parts, styleDim.Render(h))
		}
	}
	return styleStatusBar.Width(m.width).Render("  " + strings.Join(parts, "  "))
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
	dc, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
	if err != nil {
		dc = nil
	}
	if dc != nil {
		defer dc.Close()
	}
	p := tea.NewProgram(NewModel(dc), tea.WithAltScreen())
	_, err = p.Run()
	return err
}

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
