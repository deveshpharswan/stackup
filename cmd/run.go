package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/deveshpharswan/stackup/internal/config"
	"github.com/deveshpharswan/stackup/internal/constants"
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
			c := exec.Command("docker", "compose", "exec", "-T", named.Service, "sh", "-c", named.Run)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		},
	}
}
