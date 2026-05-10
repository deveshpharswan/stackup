package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SidebarModel struct {
	services    []ServiceInfo
	cursor      int
	offset      int
	visibleRows int
}

func NewSidebarModel() SidebarModel {
	return SidebarModel{}
}

func (m SidebarModel) SetVisibleRows(n int) SidebarModel {
	m.visibleRows = n
	return m
}

func (m SidebarModel) UpdateUptimes(startedAt map[string]time.Time) SidebarModel {
	now := time.Now()
	for i, svc := range m.services {
		if t, ok := startedAt[svc.Name]; ok {
			m.services[i].Uptime = now.Sub(t)
		}
	}
	return m
}

func (m SidebarModel) SetServices(services []ServiceInfo) SidebarModel {
	m.services = services
	if m.cursor >= len(services) && len(services) > 0 {
		m.cursor = len(services) - 1
	}
	return m
}

func (m SidebarModel) Selected() string {
	if m.cursor < len(m.services) {
		return m.services[m.cursor].Name
	}
	return ""
}

func (m SidebarModel) SelectedInfo() *ServiceInfo {
	if m.cursor < len(m.services) {
		s := m.services[m.cursor]
		return &s
	}
	return nil
}

func (m SidebarModel) Update(msg tea.Msg) (SidebarModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.services)-1 {
				m.cursor++
				m.adjustOffset()
				return m, func() tea.Msg { return SidebarSelectionMsg{Service: m.Selected()} }
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				m.adjustOffset()
				return m, func() tea.Msg { return SidebarSelectionMsg{Service: m.Selected()} }
			}
		}
	case ServiceUpdateMsg:
		if msg.Err == nil {
			prev := m.Selected()
			m.services = msg.Services
			// Restore cursor to same service name if possible
			for i, s := range m.services {
				if s.Name == prev {
					m.cursor = i
					return m, nil
				}
			}
			if m.cursor >= len(m.services) && len(m.services) > 0 {
				m.cursor = len(m.services) - 1
			}
			// Emit initial selection so detail panel populates on startup
			if prev == "" && len(m.services) > 0 {
				return m, func() tea.Msg { return SidebarSelectionMsg{Service: m.Selected()} }
			}
		}
	}
	return m, nil
}

func (m *SidebarModel) adjustOffset() {
	visible := m.visibleRows
	if visible < 1 {
		visible = 10
	}
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+visible {
		m.offset = m.cursor - visible + 1
	}
}

func (m SidebarModel) View(width, height int) string {
	if len(m.services) == 0 {
		return styleDim.Render("  No services")
	}

	// Scroll window
	visible := height - 2 // subtract header line
	if visible < 1 {
		visible = 1
	}

	var b strings.Builder
	// Panel header
	b.WriteString(lipgloss.NewStyle().
		Background(lipgloss.Color("#0f1117")).
		Foreground(colorBlue).
		Bold(true).
		Width(width).
		Padding(0, 1).
		Render(fmt.Sprintf("Services  %d/%d", len(m.services), len(m.services))) + "\n")

	currentTier := -1
	rendered := 0
	for i, svc := range m.services {
		if i < m.offset {
			continue
		}
		if rendered >= visible {
			break
		}

		// Tier divider
		if svc.Tier != currentTier && svc.Tier > 0 {
			currentTier = svc.Tier
			if rendered >= visible {
				break
			}
			divider := styleDim.Width(width).Render(fmt.Sprintf(" tier %d ", svc.Tier))
			b.WriteString(divider + "\n")
			rendered++
			if rendered >= visible {
				break
			}
		}

		dot, nameStyle, uptimeStr := svcStatusParts(svc)
		name := nameStyle.Render(truncate(svc.Name, width-10))
		uptime := styleDim.Render(truncate(uptimeStr, 7))

		row := fmt.Sprintf(" %s %-*s %s", dot, width-12, name, uptime)

		if i == m.cursor {
			b.WriteString(lipgloss.NewStyle().
				Background(lipgloss.Color("#161b22")).
				BorderLeft(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(colorGreen).
				Width(width).
				Render(row) + "\n")
		} else {
			b.WriteString(lipgloss.NewStyle().Width(width).Render(row) + "\n")
		}
		rendered++
	}
	return b.String()
}

// svcStatusParts returns dot symbol, name style, and uptime string for a service.
func svcStatusParts(svc ServiceInfo) (string, lipgloss.Style, string) {
	uptime := ""
	if svc.Uptime > 0 {
		uptime = formatUptime(svc.Uptime)
	}
	switch {
	case svc.State == "running" && svc.Health == "healthy":
		return styleHealthy.Render("●"), styleHealthy, uptime
	case svc.State == "running":
		return styleWarning.Render("◐"), styleWarning, uptime
	case svc.State == "exited" || svc.State == "restarting":
		return styleFailed.Render("✗"), styleFailed, uptime
	default:
		return styleDim.Render("◌"), styleDim, uptime
	}
}

func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}
