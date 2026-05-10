package tui

import (
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func init() {
	if os.Getenv("NO_COLOR") != "" {
		lipgloss.DefaultRenderer().SetColorProfile(termenv.Ascii)
	}
}

var (
	colorGreen  = lipgloss.Color("#7ee787")
	colorYellow = lipgloss.Color("#d29922")
	colorRed    = lipgloss.Color("#f85149")
	colorBlue   = lipgloss.Color("#58a6ff")
	colorDim    = lipgloss.Color("#484f58")
	colorWhite  = lipgloss.Color("#c9d1d9")
	colorBorder = lipgloss.Color("#30363d")

	styleHeader = lipgloss.NewStyle().
			Background(lipgloss.Color("#161b22")).
			Foreground(colorWhite).
			Padding(0, 1)

	styleStatusBar = lipgloss.NewStyle().
			Background(lipgloss.Color("#161b22")).
			Foreground(colorDim).
			Padding(0, 1)

	styleSelected = lipgloss.NewStyle().
			Background(lipgloss.Color("#1f2937")).
			Foreground(colorGreen)

	styleHealthy = lipgloss.NewStyle().Foreground(colorGreen)
	styleWarning = lipgloss.NewStyle().Foreground(colorYellow)
	styleFailed  = lipgloss.NewStyle().Foreground(colorRed)
	styleInfo    = lipgloss.NewStyle().Foreground(colorBlue)
	styleDim     = lipgloss.NewStyle().Foreground(colorDim)
	styleBold    = lipgloss.NewStyle().Bold(true).Foreground(colorWhite)

	styleModal = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorBorder).
			Padding(1, 2).
			Width(36)
)
