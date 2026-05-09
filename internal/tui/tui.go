package tui

import (
	"fmt"

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
	return m.services.Init()
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
	p := tea.NewProgram(NewModel(), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
