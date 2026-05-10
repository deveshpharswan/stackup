package e2e_test

import (
	"strings"
	"testing"
)

// TestFlags_Only starts only the backend profile service using --only.
func TestFlags_Only(t *testing.T) {
	skipIfNoDocker(t)
	dir := copyFixture(t, "profiles")
	t.Cleanup(func() { cleanupContainers(t, dir) })

	// Start only the cache service (redis). --only resolves dependencies so
	// web is not started (it has no compose-level depends_on on cache here).
	result := runCLI(t, dir, "up", "--only", "cache")
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0 for --only cache, got %d\nstdout: %s\nstderr: %s",
			result.ExitCode, result.Stdout, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "cache") {
		t.Errorf("expected 'cache' in stdout, got:\n%s", result.Stdout)
	}
}

// TestFlags_Profile starts only the backend profile services.
func TestFlags_Profile(t *testing.T) {
	skipIfNoDocker(t)
	dir := copyFixture(t, "profiles")
	t.Cleanup(func() { cleanupContainers(t, dir) })

	result := runCLI(t, dir, "up", "--profile", "backend")
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0 for --profile backend, got %d\nstdout: %s\nstderr: %s",
			result.ExitCode, result.Stdout, result.Stderr)
	}
}

// TestFlags_OnlyUnknownServiceErrors verifies --only with an unknown service name
// returns a clear error.
func TestFlags_OnlyUnknownServiceErrors(t *testing.T) {
	dir := copyFixture(t, "simple-stack")

	result := runCLI(t, dir, "up", "--only", "nonexistent-service-xyz")
	if result.ExitCode == 0 {
		t.Fatal("expected non-zero exit for --only with unknown service")
	}
	combined := result.Stdout + result.Stderr
	if !strings.Contains(combined, "nonexistent-service-xyz") {
		t.Errorf("expected service name in error output, got:\n%s", combined)
	}
}
