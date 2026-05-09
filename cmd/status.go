package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

type statusJSON struct {
	Services []statusService `json:"services"`
}

type statusService struct {
	Name   string `json:"name"`
	State  string `json:"state"`
	Status string `json:"status,omitempty"`
}

func newStatusCmd() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show health status of all services",
		RunE: func(cmd *cobra.Command, args []string) error {
			if output == "json" {
				return statusAsJSON(cmd)
			}

			c := exec.Command("docker", "compose", "ps", "--format", "table {{.Service}}\t{{.State}}\t{{.Status}}")
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				return fmt.Errorf("docker compose ps failed: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&output, "output", "text", "Output format: text or json")
	return cmd
}

func statusAsJSON(cmd *cobra.Command) error {
	c := exec.Command("docker", "compose", "ps", "--format", "{{.Service}}\t{{.State}}\t{{.Status}}")
	out, err := c.Output()
	if err != nil {
		return fmt.Errorf("docker compose ps failed: %w", err)
	}

	var result statusJSON
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 2 {
			continue
		}
		svc := statusService{
			Name:  strings.TrimSpace(parts[0]),
			State: strings.TrimSpace(parts[1]),
		}
		if len(parts) == 3 {
			svc.Status = strings.TrimSpace(parts[2])
		}
		result.Services = append(result.Services, svc)
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
