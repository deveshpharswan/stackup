package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHealthHTTP_PassesWhenNginxIsUp verifies the HTTP health check passes for a live nginx.
func TestHealthHTTP_PassesWhenNginxIsUp(t *testing.T) {
	skipIfNoDocker(t)
	dir := copyFixture(t, "http-health")
	t.Cleanup(func() { cleanupContainers(t, dir) })

	result := runCLI(t, dir, "up")
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d\nstdout: %s\nstderr: %s",
			result.ExitCode, result.Stdout, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "healthy") {
		t.Errorf("expected 'healthy' in stdout, got:\n%s", result.Stdout)
	}
}

// TestHealthHTTP_FailsOnUnreachablePort verifies the HTTP health check times out
// when the configured port is unreachable (nothing running on 19999).
func TestHealthHTTP_FailsOnUnreachablePort(t *testing.T) {
	skipIfNoDocker(t)
	dir := copyFixture(t, "http-health")
	t.Cleanup(func() { cleanupContainers(t, dir) })

	// Override stackup.yml to point at a port nothing listens on.
	badStackup := "version: \"1\"\nservices:\n  web:\n    health:\n      type: http\n      url: http://localhost:19999/\n      timeout: 5s\n      interval: 1s\n"
	if err := os.WriteFile(filepath.Join(dir, "stackup.yml"), []byte(badStackup), 0644); err != nil {
		t.Fatalf("write stackup.yml: %v", err)
	}

	result := runCLI(t, dir, "up")
	if result.ExitCode == 0 {
		t.Fatalf("expected non-zero exit when HTTP health check is unreachable\nstdout: %s\nstderr: %s",
			result.Stdout, result.Stderr)
	}
}
