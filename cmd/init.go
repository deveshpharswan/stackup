package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stackup-dev/stackup/internal/scaffold"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Generate a starter stackup.yml from docker-compose.yml and .env.example",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := os.Stat("stackup.yml"); err == nil {
				return fmt.Errorf("stackup.yml already exists — delete it first if you want to regenerate")
			}
			out, err := scaffold.Generate("docker-compose.yml", ".env.example")
			if err != nil {
				return err
			}
			if err := os.WriteFile("stackup.yml", []byte(out), 0644); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "stackup.yml generated — review and customise health check types before running `stackup up`")
			return nil
		},
	}
}
