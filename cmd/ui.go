package cmd

import (
	"github.com/deveshpharswan/stackup/internal/tui"
	"github.com/spf13/cobra"
)

func newUICmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Interactive terminal dashboard for managing your stack",
		Long:  "Launch a k9s-style TUI with real-time service status, log streaming, diagnostics, and dependency visualization.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run()
		},
	}
	return cmd
}
