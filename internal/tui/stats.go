package tui

import (
	"bufio"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"

	tea "github.com/charmbracelet/bubbletea"
)

const sparklineLen = 16

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
	return renderSparklineN(values, maxVal, sparklineLen)
}

func renderSparklineN(values []float64, maxVal float64, n int) string {
	if n <= 0 {
		n = sparklineLen
	}
	if len(values) == 0 {
		return strings.Repeat(string(sparkChars[0]), n)
	}
	if maxVal <= 0 {
		maxVal = 100
	}
	var b strings.Builder
	// Use last n values
	start := len(values) - n
	if start < 0 {
		// Pad with empty chars
		for i := 0; i < -start; i++ {
			b.WriteRune(sparkChars[0])
		}
		start = 0
	}
	for _, v := range values[start:] {
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

// pollStats collects CPU and memory percentages for each compose service.
// It uses `docker compose ps` for project-scoped container ID resolution,
// then calls the Docker SDK ContainerStats for reliable cross-platform stats.
func pollStats(dc *dockerclient.Client) tea.Cmd {
	return func() tea.Msg {
		if dc == nil {
			return StatsUpdateMsg{Stats: nil}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Get project-scoped container IDs via compose CLI (avoids mixing projects).
		psOut, err := exec.CommandContext(ctx, "docker", "compose", "ps",
			"--format", "{{.ID}}\t{{.Service}}").Output()
		if err != nil {
			return StatsUpdateMsg{Stats: nil}
		}
		idToService := make(map[string]string)
		scanner := bufio.NewScanner(strings.NewReader(string(psOut)))
		for scanner.Scan() {
			parts := strings.SplitN(scanner.Text(), "\t", 2)
			if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
				idToService[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
			}
		}
		if len(idToService) == 0 {
			return StatsUpdateMsg{Stats: nil}
		}

		stats := make(map[string]ServiceStats)
		for id, svcName := range idToService {
			resp, err := dc.ContainerStats(ctx, id, false)
			if err != nil {
				continue
			}
			var s container.StatsResponse
			if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
				resp.Body.Close()
				continue
			}
			resp.Body.Close()
			stats[svcName] = ServiceStats{
				CPU:    calcCPUPercent(&s),
				Memory: calcMemPercent(&s),
			}
		}
		if len(stats) == 0 {
			return StatsUpdateMsg{Stats: nil}
		}
		return StatsUpdateMsg{Stats: stats}
	}
}

// calcCPUPercent computes the CPU usage percentage from a stats sample.
func calcCPUPercent(s *container.StatsResponse) float64 {
	cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage) - float64(s.PreCPUStats.CPUUsage.TotalUsage)
	sysDelta := float64(s.CPUStats.SystemUsage) - float64(s.PreCPUStats.SystemUsage)
	numCPUs := float64(s.CPUStats.OnlineCPUs)
	if numCPUs == 0 {
		numCPUs = float64(len(s.CPUStats.CPUUsage.PercpuUsage))
	}
	if numCPUs == 0 {
		numCPUs = 1
	}
	if sysDelta > 0 && cpuDelta > 0 {
		return (cpuDelta / sysDelta) * numCPUs * 100.0
	}
	return 0
}

// calcMemPercent computes memory usage percentage.
// Prefers inactive_file (cgroups v2) over cache (cgroups v1).
func calcMemPercent(s *container.StatsResponse) float64 {
	if s.MemoryStats.Limit == 0 {
		return 0
	}
	cacheUsage := s.MemoryStats.Stats["inactive_file"]
	if cacheUsage == 0 {
		cacheUsage = s.MemoryStats.Stats["cache"]
	}
	used := float64(s.MemoryStats.Usage) - float64(cacheUsage)
	if used < 0 {
		used = 0
	}
	return (used / float64(s.MemoryStats.Limit)) * 100.0
}

func statsTickEvery(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(t time.Time) tea.Msg {
		return statsTickMsg(t)
	})
}

type statsTickMsg time.Time
