package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deveshpharswan/stackup/internal/constants"
)

type HeaderModel struct {
	stack     string
	compose   string
	tiers     int
	healthy   int
	total     int
	failed    int
	startTime time.Time
}

func NewHeaderModel() HeaderModel {
	composeName := "docker-compose.yml"
	if found := constants.FindComposeFile("."); found != "" {
		composeName = filepath.Base(found)
	}
	return HeaderModel{
		stack:     currentDirName(),
		compose:   composeName,
		startTime: time.Now(),
	}
}

func (m HeaderModel) Init() tea.Cmd { return nil }

func (m HeaderModel) Update(msg tea.Msg) (HeaderModel, tea.Cmd) {
	switch msg := msg.(type) {
	case ServiceUpdateMsg:
		if msg.Err == nil {
			m.total = len(msg.Services)
			healthy, failed := 0, 0
			for _, s := range msg.Services {
				switch {
				case s.Health == "healthy":
					healthy++
				case s.State == "exited" || s.State == "restarting":
					failed++
				}
			}
			m.healthy = healthy
			m.failed = failed
		}
	case graphDataMsg:
		if msg.err == nil {
			m.tiers = len(msg.tiers)
		}
	}
	return m, nil
}

func tierStr(n int) string {
	if n == 0 {
		return "—"
	}
	return fmt.Sprintf("%d", n)
}

func (m HeaderModel) View(width int, active ViewType) string {
	uptime := time.Since(m.startTime).Truncate(time.Second)

	logo := styleInfo.Bold(true).Render("STACKUP")
	sep := styleDim.Render(" │ ")
	project := styleBold.Render(m.stack)

	var badges []string
	starting := m.total - m.healthy - m.failed
	// Show worst state first for quick visual scanning
	if m.failed > 0 {
		badges = append(badges, styleFailed.Render(fmt.Sprintf("✗ %d failed", m.failed)))
	}
	if starting > 0 {
		badges = append(badges, styleWarning.Render(fmt.Sprintf("◐ %d starting", starting)))
	}
	if m.healthy > 0 {
		badges = append(badges, styleHealthy.Render(fmt.Sprintf("● %d healthy", m.healthy)))
	}

	meta := styleDim.Render(fmt.Sprintf("%s  tiers:%s  uptime:%s",
		m.compose, tierStr(m.tiers), formatUptime(uptime)))

	left := logo + sep + project
	if len(badges) > 0 {
		left += sep + strings.Join(badges, "  ")
	}
	left += sep + meta

	// Truncate if needed to fit within terminal width (use visual width, not byte length)
	if lipgloss.Width(left) > width-2 {
		// Rebuild with fewer components for narrow terminals
		left = logo + sep + project
		if lipgloss.Width(left) > width-2 {
			left = logo + sep + styleBold.Render(truncate(m.stack, width-12))
		}
	}

	return styleHeader.Width(width).Render(left)
}

func formatUptime(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh %02dm", h, m)
	}
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
