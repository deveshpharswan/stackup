package tui

import (
	"bufio"
	"context"
	"fmt"
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
		_ = cancel

		c := exec.CommandContext(ctx, "docker", "compose", "logs", "-f", "--tail", "100", service)
		stdout, err := c.StdoutPipe()
		if err != nil {
			return LogErrMsg{Err: err}
		}
		if err := c.Start(); err != nil {
			return LogErrMsg{Err: err}
		}

		scanner := bufio.NewScanner(stdout)
		if scanner.Scan() {
			return LogLineMsg{Line: scanner.Text()}
		}
		if err := scanner.Err(); err != nil {
			return LogErrMsg{Err: err}
		}
		return LogErrMsg{Err: fmt.Errorf("log stream ended")}
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
