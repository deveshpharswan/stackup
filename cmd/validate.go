package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/deveshpharswan/stackup/internal/config"
	"github.com/deveshpharswan/stackup/internal/constants"
	"github.com/deveshpharswan/stackup/internal/orchestrator"
	"github.com/deveshpharswan/stackup/internal/printer"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate .env without starting any services",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.LoadOrEmpty(constants.DefaultConfigFile)
			o := orchestrator.New(printer.New(cmd.OutOrStdout()))
			if !o.PreFlight(constants.DefaultEnvFile, constants.DefaultExampleFile, cfg.Env.Schema) {
				return fmt.Errorf("validation failed")
			}
			return nil
		},
	}
}
