package tui

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	dockerclient "github.com/docker/docker/client"
)

// ViewType is retained for backward-compatibility with help.go and command.go.
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

	activeTab TabType

	services     ServicesModel            // data source: polling + filter state
	sidebar      SidebarModel
	detail       DetailModel
	logTail      LogsModel                // right panel live log tail
	logs         LogsModel                // tab 2 full-screen logs
	doctor       DoctorViewModel
	graph        GraphModel
	progress     ProgressModel
	statsHistory map[string]*StatsHistory // updated from StatsUpdateMsg

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
		activeTab:    TabServices,
		services:     NewServicesModel(dc),
		sidebar:      NewSidebarModel(),
		detail:       NewDetailModel(),
		doctor:       NewDoctorViewModel(),
		graph:        NewGraphModel(),
		progress:     NewProgressModel(),
		statsHistory: make(map[string]*StatsHistory),
		header:       NewHeaderModel(),
		command:      NewCommandModel(),
		toast:        NewToastModel(),
		help:         NewHelpModel(),
		confirm:      NewConfirmModel(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.services.Init(),
		m.graph.Init(), // loads tier data via graphDataMsg
	)
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

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case ServiceUpdateMsg:
		if msg.Err == nil {
			names := make([]string, len(msg.Services))
			for i, s := range msg.Services {
				names[i] = s.Name
			}
			m.command.SetServiceNames(names)

			// Update sidebar
			newSidebar, cmd := m.sidebar.Update(msg)
			m.sidebar = newSidebar
			cmds = append(cmds, cmd)

			// Update detail with selected service data
			if svc := m.sidebar.SelectedInfo(); svc != nil {
				m.detail = m.detail.SetService(*svc, msg.Services, m.statsHistory)
			}

			m.progress = m.progress.Update(msg.Services)
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
			// Refresh detail panel sparklines
			if svc := m.sidebar.SelectedInfo(); svc != nil {
				m.detail = m.detail.SetService(*svc, m.sidebar.services, m.statsHistory)
			}
		}

	case SidebarSelectionMsg:
		// Update log tail for new selection
		m.logTail.Stop()
		newLogTail, cmd := m.logTail.Start(msg.Service, rightWidth, m.height-headerLines-footerLines)
		m.logTail = newLogTail
		cmds = append(cmds, cmd)
		// Trigger inspect fetch
		cmds = append(cmds, fetchInspect(msg.Service))
		// Update detail for new service
		if svc := m.sidebar.SelectedInfo(); svc != nil {
			m.detail = m.detail.SetService(*svc, m.sidebar.services, m.statsHistory)
		}

	case InspectResultMsg:
		newDetail, cmd := m.detail.Update(msg)
		m.detail = newDetail
		cmds = append(cmds, cmd)

	case graphDataMsg:
		if msg.err == nil {
			// Build name->tier map and stamp tiers onto sidebar services
			tierMap := make(map[string]int)
			for i, tier := range msg.tiers {
				for _, name := range tier {
					tierMap[name] = i + 1
				}
			}
			services := make([]ServiceInfo, len(m.sidebar.services))
			copy(services, m.sidebar.services)
			for i, s := range services {
				services[i].Tier = tierMap[s.Name]
			}
			m.sidebar = m.sidebar.SetServices(services)
		}
		newGraph, cmd := m.graph.Update(msg)
		m.graph = newGraph
		cmds = append(cmds, cmd)

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
		case ConfirmStackDown:
			return m, downStack()
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
	}

	// Route polling messages to services model always
	switch msg.(type) {
	case TickMsg, ServiceUpdateMsg, StatsUpdateMsg, statsTickMsg:
		newSvc, cmd := m.services.Update(msg)
		m.services = newSvc
		cmds = append(cmds, cmd)
	}

	// Route key messages and others to active tab
	switch m.activeTab {
	case TabServices:
		switch msg.(type) {
		case TickMsg, ServiceUpdateMsg, StatsUpdateMsg, statsTickMsg:
			// already handled above
		case tea.KeyMsg:
			// Handle sidebar navigation
			newSidebar, cmd := m.sidebar.Update(msg)
			m.sidebar = newSidebar
			cmds = append(cmds, cmd)
			// Handle detail sub-tab navigation
			newDetail, cmd2 := m.detail.Update(msg)
			m.detail = newDetail
			cmds = append(cmds, cmd2)
		}
	case TabLogs:
		newLogs, cmd := m.logs.Update(msg)
		m.logs = newLogs
		cmds = append(cmds, cmd)
	case TabDoctor:
		newDoc, cmd := m.doctor.Update(msg)
		m.doctor = newDoc
		cmds = append(cmds, cmd)
	case TabGraph:
		newGraph, cmd := m.graph.Update(msg)
		m.graph = newGraph
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
	if m.width < minWidth || m.height < minHeight {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			styleBold.Render("Terminal too small")+"\n"+
				styleDim.Render(fmt.Sprintf("Need %dx%d, got %dx%d", minWidth, minHeight, m.width, m.height)))
	}

	layout := ComputeLayout(m.width, m.height)

	hdr := m.header.View(m.width, m.activeTab)
	tabBar := renderTabBar(m.width, m.activeTab)
	footer := m.renderFooter()

	var view string
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
		hints = []string{"↑↓:nav", "enter:focus", "1-5:tabs", "r:restart", "s:stop", "u:start", "x:shell", "D:down", "/:filter", "?:help", "q:quit"}
	case TabLogs:
		hints = []string{"↑↓:scroll", "g/G:top/bot", "/:filter", "1-5:tabs", "esc:back"}
	default:
		hints = []string{"1-5:tabs", "esc:back", "?:help", "q:quit"}
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

func fetchInspect(service string) tea.Cmd {
	return func() tea.Msg {
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

		for cPort, bindings := range info.NetworkSettings.Ports {
			for _, b := range bindings {
				pm := PortMapping{
					HostPort:      b.HostPort,
					ContainerPort: cPort.Port(),
					Proto:         cPort.Proto(),
				}
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
