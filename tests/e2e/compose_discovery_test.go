package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestComposeDiscovery_FindsComposeYAML verifies that stackup up finds compose.yaml.
func TestComposeDiscovery_FindsComposeYAML(t *testing.T) {
	skipIfNoDocker(t)
	dir := copyFixture(t, "simple-stack")
	t.Cleanup(func() { cleanupContainers(t, dir) })

	result := runCLI(t, dir, "up")
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d\nstdout: %s\nstderr: %s",
			result.ExitCode, result.Stdout, result.Stderr)
	}
}

// TestComposeDiscovery_FindsComposeDotYML verifies that stackup up finds compose.yml.
func TestComposeDiscovery_FindsComposeDotYML(t *testing.T) {
	skipIfNoDocker(t)
	dir := copyFixture(t, "simple-stack")
	t.Cleanup(func() { cleanupContainers(t, dir) })

	if err := os.Rename(
		filepath.Join(dir, "compose.yaml"),
		filepath.Join(dir, "compose.yml"),
	); err != nil {
		t.Fatalf("rename: %v", err)
	}

	result := runCLI(t, dir, "up")
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d\nstdout: %s\nstderr: %s",
			result.ExitCode, result.Stdout, result.Stderr)
	}
}

// TestComposeDiscovery_FindsDockerComposeYAML verifies docker-compose.yaml is found.
func TestComposeDiscovery_FindsDockerComposeYAML(t *testing.T) {
	skipIfNoDocker(t)
	dir := copyFixture(t, "simple-stack")
	t.Cleanup(func() { cleanupContainers(t, dir) })

	if err := os.Rename(
		filepath.Join(dir, "compose.yaml"),
		filepath.Join(dir, "docker-compose.yaml"),
	); err != nil {
		t.Fatalf("rename: %v", err)
	}

	result := runCLI(t, dir, "up")
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d\nstdout: %s\nstderr: %s",
			result.ExitCode, result.Stdout, result.Stderr)
	}
}

// TestComposeDiscovery_FlagOverride verifies that -f overrides auto-discovery.
// Uses stackup init (no Docker needed) to confirm -f is respected.
func TestComposeDiscovery_FlagOverride(t *testing.T) {
	dir := t.TempDir() // no compose file here

	// Get absolute path to the compose file so it resolves correctly
	// regardless of the binary's working directory.
	composeSrc, err := filepath.Abs(filepath.Join("testdata", "tcp-health", "compose.yaml"))
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}

	result := runCLI(t, dir, "init", "-f", composeSrc)
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0 for init with -f, got %d\nstdout: %s\nstderr: %s",
			result.ExitCode, result.Stdout, result.Stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "stackup.yml")); err != nil {
		t.Fatalf("expected stackup.yml to be created, got: %v", err)
	}
}

// TestComposeDiscovery_MissingFileErrors verifies that a clear error is returned
// when no compose file exists.
func TestComposeDiscovery_MissingFileErrors(t *testing.T) {
	dir := t.TempDir() // empty dir

	result := runCLI(t, dir, "up")
	if result.ExitCode == 0 {
		t.Fatal("expected non-zero exit when no compose file found")
	}
	combined := result.Stdout + result.Stderr
	if !strings.Contains(combined, "no compose file found") {
		t.Errorf("expected 'no compose file found' in output, got:\n%s", combined)
	}
}
