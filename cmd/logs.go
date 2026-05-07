package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stackup-dev/stackup/internal/docker"
)

func newLogsCmd() *cobra.Command {
	var follow bool
	cmd := &cobra.Command{
		Use:   "logs [service]",
		Short: "Stream logs for a service",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := docker.NewClient()
			if err != nil {
				return err
			}
			defer c.Close()
			id, err := c.ContainerIDByName(args[0])
			if err != nil {
				return fmt.Errorf("service %q not found: %w", args[0], err)
			}
			return c.Logs(context.Background(), id, follow, os.Stdout)
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output")
	return cmd
}
