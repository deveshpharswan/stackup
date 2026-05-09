package tui

import (
	"bufio"
	"os/exec"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

const sparklineLen = 8

var sparkChars = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

type StatsHistory struct {
	cpu []float64
	mem []float64
}

func (h *StatsHistory) Push(cpu, mem float64) {
	h.cpu = append(h.cpu, cpu)
	h.mem = append(h.mem, mem)
	if len(h.cpu) > sparklineLen {
		h.cpu = h.cpu[len(h.cpu)-sparklineLen:]
	}
	if len(h.mem) > sparklineLen {
		h.mem = h.mem[len(h.mem)-sparklineLen:]
	}
}

func renderSparkline(values []float64, maxVal float64) string {
	if len(values) == 0 {
		return strings.Repeat(string(sparkChars[0]), sparklineLen)
	}
	if maxVal <= 0 {
		maxVal = 100
	}
	var b strings.Builder
	pad := sparklineLen - len(values)
	for i := 0; i < pad; i++ {
		b.WriteRune(sparkChars[0])
	}
	for _, v := range values {
		idx := int((v / maxVal) * float64(len(sparkChars)-1))
		if idx < 0 {
			idx = 0
		}
		if idx >= len(sparkChars) {
			idx = len(sparkChars) - 1
		}
		b.WriteRune(sparkChars[idx])
	}
	return b.String()
}

func pollStats() tea.Cmd {
	return func() tea.Msg {
		c := exec.Command("docker", "stats", "--no-stream", "--format", "{{.Name}}\t{{.CPUPerc}}\t{{.MemPerc}}")
		out, err := c.Output()
		if err != nil {
			return StatsUpdateMsg{Stats: nil}
		}

		stats := make(map[string]ServiceStats)
		scanner := bufio.NewScanner(strings.NewReader(string(out)))
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.SplitN(line, "\t", 3)
			if len(parts) < 3 {
				continue
			}
			name := extractServiceName(parts[0])
			cpu := parsePercent(parts[1])
			mem := parsePercent(parts[2])
			stats[name] = ServiceStats{CPU: cpu, Memory: mem}
		}
		return StatsUpdateMsg{Stats: stats}
	}
}

func extractServiceName(containerName string) string {
	name := strings.TrimPrefix(containerName, "/")
	parts := strings.Split(name, "-")
	if len(parts) >= 2 {
		return strings.Join(parts[1:len(parts)-1], "-")
	}
	return name
}

func parsePercent(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "%")
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func statsTickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return statsTickMsg(t)
	})
}

type statsTickMsg time.Time
