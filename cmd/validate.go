package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/deveshpharswan/stackup/internal/config"
	"github.com/deveshpharswan/stackup/internal/constants"
	"github.com/deveshpharswan/stackup/internal/env"
	"github.com/deveshpharswan/stackup/internal/orchestrator"
	"github.com/deveshpharswan/stackup/internal/printer"
)

type validateJSON struct {
	Valid    bool            `json:"valid"`
	Errors   []validateErr  `json:"errors,omitempty"`
	Injected map[string]string `json:"injected,omitempty"`
}

type validateErr struct {
	Key     string `json:"key"`
	Message string `json:"message"`
}

func newValidateCmd() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate .env without starting any services",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadOrEmpty(constants.DefaultConfigFile)
			if err != nil {
				return fmt.Errorf("invalid stackup.yml: %w", err)
			}

			if output == "json" {
				return validateAsJSON(cmd, cfg)
			}

			o := orchestrator.New(printer.New(cmd.OutOrStdout()))
			ok, _ := o.PreFlight(constants.DefaultEnvFile, constants.DefaultExampleFile, cfg.Env.Schema)
			if !ok {
				return fmt.Errorf("validation failed")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&output, "output", "text", "Output format: text or json")
	return cmd
}

func validateAsJSON(cmd *cobra.Command, cfg *config.Config) error {
	result, injected := env.ValidateWithDefaults(
		constants.DefaultEnvFile,
		constants.DefaultExampleFile,
		cfg.Env.Schema,
	)

	out := validateJSON{
		Valid:    result.Valid(),
		Injected: injected,
	}
	if out.Injected == nil {
		out.Injected = map[string]string{}
	}
	for _, e := range result.Errors {
		out.Errors = append(out.Errors, validateErr{Key: e.Key, Message: e.Message})
	}

	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return err
	}

	if !result.Valid() {
		return &ExitError{Code: 1, Message: "validation failed"}
	}
	return nil
}
