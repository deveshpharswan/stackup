package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newVersionCmd(version, commit, date string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version, commit SHA, and build date",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintf(cmd.OutOrStdout(), "stackup %s (commit: %s, built: %s)\n", version, commit, date)
		},
	}
}
