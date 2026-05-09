package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/deveshpharswan/stackup/internal/config"
	"github.com/deveshpharswan/stackup/internal/constants"
	"github.com/deveshpharswan/stackup/internal/docker"
)

func newRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "run <command>",
		Short: "Run a named command from stackup.yml inside its configured container",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(constants.DefaultConfigFile)
			if err != nil {
				return fmt.Errorf("%s not found — no commands defined", constants.DefaultConfigFile)
			}
			named, ok := cfg.Commands[args[0]]
			if !ok {
				var keys []string
				for k := range cfg.Commands {
					keys = append(keys, k)
				}
				return fmt.Errorf("unknown command %q — available: %s", args[0], strings.Join(keys, ", "))
			}
			c, err := docker.NewClient()
			if err != nil {
				return err
			}
			defer c.Close()
			id, err := c.ContainerIDByName(named.Service)
			if err != nil {
				return err
			}
			return c.ExecShell(context.Background(), id, strings.NewReader(named.Run+"\n"), os.Stdout)
		},
	}
}
