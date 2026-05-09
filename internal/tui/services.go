package tui

import (
	"bufio"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type ServicesModel struct {
	services     []ServiceInfo
	filtered     []ServiceInfo
	cursor       int
	filter       string
	errorZoom    bool
	statsHistory map[string]*StatsHistory
	err          error
}

func NewServicesModel() ServicesModel {
	return ServicesModel{
		statsHistory: make(map[string]*StatsHistory),
	}
}

func (m ServicesModel) Init() tea.Cmd {
	return tea.Batch(pollServices(), tickEvery(2*time.Second), pollStats(), statsTickEvery(5*time.Second))
}

func (m ServicesModel) Update(msg tea.Msg) (ServicesModel, tea.Cmd) {
	switch msg := msg.(type) {
	case ServiceUpdateMsg:
		if msg.Err != nil {
			m.err = msg.Err
			return m, nil
		}
		m.services = msg.Services
		m.err = nil
		m.applyFilter()
	case StatsUpdateMsg:
		if msg.Stats != nil {
			for name, s := range msg.Stats {
				h, ok := m.statsHistory[name]
				if !ok {
					h = &StatsHistory{}
					m.statsHistory[name] = h
				}
				h.Push(s.CPU, s.Memory)
			}
			for i := range m.services {
				if s, ok := msg.Stats[m.services[i].Name]; ok {
					m.services[i].CPU = s.CPU
					m.services[i].Memory = s.Memory
				}
			}
			m.applyFilter()
		}
	case statsTickMsg:
		return m, tea.Batch(pollStats(), statsTickEvery(5*time.Second))
	case TickMsg:
		return m, tea.Batch(pollServices(), tickEvery(2*time.Second))
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case "r":
			if svc := m.Selected(); svc != "" {
				return m, func() tea.Msg {
					return ConfirmRequestMsg{Action: ConfirmRestart, Service: svc}
				}
			}
		case "x":
			if svc := m.Selected(); svc != "" {
				return m, func() tea.Msg {
					return ConfirmRequestMsg{Action: ConfirmDelete, Service: svc}
				}
			}
		case "s":
			if svc := m.Selected(); svc != "" {
				return m, func() tea.Msg {
					return shellRequestMsg{Service: svc}
				}
			}
		}
	}
	return m, nil
}

func (m ServicesModel) View(width, height int) string {
	if m.err != nil {
		return styleWarning.Render(fmt.Sprintf("  Docker error: %v\n\n", m.err)) +
			styleDim.Render("  Is Docker running?")
	}
	if len(m.filtered) == 0 {
		return styleDim.Render("  No services found")
	}

	var b strings.Builder
	if m.errorZoom {
		b.WriteString(styleFailed.Render("  [ERROR ZOOM]") + " " + styleDim.Render("press e to show all") + "\n")
	}
	header := fmt.Sprintf("  %-14s %-10s %-10s %-8s %-10s %s  %s",
		"NAME", "STATE", "HEALTH", "CPU", "MEM", "UPTIME", "ACTIVITY")
	b.WriteString(styleInfo.Bold(true).Render(header) + "\n")
	b.WriteString(styleDim.Render("  "+strings.Repeat("─", width-4)) + "\n")

	for i, svc := range m.filtered {
		maxRows := height - 3
		if m.errorZoom {
			maxRows--
		}
		if i >= maxRows {
			break
		}
		row := m.renderRow(svc, i == m.cursor, width)
		b.WriteString(row + "\n")
	}

	running, healthy, failed := 0, 0, 0
	for _, s := range m.services {
		switch {
		case s.State == "running" && s.Health == "healthy":
			running++
			healthy++
		case s.State == "running":
			running++
		default:
			failed++
		}
	}
	b.WriteString("\n" + styleDim.Render("  "+strings.Repeat("─", width-4)) + "\n")
	summary := fmt.Sprintf("  %s   %s   %s",
		styleHealthy.Render(fmt.Sprintf("✓ %d healthy", healthy)),
		styleWarning.Render(fmt.Sprintf("⠋ %d starting", running-healthy)),
		styleFailed.Render(fmt.Sprintf("✗ %d failed", failed)))
	b.WriteString(summary)

	return b.String()
}

