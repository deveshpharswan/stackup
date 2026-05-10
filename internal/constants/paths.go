// Package constants defines default file paths used throughout Stackup.
package constants

import (
	"os"
	"path/filepath"
)

const (
	// DefaultConfigFile is the stackup configuration filename.
	DefaultConfigFile = "stackup.yml"
	// DefaultComposeFile is kept for backward compatibility in tests.
	// New code should call FindComposeFile instead.
	DefaultComposeFile = "docker-compose.yml"
	// DefaultEnvFile is the environment variables filename.
	DefaultEnvFile = ".env"
	// DefaultExampleFile is the example env filename used for onboarding and drift detection.
	DefaultExampleFile = ".env.example"
)

// composeFileNames is the ordered list of compose file names to search,
// matching Docker CLI's own discovery order.
var composeFileNames = []string{
	"compose.yaml",
	"compose.yml",
	"docker-compose.yaml",
	"docker-compose.yml",
}

// FindComposeFile returns the path to the first compose file found in dir,
// checking names in Docker CLI precedence order. Returns "" if none found.
func FindComposeFile(dir string) string {
	for _, name := range composeFileNames {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
