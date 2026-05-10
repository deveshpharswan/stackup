package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/deveshpharswan/stackup/internal/constants"
)

var composeFile string

func NewRootCmd(version, commit, date string) *cobra.Command {
	root := &cobra.Command{
		Use:   "stackup",
		Short: "Smart Docker Compose orchestration for development teams",
		Long:  "Stackup wraps Docker Compose with health-gated startup, .env validation, and debug workflows.",
	}
	root.PersistentFlags().StringVarP(&composeFile, "compose-file", "f", "", "Path to docker compose file (default: auto-discover)")
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
		newDoctorCmd(),
		newCheckCmd(),
		newCompletionCmd(),
		newUICmd(),
	)
	return root
}

// resolveComposeFile returns the explicit --compose-file value if set,
// otherwise auto-discovers from the current directory.
// Returns an error if neither a flag was given nor a file was found.
func resolveComposeFile() (string, error) {
	if composeFile != "" {
		return composeFile, nil
	}
	found := constants.FindComposeFile(".")
	if found == "" {
		return "", fmt.Errorf("no compose file found (looked for compose.yaml, compose.yml, docker-compose.yaml, docker-compose.yml)")
	}
	return found, nil
}
