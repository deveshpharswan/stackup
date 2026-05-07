package cmd

import "github.com/spf13/cobra"

func NewRootCmd(version, commit, date string) *cobra.Command {
	root := &cobra.Command{
		Use:   "stackup",
		Short: "Smart Docker Compose orchestration for development teams",
		Long:  "Stackup wraps Docker Compose with health-gated startup, .env validation, and debug workflows.",
	}
	root.AddCommand(
		newVersionCmd(version, commit, date),
		newUpCmd(),
		newDownCmd(),
		newValidateCmd(),
		newStatusCmd(),
		newInitCmd(),
		newLogsCmd(),
		newShellCmd(),
		newRestartCmd(),
		newRunCmd(),
	)
	return root
}
