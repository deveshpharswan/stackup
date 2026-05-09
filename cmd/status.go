package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/deveshpharswan/stackup/internal/printer"
)

type statusJSON struct {
	Services []statusService `json:"services"`
}

type statusService struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Status string `json:"status,omitempty"`
	Ports  string `json:"ports,omitempty"`
}

func newStatusCmd() *cobra.Command {
	var (
		output string
		watch  bool
	)

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show health status of all services",
		RunE: func(cmd *cobra.Command, args []string) error {
			if watch && output == "json" {
				return fmt.Errorf("--watch and --output json cannot be used together")
			}

			if output == "json" {
				return statusAsJSON(cmd)
			}

			p := printer.New(cmd.OutOrStdout())

			if watch {
				return statusWatch(cmd, p)
			}

			return statusOnce(cmd, p)
		},
	}

	cmd.Flags().StringVar(&output, "output", "text", "Output format: text or json")
	cmd.Flags().BoolVar(&watch, "watch", false, "Continuously refresh status every 2 seconds")
	return cmd
}

func statusOnce(cmd *cobra.Command, p *printer.Printer) error {
	services, err := fetchServices()
	if err != nil {
		return err
	}
	renderStatusTable(cmd, p, services, false)
	return nil
}

func statusWatch(cmd *cobra.Command, p *printer.Printer) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	// Initial render
	services, err := fetchServices()
	if err != nil {
		return err
	}
	p.ClearScreen()
	renderStatusTable(cmd, p, services, true)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Fprintln(cmd.OutOrStdout())
			return nil
		case <-ticker.C:
			services, err = fetchServices()
			if err != nil {
				continue
			}
			p.ClearScreen()
			renderStatusTable(cmd, p, services, true)
		}
	}
}

func fetchServices() ([]statusService, error) {
	c := exec.Command("docker", "compose", "ps", "--format", "{{.Service}}\t{{.State}}\t{{.Status}}\t{{.Ports}}")
	out, err := c.Output()
	if err != nil {
		return nil, fmt.Errorf("docker compose ps failed: %w", err)
	}

	var services []statusService
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
		svc := statusService{
			Name:  strings.TrimSpace(parts[0]),
			State: strings.TrimSpace(parts[1]),
		}
		if len(parts) >= 3 {
			svc.Status = strings.TrimSpace(parts[2])
		}
		if len(parts) >= 4 {
			svc.Ports = strings.TrimSpace(parts[3])
		}
		services = append(services, svc)
	}
	return services, nil
}

func renderStatusTable(cmd *cobra.Command, p *printer.Printer, services []statusService, watching bool) {
	w := cmd.OutOrStdout()

	// Header
	if watching {
		ts := time.Now().Format("15:04:05")
		fmt.Fprintf(w, "%s  %s\n", p.Bold("Stackup Status"), p.Dim("(watching — Ctrl+C to exit)          "+ts))
	} else {
		fmt.Fprintf(w, "%s\n", p.Bold("Stackup Status"))
	}
	fmt.Fprintf(w, "%s\n", p.Dim(strings.Repeat("─", 65)))

	// Column headers
	fmt.Fprintf(w, "  %-14s %-12s %-10s %s\n",
		p.Bold("Service"),
		p.Bold("State"),
		p.Bold("Health"),
		p.Bold("Ports"),
	)
	fmt.Fprintf(w, "  %s\n", p.Dim(strings.Repeat("─", 61)))

	// Service rows
	var running, healthy, failed int
	for _, svc := range services {
		state := strings.ToLower(svc.State)
		healthStr := parseHealth(svc.Status)
		ports := svc.Ports
		if ports == "" {
			ports = "—"
		}

		// Color logic
		var nameStr, stateStr, healthColorStr string
		switch {
		case state == "running" && healthStr == "healthy":
			nameStr = p.Green(svc.Name)
			stateStr = p.Green(state)
			healthColorStr = p.Green(healthStr)
			running++
			healthy++
		case state == "running" && healthStr == "(none)":
			nameStr = svc.Name
			stateStr = p.Green(state)
			healthColorStr = p.Dim(healthStr)
			running++
		case state == "running":
			nameStr = p.Yellow(svc.Name)
			stateStr = p.Green(state)
			healthColorStr = p.Yellow(healthStr)
			running++
		case state == "exited" || state == "restarting":
			nameStr = p.Red(svc.Name)
			stateStr = p.Red(state)
			healthColorStr = p.Red("failed")
			failed++
		default:
			nameStr = p.Yellow(svc.Name)
			stateStr = p.Yellow(state)
			healthColorStr = p.Dim(healthStr)
		}

		fmt.Fprintf(w, "  %-14s %-12s %-10s %s\n", nameStr, stateStr, healthColorStr, p.Dim(ports))
	}

	// Footer summary
	fmt.Fprintf(w, "%s\n", p.Dim(strings.Repeat("─", 65)))
	fmt.Fprintf(w, "  %s  │  %s  │  %s  │  %s\n",
		p.Bold(fmt.Sprintf("%d services", len(services))),
		p.Green(fmt.Sprintf("%d running", running)),
		p.Green(fmt.Sprintf("%d healthy", healthy)),
		colorCount(p, failed, "failed"),
	)
}

func parseHealth(status string) string {
	lower := strings.ToLower(status)
	if strings.Contains(lower, "healthy") {
		return "healthy"
	}
	if strings.Contains(lower, "unhealthy") {
		return "unhealthy"
	}
	if strings.Contains(lower, "starting") {
		return "starting"
	}
	return "(none)"
}

func colorCount(p *printer.Printer, count int, label string) string {
	s := fmt.Sprintf("%d %s", count, label)
	if count > 0 {
		return p.Red(s)
	}
	return p.Dim(s)
}

func statusAsJSON(cmd *cobra.Command) error {
	services, err := fetchServices()
	if err != nil {
		return err
	}

	result := statusJSON{Services: services}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
