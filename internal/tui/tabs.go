package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type TabType int

const (
	TabServices TabType = iota
	TabLogs
	TabStats
	TabDoctor
	TabGraph
)

var tabLabels = []string{"Services", "Logs", "Stats", "Doctor", "Graph"}

func renderTabBar(width int, active TabType) string {
	var parts []string
	for i, label := range tabLabels {
		num := styleDim.Render(fmt.Sprintf("%d", i+1))
		if TabType(i) == active {
			tab := lipgloss.NewStyle().
				Background(lipgloss.Color("#161b22")).
				Foreground(colorBlue).
				Bold(true).
				BorderBottom(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(colorBlue).
				Padding(0, 1).
				Render(num + " " + label)
			parts = append(parts, tab)
		} else {
			tab := lipgloss.NewStyle().
				Foreground(colorDim).
				Padding(0, 1).
				Render(num + " " + label)
			parts = append(parts, tab)
		}
	}
	bar := strings.Join(parts, "")
	return lipgloss.NewStyle().
		Background(lipgloss.Color("#0d1117")).
		BorderBottom(true).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(colorBorder).
		Width(width).
		Render(bar)
}
