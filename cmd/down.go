package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func newDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Stop and remove all containers in the stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := exec.Command("docker", "compose", "down", "--remove-orphans")
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				return fmt.Errorf("docker compose down failed: %w", err)
			}
			return nil
		},
	}
}
