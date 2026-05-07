package cmd

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
	"github.com/stackup-dev/stackup/internal/config"
	"github.com/stackup-dev/stackup/internal/docker"
)

func newRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart <service>",
		Short: "Restart a service and re-run its health check",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			svcName := args[0]
			c, err := docker.NewClient()
			if err != nil {
				return err
			}
			defer c.Close()
			id, err := c.ContainerIDByName(svcName)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Restarting %s...\n", svcName)
			if err := c.Restart(ctx, id); err != nil {
				return err
			}
			cfg := config.LoadOrEmpty("stackup.yml")
			svc, ok := cfg.Services[svcName]
			if !ok || svc.Health == nil {
				fmt.Fprintf(cmd.OutOrStdout(), "%s restarted\n", svcName)
				return nil
			}
			dc, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "%s restarted (skipped health check)\n", svcName)
				return nil
			}
			defer dc.Close()
			checkers := buildCheckers(cfg, dc)
			named, ok := checkers[svcName]
			if !ok {
				fmt.Fprintf(cmd.OutOrStdout(), "%s restarted\n", svcName)
				return nil
			}
			if err := named.Checker.Check(ctx); err != nil {
				return fmt.Errorf("health check failed after restart: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s restarted and healthy\n", svcName)
			return nil
		},
	}
}
