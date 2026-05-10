package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type DetailModel struct {
	service      *ServiceInfo
	statsHistory map[string]*StatsHistory
	deps         []string
	healthDesc   string
}

func NewDetailModel() DetailModel {
	return DetailModel{}
}

func (m DetailModel) SetService(svc *ServiceInfo, history map[string]*StatsHistory) DetailModel {
	m.service = svc
	m.statsHistory = history
	return m
}

func (m DetailModel) SetServiceMeta(deps []string, healthDesc string) DetailModel {
	m.deps = deps
	m.healthDesc = healthDesc
	return m
}

func (m DetailModel) View(width, height int, logTailView string) string {
	if m.service == nil {
		return styleDim.Render("  Select a service")
	}

	var b strings.Builder
	svc := m.service

	// Header: service name + state
	dot, nameStyle, _ := svcStatusParts(*svc)
	healthLabel := svc.Health
	if healthLabel == "" || healthLabel == "(none)" {
		healthLabel = svc.State
	}
	hdr := fmt.Sprintf(" %s %s — %s",
		dot,
		nameStyle.Bold(true).Render(svc.Name),
		nameStyle.Render(healthLabel))
	b.WriteString(lipgloss.NewStyle().
		Background(lipgloss.Color("#0f1117")).
		Width(width).
		Render(hdr) + "\n")
	b.WriteString(styleDim.Render(strings.Repeat("─", width)) + "\n")

	// Detail rows
	row := func(key, val string) {
		b.WriteString(fmt.Sprintf("  %-10s %s\n", styleDim.Render(key), val))
	}

	// Health check
	if m.healthDesc != "" {
		row("Health", styleInfo.Render(m.healthDesc))
	}

	// CPU sparkline
	cpuSpark, cpuVal := "—", "—"
	if m.statsHistory != nil {
		if h, ok := m.statsHistory[svc.Name]; ok && len(h.cpu) > 0 {
			cpuSpark = styleInfo.Render(renderSparkline(h.cpu, 100))
			cpuVal = styleHealthy.Render(fmt.Sprintf("%.1f%%", svc.CPU))
		}
	}
	row("CPU", cpuSpark+"  "+cpuVal)

	// Memory sparkline
	memSpark, memVal := "—", "—"
	if m.statsHistory != nil {
		if h, ok := m.statsHistory[svc.Name]; ok && len(h.mem) > 0 {
			memSpark = styleInfo.Render(renderSparkline(h.mem, 100))
			memVal = styleInfo.Render(fmt.Sprintf("%.1f%%", svc.Memory))
		}
	}
	row("Memory", memSpark+"  "+memVal)

	// Ports
	if svc.Ports != "" {
		row("Ports", styleInfo.Render(svc.Ports))
	}

	// Uptime
	row("Uptime", styleBold.Render(formatUptime(svc.Uptime)))

	// Dependencies
	if len(m.deps) > 0 {
		row("Depends", styleDim.Render(strings.Join(m.deps, ", ")))
	}

	// Log tail section
	overviewLines := strings.Count(b.String(), "\n")
	logHeight := height - overviewLines - 2
	if logHeight < 3 {
		logHeight = 3
	}

	b.WriteString("\n")
	b.WriteString(styleDim.Render(" ─── Logs ") + styleDim.Render(strings.Repeat("─", width-11)) + "\n")

	if logTailView != "" {
		lines := strings.Split(logTailView, "\n")
		if len(lines) > logHeight {
			lines = lines[len(lines)-logHeight:]
		}
		for _, line := range lines {
			if line != "" {
				b.WriteString(" " + truncate(line, width-2) + "\n")
			}
		}
	} else {
		b.WriteString(styleDim.Render("  Waiting for logs…") + "\n")
	}

	return b.String()
}
