package cmd

import (
	"context"
	"os"

	"github.com/spf13/cobra"
	"github.com/stackup-dev/stackup/internal/docker"
)

func newShellCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "shell <service>",
		Short: "Open an interactive shell inside a running container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := docker.NewClient()
			if err != nil {
				return err
			}
			defer c.Close()
			id, err := c.ContainerIDByName(args[0])
			if err != nil {
				return err
			}
			return c.ExecShell(context.Background(), id, os.Stdin, os.Stdout)
		},
	}
}
