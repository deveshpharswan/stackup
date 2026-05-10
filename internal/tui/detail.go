package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/deveshpharswan/stackup/internal/constants"
	"github.com/deveshpharswan/stackup/internal/scaffold"
)

type DetailTab int

const (
	DetailTabOverview DetailTab = iota
	DetailTabLogViewer
	DetailTabEnv
	DetailTabPorts
	DetailTabVolumes
	DetailTabConfig
)

var detailTabLabels = []string{"overview", "logs", "env", "ports", "volumes", "config"}

type InspectData struct {
	Env   []string // "KEY=value" format
	Binds []string // "/host:/container:rw" format
	Ports []PortMapping
}

type PortMapping struct {
	HostPort      string
	ContainerPort string
	Proto         string
	Conflict      bool
}

type DetailModel struct {
	service      ServiceInfo
	services     []ServiceInfo
	statsHistory map[string]*StatsHistory
	tab          DetailTab
	inspect      *InspectData
	envMasked    bool
	viewport     viewport.Model
	ready        bool
}

func NewDetailModel() DetailModel {
	return DetailModel{envMasked: true}
}

func (m DetailModel) SetService(svc ServiceInfo, services []ServiceInfo, history map[string]*StatsHistory) DetailModel {
	m.service = svc
	m.services = services
	m.statsHistory = history
	m.ready = true
	m.inspect = nil // reset inspect; will re-fetch
	return m
}

func (m DetailModel) Update(msg tea.Msg) (DetailModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab":
			m.tab = DetailTab((int(m.tab) + 1) % len(detailTabLabels))
		case "shift+tab":
			m.tab = DetailTab((int(m.tab) - 1 + len(detailTabLabels)) % len(detailTabLabels))
		case "v":
			m.envMasked = !m.envMasked
		}
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	case ServiceUpdateMsg:
		if msg.Err == nil {
			for _, s := range msg.Services {
				if s.Name == m.service.Name {
					m.service = s
					break
				}
			}
			m.services = msg.Services
		}
	case InspectResultMsg:
		if msg.Service == m.service.Name && msg.Err == nil {
			m.inspect = &msg.Data
		}
	case tea.WindowSizeMsg:
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height
	}
	return m, nil
}

func (m DetailModel) View(width, height int) string {
	if !m.ready {
		return styleDim.Render("  Select a service")
	}
	switch m.tab {
	case DetailTabOverview:
		return m.viewOverview(width, height)
	case DetailTabLogViewer:
		return m.viewLogPlaceholder(width, height)
	case DetailTabEnv:
		return m.viewEnv(width, height)
	case DetailTabPorts:
		return m.viewPorts(width, height)
	case DetailTabVolumes:
		return m.viewVolumes(width, height)
	case DetailTabConfig:
		return m.viewConfig(width, height)
	}
	return ""
}

func (m DetailModel) renderSubTabBar(width int) string {
	var parts []string
	for i, label := range detailTabLabels {
		if DetailTab(i) == m.tab {
			parts = append(parts, styleInfo.Bold(true).Render("["+label+"]"))
		} else {
			parts = append(parts, styleDim.Render(" "+label+" "))
		}
	}
	bar := strings.Join(parts, styleDim.Render("│"))
	return lipgloss.NewStyle().
		Background(lipgloss.Color("#0d1117")).
		Width(width).
		Padding(0, 1).
		Render(bar)
}

func (m DetailModel) viewOverview(width, height int) string {
	var b strings.Builder

	// Panel header
	dot, nameStyle, _ := svcStatusParts(m.service)
	healthLabel := m.service.Health
	if healthLabel == "" || healthLabel == "(none)" {
		healthLabel = m.service.State
	}
	hdr := fmt.Sprintf("%s %s — %s",
		dot,
		nameStyle.Bold(true).Render(m.service.Name),
		nameStyle.Render(healthLabel))
	if m.service.Ports != "" {
		hdr += styleDim.Render("  ·  ") + styleInfo.Render(m.service.Ports)
	}
	b.WriteString(lipgloss.NewStyle().
		Background(lipgloss.Color("#0f1117")).
		Width(width).
		Padding(0, 1).
		Render(hdr) + "\n")
	b.WriteString(m.renderSubTabBar(width) + "\n")

	// Resource rows
	cpuSpark, memSpark := "—", "—"
	cpuVal, memVal := "—", "—"
	if m.statsHistory != nil {
		if hist, ok := m.statsHistory[m.service.Name]; ok && len(hist.cpu) > 0 {
			cpuSpark = styleInfo.Render(renderSparkline(hist.cpu, 100))
			memSpark = styleInfo.Render(renderSparkline(hist.mem, 100))
			cpuVal = styleHealthy.Render(fmt.Sprintf("%.1f%%", m.service.CPU))
			memVal = styleInfo.Render(fmt.Sprintf("%.1f%%", m.service.Memory))
		}
	}

	row := func(key, val string) string {
		return fmt.Sprintf("  %-10s %s\n", styleDim.Render(key), val)
	}

	b.WriteString(row("cpu", cpuSpark+" "+cpuVal))
	b.WriteString(row("memory", memSpark+" "+memVal))
	b.WriteString(row("uptime", styleBold.Render(formatUptime(m.service.Uptime))))

	b.WriteString("\n")
	b.WriteString(m.renderMiniTable(width) + "\n")
	return b.String()
}

