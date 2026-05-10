package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// ProgressModel renders startup progress in the right panel.
// It disappears once all services are healthy.
type ProgressModel struct {
	services  []ServiceInfo
	firstSeen map[string]time.Time
}

func NewProgressModel() ProgressModel {
	return ProgressModel{firstSeen: make(map[string]time.Time)}
}

// AllDone returns true when every service is healthy or exited (no pending startup).
func (m ProgressModel) AllDone() bool {
	for _, s := range m.services {
		if s.State == "running" && s.Health != "healthy" {
			return false
		}
	}
	return true // empty list or all services done
}

func (m ProgressModel) Update(services []ServiceInfo) ProgressModel {
	m.services = services
	now := time.Now()
	for _, s := range services {
		if _, seen := m.firstSeen[s.Name]; !seen {
			m.firstSeen[s.Name] = now
		}
	}
	return m
}

func (m ProgressModel) View(width int) string {
	if m.AllDone() {
		return ""
	}
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().
		Foreground(colorDim).
		Width(width).
		Padding(0, 1).
		Render("Startup") + "\n")

	barWidth := width - 12
	if barWidth < 4 {
		barWidth = 4
	}

	for _, s := range m.services {
		var fillStyle lipgloss.Style
		var filled int
		var statusChar string

		switch {
		case s.State == "running" && s.Health == "healthy":
			filled = barWidth
			fillStyle = lipgloss.NewStyle().Foreground(colorGreen)
			statusChar = styleHealthy.Render("✓")
		case s.State == "exited":
			filled = barWidth
			fillStyle = lipgloss.NewStyle().Foreground(colorRed)
			statusChar = styleFailed.Render("✗")
		default:
			// Estimate progress from elapsed time (max out at 90%)
			elapsed := time.Since(m.firstSeen[s.Name])
			pct := int(elapsed.Seconds() / 30 * float64(barWidth))
			if pct > barWidth*9/10 {
				pct = barWidth * 9 / 10
			}
			filled = pct
			fillStyle = lipgloss.NewStyle().Foreground(colorYellow)
			statusChar = styleWarning.Render("…")
		}

		bar := fillStyle.Render(strings.Repeat("█", filled)) +
			styleDim.Render(strings.Repeat("░", barWidth-filled))

		name := truncate(s.Name, 7)
		b.WriteString(fmt.Sprintf("  %-7s %s %s\n", name, bar, statusChar))
	}
	return b.String()
}
