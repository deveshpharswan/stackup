package tui

import (
	"bufio"
	"context"
	"fmt"
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
	filtering  bool   // true when filter input is open
	filter     string // active filter text
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
		if m.filtering {
			switch msg.String() {
			case "esc", "enter":
				m.filtering = false
			case "backspace", "ctrl+h":
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
		switch msg.String() {
		case "esc":
			if m.filter != "" {
				m.filter = ""
				m.viewport.SetContent(m.renderLines())
				return m, nil
			}
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

func (m LogsModel) ServiceName() string { return m.service }

// ActivateFilter enables the in-log filter mode.
func (m LogsModel) ActivateFilter() (LogsModel, tea.Cmd) {
	m.filtering = true
	return m, nil
}

func (m LogsModel) Stop() LogsModel {
	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}
	m.logCh = nil
	return m
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
		if m.filter != "" && !strings.Contains(strings.ToLower(line), strings.ToLower(m.filter)) {
			continue
		}
		display := line
		if !m.timestamps {
			display = stripTimestamp(display)
		}
		rendered := m.colorLogLine(display)
		if m.filter != "" {
			lower := strings.ToLower(rendered)
			lowerFilter := strings.ToLower(m.filter)
			idx := strings.Index(lower, lowerFilter)
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
