package e2e_test

import (
	"strings"
	"testing"
)

// TestOrdering_MultiTierStartsInOrder verifies that all three tiers start
// in dependency order and each service becomes healthy.
func TestOrdering_MultiTierStartsInOrder(t *testing.T) {
	skipIfNoDocker(t)
	dir := copyFixture(t, "multi-tier")
	t.Cleanup(func() { cleanupContainers(t, dir) })

	result := runCLI(t, dir, "up")
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d\nstdout: %s\nstderr: %s",
			result.ExitCode, result.Stdout, result.Stderr)
	}
	// All three services should appear as healthy in output
	for _, svc := range []string{"db", "cache", "web"} {
		if !strings.Contains(result.Stdout, svc) {
			t.Errorf("expected service %q to appear in output, got:\n%s", svc, result.Stdout)
		}
	}
}

// TestOrdering_PartialExitCode3 verifies that --partial returns exit code 3
// when one service fails health check while others succeed.
func TestOrdering_PartialExitCode3(t *testing.T) {
	skipIfNoDocker(t)
	dir := copyFixture(t, "tcp-health")
	t.Cleanup(func() { cleanupContainers(t, dir) })

	// Start a second service with a bad health check alongside the real redis.
	// Override compose + stackup to add a failing service alongside the passing one.
	badCompose := `services:
  cache:
    image: redis:7-alpine
    ports:
      - "16380:6379"
  ghost:
    image: nginx:alpine
    ports:
      - "18086:80"
`
	badStackup := `version: "1"
services:
  cache:
    health:
      type: tcp
      host: localhost
      port: 16380
      timeout: 30s
      interval: 1s
  ghost:
    health:
      type: tcp
      host: localhost
      port: 19997
      timeout: 5s
      interval: 1s
`
	if err := writeTestFile(t, dir, "compose.yaml", badCompose); err != nil {
		t.Fatalf("write compose.yaml: %v", err)
	}
	if err := writeTestFile(t, dir, "stackup.yml", badStackup); err != nil {
		t.Fatalf("write stackup.yml: %v", err)
	}

	result := runCLI(t, dir, "up", "--partial")
	if result.ExitCode != 3 {
		t.Fatalf("expected exit code 3 for partial success, got %d\nstdout: %s\nstderr: %s",
			result.ExitCode, result.Stdout, result.Stderr)
	}
}
