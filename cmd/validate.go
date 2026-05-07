package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stackup-dev/stackup/internal/config"
	"github.com/stackup-dev/stackup/internal/orchestrator"
	"github.com/stackup-dev/stackup/internal/printer"
)

func newValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate .env without starting any services",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.LoadOrEmpty("stackup.yml")
			o := orchestrator.New(printer.New(cmd.OutOrStdout()))
			if !o.PreFlight(".env", ".env.example", cfg.Env.Schema) {
				return fmt.Errorf("validation failed")
			}
			return nil
		},
	}
}
