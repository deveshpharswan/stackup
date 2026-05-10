package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/deveshpharswan/stackup/internal/constants"
	"github.com/deveshpharswan/stackup/internal/scaffold"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Generate a starter stackup.yml from docker-compose.yml and .env.example",
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := os.Stat(constants.DefaultConfigFile); err == nil {
				return fmt.Errorf("%s already exists — delete it first if you want to regenerate", constants.DefaultConfigFile)
			}
			composePath, err := resolveComposeFile()
			if err != nil {
				return err
			}
			out, err := scaffold.Generate(composePath, constants.DefaultExampleFile)
			if err != nil {
				return err
			}
			if err := os.WriteFile(constants.DefaultConfigFile, []byte(out), 0644); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s generated — review and customise health check types before running `stackup up`\n", constants.DefaultConfigFile)
			return nil
		},
	}
}
