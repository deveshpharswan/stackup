package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deveshpharswan/stackup/internal/constants"
	"github.com/deveshpharswan/stackup/internal/orchestrator"
	"github.com/deveshpharswan/stackup/internal/scaffold"
)

type GraphModel struct {
	tiers     []orchestrator.Tier
	deps      map[string][]string
	services  []ServiceInfo
	focusTier int
}

func NewGraphModel() GraphModel {
	return GraphModel{focusTier: -1}
}

func (m GraphModel) Init() tea.Cmd {
	return func() tea.Msg {
		composeSvcs, err := scaffold.ParseServices(constants.DefaultComposeFile)
		if err != nil {
			return graphDataMsg{err: err}
		}
		tiers, err := orchestrator.BuildTiers(composeSvcs)
		if err != nil {
			return graphDataMsg{err: err}
		}
		return graphDataMsg{tiers: tiers, deps: composeSvcs}
	}
}

type graphDataMsg struct {
	tiers []orchestrator.Tier
	deps  map[string][]string
	err   error
}

func (m GraphModel) Update(msg tea.Msg) (GraphModel, tea.Cmd) {
	switch msg := msg.(type) {
	case graphDataMsg:
		if msg.err == nil {
			m.tiers = msg.tiers
			m.deps = msg.deps
		}
	case ServiceUpdateMsg:
		m.services = msg.Services
	case tea.KeyMsg:
		switch msg.String() {
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			tier := int(msg.Runes[0] - '0')
			if tier <= len(m.tiers) {
				m.focusTier = tier
			}
		case "0":
			m.focusTier = -1
		}
	}
	return m, nil
}

func (m GraphModel) View(width, height int) string {
	if len(m.tiers) == 0 {
		return styleDim.Render("  No dependency data available")
	}

	var b strings.Builder

	b.WriteString("\n")
	for i, tier := range m.tiers {
		tierNum := i + 1
		_ = tier
		label := fmt.Sprintf("Tier %d", tierNum)
		b.WriteString(styleDim.Render(fmt.Sprintf("        %-20s", label)))
	}
	b.WriteString("\n")
	for range m.tiers {
		b.WriteString(styleDim.Render("       ──────────────      "))
	}
	b.WriteString("\n\n")

	maxServices := 0
	for _, tier := range m.tiers {
		if len(tier) > maxServices {
			maxServices = len(tier)
		}
	}

	for row := 0; row < maxServices; row++ {
		line1 := ""
		line2 := ""
		line3 := ""
		for i, tier := range m.tiers {
			focused := m.focusTier == i+1 || m.focusTier == -1
			if row < len(tier) {
				svc := tier[row]
				health := m.healthFor(svc)
				icon := m.iconFor(health)
				boxStyle := m.styleFor(health, focused)

				top := "  ┌──────────────┐  "
				mid := fmt.Sprintf("  │ %s %-10s │  ", icon, svc)
				bot := "  └──────────────┘  "

				if !focused {
					top = styleDim.Render(top)
					mid = styleDim.Render(mid)
					bot = styleDim.Render(bot)
				} else {
					top = boxStyle.Render(top)
					mid = boxStyle.Render(mid)
					bot = boxStyle.Render(bot)
				}
				line1 += top
				line2 += mid
				line3 += bot
			} else {
				pad := strings.Repeat(" ", 20)
				line1 += pad
				line2 += pad
				line3 += pad
			}
		}
		b.WriteString(line1 + "\n")
		b.WriteString(line2 + "\n")
		b.WriteString(line3 + "\n")
	}

	b.WriteString("\n")
	b.WriteString(styleDim.Render("  Legend: "))
	b.WriteString(styleHealthy.Render("◉ healthy") + "  ")
	b.WriteString(styleWarning.Render("◎ starting") + "  ")
	b.WriteString(styleFailed.Render("◈ failed") + "  ")
	b.WriteString(styleDim.Render("──▸ depends on"))
	b.WriteString("\n\n")

	var order []string
	for _, tier := range m.tiers {
		order = append(order, strings.Join(tier, ", "))
	}
	b.WriteString(styleDim.Render("  Startup order: " + strings.Join(order, " → ")))

	return b.String()
}

func (m GraphModel) healthFor(name string) string {
	for _, s := range m.services {
		if s.Name == name {
			return s.Health
		}
	}
	return "(none)"
}

func (m GraphModel) iconFor(health string) string {
	switch health {
	case "healthy":
		return styleHealthy.Render("◉")
	case "starting":
		return styleWarning.Render("◎")
	case "unhealthy":
		return styleFailed.Render("◈")
	default:
		return styleDim.Render("○")
	}
}

func (m GraphModel) styleFor(health string, focused bool) lipgloss.Style {
	if !focused {
		return styleDim
	}
	switch health {
	case "healthy":
		return styleHealthy
	case "starting":
		return styleWarning
	case "unhealthy":
		return styleFailed
	default:
		return styleDim
	}
}
