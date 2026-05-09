package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type HeaderModel struct {
	stack     string
	compose   string
	tiers     int
	healthy   int
	total     int
	startTime time.Time
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
