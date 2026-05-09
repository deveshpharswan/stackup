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
	View   ViewType
	Arg    string
	IsQuit bool
	Filter string
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
