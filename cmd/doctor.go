package cmd

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/deveshpharswan/stackup/internal/constants"
	"github.com/deveshpharswan/stackup/internal/doctor"
)

type doctorJSON struct {
	Findings []doctorFinding `json:"findings"`
	Summary  doctorSummary   `json:"summary"`
}

type doctorFinding struct {
	Severity string `json:"severity"`
	Title    string `json:"title"`
	Detail   string `json:"detail,omitempty"`
	Fix      string `json:"fix,omitempty"`
	Service  string `json:"service,omitempty"`
}

type doctorSummary struct {
	Errors   int `json:"errors"`
	Warnings int `json:"warnings"`
	OK       int `json:"ok"`
}

func newDoctorCmd() *cobra.Command {
	var output string

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run automated diagnostics on your stack",
		Long:  "Checks for port conflicts, crash loops, env drift, container status, and localhost misuse.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if output != "text" && output != "json" {
				return fmt.Errorf("unknown output format %q — must be text or json", output)
			}
			opts := &doctor.Options{
				ComposeFile: constants.DefaultComposeFile,
				EnvFile:     constants.DefaultEnvFile,
				ExampleFile: constants.DefaultExampleFile,
				ConfigFile:  constants.DefaultConfigFile,
			}

			d := doctor.New()
			ctx := context.Background()
			findings := d.Run(ctx, opts)

			if output == "json" {
				return printDoctorJSON(cmd, findings)
			}

			doctor.PrintFindings(cmd.OutOrStdout(), findings)
			return nil
		},
	}

	cmd.Flags().StringVar(&output, "output", "text", "Output format: text or json")
	return cmd
}

func printDoctorJSON(cmd *cobra.Command, findings []doctor.Finding) error {
	result := doctorJSON{}
	for _, f := range findings {
		result.Findings = append(result.Findings, doctorFinding{
			Severity: severityString(f.Severity),
			Title:    f.Title,
			Detail:   f.Detail,
			Fix:      f.Fix,
			Service:  f.Service,
		})
		switch f.Severity {
		case doctor.SeverityError:
			result.Summary.Errors++
		case doctor.SeverityWarning:
			result.Summary.Warnings++
		case doctor.SeverityOK:
			result.Summary.OK++
		}
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func severityString(s doctor.Severity) string {
	switch s {
	case doctor.SeverityError:
		return "error"
	case doctor.SeverityWarning:
		return "warning"
	case doctor.SeverityOK:
		return "ok"
	default:
		return "unknown"
	}
}
