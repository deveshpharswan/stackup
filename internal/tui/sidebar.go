package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SidebarModel struct {
	services []ServiceInfo
	cursor   int
	uptimeAt time.Time // when the last poll arrived; used for live uptime
}

func NewSidebarModel() SidebarModel {
	return SidebarModel{}
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
				return m, func() tea.Msg { return SidebarSelectionMsg{Service: m.Selected()} }
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
				return m, func() tea.Msg { return SidebarSelectionMsg{Service: m.Selected()} }
			}
		}
	case ServiceUpdateMsg:
		if msg.Err == nil {
			prev := m.Selected()
			m.services = msg.Services
			m.uptimeAt = time.Now()
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
		}
	}
	return m, nil
}

func (m SidebarModel) View(width, height int) string {
	if len(m.services) == 0 {
		return styleDim.Render("  No services")
	}

	// Compute elapsed time since last poll for live uptime display
	elapsed := time.Duration(0)
	if !m.uptimeAt.IsZero() {
		elapsed = time.Since(m.uptimeAt)
	}

	// Build all logical rows (service rows + tier dividers) so we can scroll them uniformly
	type sidebarRow struct {
		text      string
		svcIndex  int // -1 for dividers
	}

	// Row layout: 1(indicator) + 1(dot) + 1(space) + name + 1(space) + uptime(6) = width
	// uptime column is 6 chars: "59m59s" at most
	const uptimeCols = 6
	nameWidth := width - 3 - uptimeCols
	if nameWidth < 4 {
		nameWidth = 4
	}

	var rows []sidebarRow
	currentTier := -1
	for i, svc := range m.services {
		if svc.Tier != currentTier && svc.Tier > 0 {
			currentTier = svc.Tier
			label := fmt.Sprintf(" tier %d ", svc.Tier)
			dashes := width - len(label) - 2
			if dashes < 1 {
				dashes = 1
			}
			divText := styleDim.Render("──" + label + strings.Repeat("─", dashes))
			rows = append(rows, sidebarRow{text: divText, svcIndex: -1})
		}

		liveUptime := svc.Uptime
		if svc.Uptime > 0 {
			liveUptime += elapsed
		}
		uptimeStr := ""
		if liveUptime > 0 {
			uptimeStr = formatUptimeShort(liveUptime)
		}

		dot, baseStyle := svcDotAndStyle(svc)

		indicator := " "
		var rowStyle lipgloss.Style
		if i == m.cursor {
			indicator = styleHealthy.Render("│")
			rowStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("#1c2128")).
				Foreground(colorGreen).
				Bold(true)
		} else {
			rowStyle = baseStyle
		}

		nameStr := truncate(svc.Name, nameWidth)
		raw := fmt.Sprintf("%s%s %-*s %-6s", indicator, dot, nameWidth, nameStr, uptimeStr)
		rows = append(rows, sidebarRow{text: rowStyle.Width(width).Render(raw), svcIndex: i})
	}

	// Header takes 1 line; remaining is scrollable content
	contentRows := height - 1
	if contentRows < 1 {
		contentRows = 1
	}

	// Find which row index corresponds to the cursor service
	cursorRowIdx := 0
	for ri, r := range rows {
		if r.svcIndex == m.cursor {
			cursorRowIdx = ri
			break
		}
	}

	// Scroll offset to keep cursor visible
	offset := 0
	if cursorRowIdx >= contentRows {
		offset = cursorRowIdx - contentRows + 1
	}

	var b strings.Builder

	// Header
	b.WriteString(lipgloss.NewStyle().
		Background(lipgloss.Color("#0f1117")).
		Foreground(colorBlue).
		Bold(true).
		Width(width).
		Render(fmt.Sprintf(" SERVICES  %d", len(m.services))) + "\n")

	// Render visible rows
	shown := 0
	for ri, r := range rows {
		if ri < offset {
			continue
		}
		if shown >= contentRows {
			break
		}
		b.WriteString(r.text + "\n")
		shown++
	}

	return b.String()
}

// svcDotAndStyle returns the dot symbol and base text style for a service.
func svcDotAndStyle(svc ServiceInfo) (string, lipgloss.Style) {
	switch {
	case svc.State == "running" && svc.Health == "healthy":
		return styleHealthy.Render("●"), styleHealthy
	case svc.State == "running":
		return styleWarning.Render("◐"), styleWarning
	case svc.State == "exited" || svc.State == "restarting":
		return styleFailed.Render("✗"), styleFailed
	default:
		return styleDim.Render("◌"), styleDim
	}
}

// svcStatusParts is kept for compatibility with describe.go and other callers.
func svcStatusParts(svc ServiceInfo) (string, lipgloss.Style, string) {
	dot, style := svcDotAndStyle(svc)
	uptime := ""
	if svc.Uptime > 0 {
		uptime = formatUptimeShort(svc.Uptime)
	}
	return dot, style, uptime
}

// formatUptimeShort formats a duration compactly for the sidebar (e.g. "2m34s", "1h02m").
func formatUptimeShort(d time.Duration) string {
	h := int(d.Hours())
	m := int(d.Minutes()) % 60
	s := int(d.Seconds()) % 60
	if h > 0 {
		return fmt.Sprintf("%dh%02dm", h, m)
	}
	return fmt.Sprintf("%dm%02ds", m, s)
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
