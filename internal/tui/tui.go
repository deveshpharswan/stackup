package tui

import (
	"fmt"
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

	quitting bool
}

func NewModel(dc *dockerclient.Client) Model {
	return Model{
		activeView: ViewServices,
		viewStack:  []ViewType{ViewServices},
		services:   NewServicesModel(dc),
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
	return m.services.Init()
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
				if svc := m.services.Selected(); svc != "" {
					m.logs.Stop()
					m = m.pushView(ViewLogs)
					newLogs, cmd := m.logs.Start(svc, m.width, m.height-7)
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
				if svc := m.services.Selected(); svc != "" {
					m = m.pushView(ViewDescribe)
					newDesc, cmd := m.describe.Start(svc, m.services.Services(), m.width, m.height-7)
					m.describe = newDesc
					return m, cmd
				}
			}
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
			newLogs, cmd := m.logs.Start(msg.Arg, m.width, m.height-7)
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
			newDesc, cmd := m.describe.Start(msg.Arg, m.services.Services(), m.width, m.height-7)
			m.describe = newDesc
			return m, cmd
		}
		m = m.pushView(msg.View)
	}

	switch msg.(type) {
	case TickMsg, ServiceUpdateMsg, StatsUpdateMsg, statsTickMsg:
		newSvc, cmd := m.services.Update(msg)
		m.services = newSvc
		cmds = append(cmds, cmd)
	}

	switch m.activeView {
	case ViewServices:
		switch msg.(type) {
		case TickMsg, ServiceUpdateMsg, StatsUpdateMsg, statsTickMsg:
		default:
			newSvc, cmd := m.services.Update(msg)
			m.services = newSvc
			cmds = append(cmds, cmd)
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

	contentHeight := m.height - 7

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
