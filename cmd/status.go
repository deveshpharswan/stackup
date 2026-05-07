package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show health status of all services",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := exec.Command("docker", "compose", "ps", "--format", "table {{.Service}}\t{{.State}}\t{{.Status}}")
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				return fmt.Errorf("docker compose ps failed: %w", err)
			}
			return nil
		},
	}
}
