package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHealthTCP_PassesWhenRedisIsUp verifies the TCP health check passes for a live redis.
func TestHealthTCP_PassesWhenRedisIsUp(t *testing.T) {
	skipIfNoDocker(t)
	dir := copyFixture(t, "tcp-health")
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

// TestHealthTCP_FailsOnClosedPort verifies the TCP health check times out
// when nothing listens on the configured port.
func TestHealthTCP_FailsOnClosedPort(t *testing.T) {
	skipIfNoDocker(t)
	dir := copyFixture(t, "tcp-health")
	t.Cleanup(func() { cleanupContainers(t, dir) })

	// Override stackup.yml to point at a port nothing listens on.
	badStackup := "version: \"1\"\nservices:\n  cache:\n    health:\n      type: tcp\n      host: localhost\n      port: 19998\n      timeout: 5s\n      interval: 1s\n"
	if err := os.WriteFile(filepath.Join(dir, "stackup.yml"), []byte(badStackup), 0644); err != nil {
		t.Fatalf("write stackup.yml: %v", err)
	}

	result := runCLI(t, dir, "up")
	if result.ExitCode == 0 {
		t.Fatalf("expected non-zero exit when TCP health check fails\nstdout: %s\nstderr: %s",
			result.Stdout, result.Stderr)
	}
}