func (m DetailModel) renderMiniTable(width int) string {
	if width < 60 {
		return styleDim.Render("  Terminal too narrow for table\n")
	}
	var b strings.Builder
	hdr := fmt.Sprintf("  %-14s %-10s %-12s %-7s %-7s %s",
		"NAME", "STATE", "HEALTH", "CPU", "MEM", "UPTIME")
	b.WriteString(styleInfo.Bold(true).Render(hdr) + "\n")
	b.WriteString(styleDim.Render("  "+strings.Repeat("─", width-4)) + "\n")

	for _, svc := range m.services {
		cpuStr := fmt.Sprintf("%.1f%%", svc.CPU)
		memStr := fmt.Sprintf("%.1f%%", svc.Memory)
		row := fmt.Sprintf("  %-14s %-10s %-12s %-7s %-7s %s",
			truncate(svc.Name, 14), svc.State, svc.Health,
			cpuStr, memStr, formatUptime(svc.Uptime))
		_, rowStyle, _ := svcStatusParts(svc)
		if svc.Name == m.service.Name {
			b.WriteString(styleSelected.Width(width).Render(row) + "\n")
		} else {
			b.WriteString(rowStyle.Render(row) + "\n")
		}
	}
	return b.String()
}

func (m DetailModel) viewLogPlaceholder(width, height int) string {
	var b strings.Builder
	b.WriteString(m.renderSubTabBar(width) + "\n")
	b.WriteString(styleDim.Render("  Switch to Logs tab (2) for full-screen log view.\n"))
	b.WriteString(styleDim.Render("  The right panel shows live log tail for this service.\n"))
	return b.String()
}

func (m DetailModel) viewEnv(width, height int) string {
	var b strings.Builder
	b.WriteString(m.renderSubTabBar(width) + "\n")
	if m.inspect == nil {
		b.WriteString(styleDim.Render("  Loading… (fetching container inspect)\n"))
		return b.String()
	}
	hint := styleDim.Render("  v") + styleDim.Render(" to reveal/mask all values") + "\n"
	b.WriteString(hint)
	b.WriteString(styleDim.Render("  "+strings.Repeat("─", width-4)) + "\n")
	for _, env := range m.inspect.Env {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := styleInfo.Render(parts[0])
		val := parts[1]
		if m.envMasked {
			val = styleDim.Render("****")
		} else {
			val = styleBold.Render(val)
		}
		b.WriteString(fmt.Sprintf("  %-30s %s\n", key, val))
	}
	return b.String()
}

func (m DetailModel) viewPorts(width, height int) string {
	var b strings.Builder
	b.WriteString(m.renderSubTabBar(width) + "\n")
	if m.inspect == nil {
		b.WriteString(styleDim.Render("  Loading…\n"))
		return b.String()
	}
	if len(m.inspect.Ports) == 0 {
		b.WriteString(styleDim.Render("  No port bindings.\n"))
		return b.String()
	}
	b.WriteString(styleDim.Render("  "+strings.Repeat("─", width-4)) + "\n")
	for _, p := range m.inspect.Ports {
		binding := fmt.Sprintf("  0.0.0.0:%-6s → %s/%s", p.HostPort, p.ContainerPort, p.Proto)
		if p.Conflict {
			b.WriteString(styleWarning.Render(binding) + "  " + styleFailed.Render("⚠ conflict") + "\n")
		} else {
			b.WriteString(styleInfo.Render(binding) + "\n")
		}
	}
	return b.String()
}

func (m DetailModel) viewVolumes(width, height int) string {
	var b strings.Builder
	b.WriteString(m.renderSubTabBar(width) + "\n")
	if m.inspect == nil {
		b.WriteString(styleDim.Render("  Loading…\n"))
		return b.String()
	}
	if len(m.inspect.Binds) == 0 {
		b.WriteString(styleDim.Render("  No volume binds.\n"))
		return b.String()
	}
	b.WriteString(styleDim.Render("  "+strings.Repeat("─", width-4)) + "\n")
	for _, bind := range m.inspect.Binds {
		parts := strings.SplitN(bind, ":", 3)
		mode := "rw"
		if len(parts) == 3 {
			mode = parts[2]
		}
		src, dst := "", ""
		if len(parts) >= 2 {
			src = parts[0]
			dst = parts[1]
		}
		modeStyle := styleDim
		if mode == "ro" {
			modeStyle = styleWarning
		}
		b.WriteString(fmt.Sprintf("  %s → %s  %s\n",
			styleDim.Render(truncate(src, 30)),
			styleInfo.Render(truncate(dst, 30)),
			modeStyle.Render(mode)))
	}
	return b.String()
}

func (m DetailModel) viewConfig(width, height int) string {
	var b strings.Builder
	b.WriteString(m.renderSubTabBar(width) + "\n")
	composePath := constants.FindComposeFile(".")
	if composePath == "" {
		composePath = constants.DefaultComposeFile
	}
	composeSvcs, err := scaffold.ParseServices(composePath)
	if err != nil {
		b.WriteString(styleFailed.Render("  Error reading compose file: "+err.Error()) + "\n")
		return b.String()
	}
	deps := composeSvcs[m.service.Name]
	if len(deps) == 0 {
		b.WriteString(styleDim.Render("  No dependencies defined.\n"))
	} else {
		b.WriteString(styleBold.Render("  depends_on:") + "\n")
		for _, dep := range deps {
			b.WriteString(styleInfo.Render("    - "+dep) + "\n")
		}
	}
	return b.String()
}
