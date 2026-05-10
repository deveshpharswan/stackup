package tui

import (
	"bufio"
	"context"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

type LogsModel struct {
	service    string
	viewport   viewport.Model
	lines      []string
	cancel     context.CancelFunc
	logCh      <-chan string
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

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel
	ch := startLogStream(ctx, service)
	m.logCh = ch
	return m, waitForLogLine(ch)
}

func (m LogsModel) Update(msg tea.Msg) (LogsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case LogLineMsg:
		line := msg.Line
		if strings.HasPrefix(line, "\x00ERR:") {
			errText := strings.TrimPrefix(line, "\x00ERR:")
			m.lines = append(m.lines, styleFailed.Render("Stream error: "+errText))
			m.viewport.SetContent(m.renderLines())
			return m, nil
		}
		m.lines = append(m.lines, line)
		if len(m.lines) > 1000 {
			m.lines = m.lines[len(m.lines)-1000:]
		}
		m.viewport.SetContent(m.renderLines())
		m.viewport.GotoBottom()
		return m, waitForLogLine(m.logCh)
	case LogErrMsg:
		if msg.Err != nil {
			m.lines = append(m.lines, styleFailed.Render("Stream error: "+msg.Err.Error()))
			m.viewport.SetContent(m.renderLines())
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "t":
			m.timestamps = !m.timestamps
			m.viewport.SetContent(m.renderLines())
		case "w":
			m.wrap = !m.wrap
			m.viewport.SetContent(m.renderLines())
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
	m.logCh = nil
}

func startLogStream(ctx context.Context, service string) <-chan string {
	ch := make(chan string, 64)
	go func() {
		defer close(ch)
		c := exec.CommandContext(ctx, "docker", "compose", "logs", "-f", "--tail", "100", "--timestamps", service)
		stdout, err := c.StdoutPipe()
		if err != nil {
			select {
			case <-ctx.Done():
			case ch <- "\x00ERR:" + err.Error():
			}
			return
		}
		if err := c.Start(); err != nil {
			select {
			case <-ctx.Done():
			case ch <- "\x00ERR:" + err.Error():
			}
			return
		}
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case ch <- scanner.Text():
			}
		}
		_ = c.Wait()
	}()
	return ch
}

func waitForLogLine(ch <-chan string) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		line, ok := <-ch
		if !ok {
			return LogErrMsg{Err: nil}
		}
		return LogLineMsg{Line: line}
	}
}

func (m LogsModel) renderLines() string {
	var b strings.Builder
	for _, line := range m.lines {
		display := line
		if !m.timestamps {
			display = stripTimestamp(display)
		}
		rendered := m.colorLogLine(display)
		if m.wrap && m.viewport.Width > 0 {
			rendered = lipgloss.NewStyle().Width(m.viewport.Width).Render(rendered)
		}
		b.WriteString(rendered + "\n")
	}
	return b.String()
}

func stripTimestamp(line string) string {
	if len(line) > 31 && line[4] == '-' && line[10] == 'T' {
		if idx := strings.IndexByte(line[20:], ' '); idx >= 0 {
			return line[20+idx+1:]
		}
	}
	return line
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
