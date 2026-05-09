package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/stackup-dev/stackup/internal/doctor"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run automated diagnostics on your stack",
		Long:  "Checks for port conflicts, crash loops, env drift, container status, and localhost misuse.",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := &doctor.Options{
				ComposeFile: "docker-compose.yml",
				EnvFile:     ".env",
				ExampleFile: ".env.example",
				ConfigFile:  "stackup.yml",
			}

			d := doctor.New()
			ctx := context.Background()
			findings := d.Run(ctx, opts)

			doctor.PrintFindings(cmd.OutOrStdout(), findings)
			return nil
		},
	}
}
