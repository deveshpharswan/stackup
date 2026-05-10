package e2e_test

import (
	"strings"
	"testing"
)

// TestHealthLog_PassesWhenPatternAppearsInLogs verifies the log health check passes
// when the pattern "ready for start up" appears in nginx startup logs.
func TestHealthLog_PassesWhenPatternAppearsInLogs(t *testing.T) {
	skipIfNoDocker(t)
	dir := copyFixture(t, "log-health")
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

// TestHealthLog_TimesOutWhenPatternNeverAppears verifies the log health check fails
// when the required pattern is never logged.
func TestHealthLog_TimesOutWhenPatternNeverAppears(t *testing.T) {
	skipIfNoDocker(t)
	dir := copyFixture(t, "log-health")
	t.Cleanup(func() { cleanupContainers(t, dir) })

	// Override pattern to something that nginx never logs.
	badStackup := `version: "1"
services:
  web:
    health:
      type: log
      pattern: "PATTERN_THAT_NEVER_APPEARS_xyzzy_12345"
      timeout: 8s
      interval: 1s
`
	if err := writeTestFile(t, dir, "stackup.yml", badStackup); err != nil {
		t.Fatalf("write stackup.yml: %v", err)
	}

	result := runCLI(t, dir, "up")
	if result.ExitCode == 0 {
		t.Fatalf("expected non-zero exit when log pattern never appears\nstdout: %s\nstderr: %s",
			result.Stdout, result.Stderr)
	}
}
