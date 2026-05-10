package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type HelpModel struct{}

func NewHelpModel() HelpModel { return HelpModel{} }

func (m HelpModel) View(width, height int, active ViewType) string {
	title := styleBold.Render("Keyboard Shortcuts")
	subtitle := styleDim.Render("Press ? or Esc to close")

	var bindings [][]string

	switch active {
	case ViewServices:
		bindings = [][]string{
			{"↑/k", "Move up"},
			{"↓/j", "Move down"},
			{"enter", "View logs"},
			{"l", "View logs"},
			{"r", "Restart service"},
			{"s", "Shell into container"},
			{"x", "Stop service"},
			{"e", "Error zoom (unhealthy only)"},
			{"d", "Doctor diagnostics"},
			{"g", "Dependency graph"},
			{"/", "Filter services"},
			{":", "Command mode"},
			{"?", "This help"},
			{"q", "Quit"},
		}
	case ViewLogs:
		bindings = [][]string{
			{"esc", "Back to services"},
			{"t", "Toggle timestamps"},
			{"w", "Toggle wrap"},
			{"/", "Search in logs"},
			{"c", "Clear viewport"},
			{"g", "Jump to top"},
			{"G", "Jump to bottom"},
			{"PgUp", "Page up"},
			{"PgDn", "Page down"},
		}
	case ViewDoctor:
		bindings = [][]string{
			{"esc", "Back to services"},
			{"↑/k", "Move up"},
			{"↓/j", "Move down"},
			{"enter", "Expand/collapse detail"},
			{"R", "Re-run diagnostics"},
		}
	case ViewGraph:
		bindings = [][]string{
			{"esc", "Back to services"},
			{"1-9", "Focus tier N"},
			{"0", "Show all tiers"},
			{"enter", "Select service"},
		}
	case ViewDescribe:
		bindings = [][]string{
			{"esc", "Back"},
			{"l", "View logs"},
			{"r", "Restart service"},
		}
	}

	var left, right []string
	mid := (len(bindings) + 1) / 2
	for i, b := range bindings {
		entry := fmt.Sprintf("  %s  %s",
			styleInfo.Render(fmt.Sprintf("%-8s", b[0])),
			b[1])
		if i < mid {
			left = append(left, entry)
		} else {
			right = append(right, entry)
		}
	}

	leftCol := strings.Join(left, "\n")
	rightCol := strings.Join(right, "\n")

	columns := lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(width/2).Render(leftCol),
		lipgloss.NewStyle().Width(width/2).Render(rightCol),
	)

	commands := styleDim.Render("\n  Commands: :services :logs <name> :doctor :graph :describe <name> :quit")

	content := title + "\n" + subtitle + "\n\n" + columns + "\n" + commands

	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(colorBorder).
		Padding(1, 2).
		Width(width - 4).
		Height(height - 4).
		Render(content)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, box)
}