func (m ServicesModel) renderRow(svc ServiceInfo, selected bool, width int) string {
	uptime := formatUptime(svc.Uptime)
	cpuStr := fmt.Sprintf("%.1f%%", svc.CPU)
	memStr := fmt.Sprintf("%.1f%%", svc.Memory)

	spark := ""
	if h, ok := m.statsHistory[svc.Name]; ok && len(h.cpu) > 0 {
		spark = renderSparkline(h.cpu, 100)
	}

	row := fmt.Sprintf("  %-14s %-10s %-10s %-8s %-10s %-7s %s",
		svc.Name, svc.State, svc.Health, cpuStr, memStr, uptime, spark)

	if selected {
		return styleSelected.Width(width).Render(row)
	}

	switch {
	case svc.State == "running" && svc.Health == "healthy":
		return styleHealthy.Render(row)
	case svc.State == "running" && (svc.Health == "starting" || svc.Health == ""):
		return styleWarning.Render(row)
	case svc.State == "exited" || svc.State == "restarting":
		return styleFailed.Render(row)
	default:
		return styleDim.Render(row)
	}
}

func (m ServicesModel) Count() int {
	return len(m.filtered)
}

func (m ServicesModel) Services() []ServiceInfo {
	return m.services
}

func (m ServicesModel) Selected() string {
	if m.cursor < len(m.filtered) {
		return m.filtered[m.cursor].Name
	}
	return ""
}

func (m ServicesModel) SetFilter(f string) ServicesModel {
	m.filter = f
	m.applyFilter()
	m.cursor = 0
	return m
}

func (m ServicesModel) ToggleErrorZoom() ServicesModel {
	m.errorZoom = !m.errorZoom
	m.applyFilter()
	m.cursor = 0
	return m
}

func (m ServicesModel) ErrorZoom() bool {
	return m.errorZoom
}

func (m *ServicesModel) applyFilter() {
	var base []ServiceInfo
	if m.errorZoom {
		for _, s := range m.services {
			if s.Health == "unhealthy" || s.State == "exited" || s.State == "restarting" {
				base = append(base, s)
			}
		}
	} else {
		base = m.services
	}

	if m.filter == "" {
		m.filtered = base
		return
	}
	re, err := regexp.Compile("(?i)" + m.filter)
	if err != nil {
		m.filtered = base
		return
	}
	var filtered []ServiceInfo
	for _, s := range base {
		if re.MatchString(s.Name) {
			filtered = append(filtered, s)
		}
	}
	m.filtered = filtered
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func pollServices() tea.Cmd {
	return func() tea.Msg {
		c := exec.Command("docker", "compose", "ps", "--format",
			"{{.Service}}\t{{.State}}\t{{.Status}}\t{{.Ports}}")
		out, err := c.Output()
		if err != nil {
			return ServiceUpdateMsg{Err: fmt.Errorf("docker compose ps: %w", err)}
		}

		var services []ServiceInfo
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			parts := strings.SplitN(line, "\t", 4)
			if len(parts) < 2 {
				continue
			}
			svc := ServiceInfo{
				Name:  strings.TrimSpace(parts[0]),
				State: strings.TrimSpace(parts[1]),
			}
			if len(parts) >= 3 {
				svc.Health = parseHealthStatus(parts[2])
				svc.Uptime = parseUptime(parts[2])
			}
			if len(parts) >= 4 {
				svc.Ports = strings.TrimSpace(parts[3])
			}
			services = append(services, svc)
		}
		return ServiceUpdateMsg{Services: services}
	}
}

func parseHealthStatus(status string) string {
	lower := strings.ToLower(status)
	switch {
	case strings.Contains(lower, "unhealthy"):
		return "unhealthy"
	case strings.Contains(lower, "healthy"):
		return "healthy"
	case strings.Contains(lower, "starting"):
		return "starting"
	default:
		return "(none)"
	}
}

func parseUptime(status string) time.Duration {
	lower := strings.ToLower(status)
	if !strings.Contains(lower, "up") {
		return 0
	}
	re := regexp.MustCompile(`up\s+(?:about\s+)?(\d+)\s*(second|minute|hour|day)`)
	matches := re.FindStringSubmatch(lower)
	if len(matches) < 3 {
		return 0
	}
	n := 0
	fmt.Sscanf(matches[1], "%d", &n)
	switch matches[2] {
	case "second":
		return time.Duration(n) * time.Second
	case "minute":
		return time.Duration(n) * time.Minute
	case "hour":
		return time.Duration(n) * time.Hour
	case "day":
		return time.Duration(n) * 24 * time.Hour
	}
	return 0
}

func tickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}
