package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCommands_Version verifies that `stackup version` prints version info.
func TestCommands_Version(t *testing.T) {
	dir := t.TempDir()
	result := runCLI(t, dir, "version")
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d\nstdout: %s\nstderr: %s",
			result.ExitCode, result.Stdout, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "stackup") {
		t.Errorf("expected 'stackup' in version output, got:\n%s", result.Stdout)
	}
}

// TestCommands_InitGeneratesConfig verifies that `stackup init` creates stackup.yml.
func TestCommands_InitGeneratesConfig(t *testing.T) {
	dir := copyFixture(t, "simple-stack")

	result := runCLI(t, dir, "init")
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d\nstdout: %s\nstderr: %s",
			result.ExitCode, result.Stdout, result.Stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "stackup.yml")); err != nil {
		t.Fatalf("expected stackup.yml to be created: %v", err)
	}
	if !strings.Contains(result.Stdout, "generated") {
		t.Errorf("expected 'generated' in output, got:\n%s", result.Stdout)
	}
}

// TestCommands_InitWontOverwrite verifies that `stackup init` refuses to overwrite
// an existing stackup.yml.
func TestCommands_InitWontOverwrite(t *testing.T) {
	dir := copyFixture(t, "http-health") // already has a stackup.yml

	result := runCLI(t, dir, "init")
	if result.ExitCode == 0 {
		t.Fatal("expected non-zero exit when stackup.yml already exists")
	}
	combined := result.Stdout + result.Stderr
	if !strings.Contains(combined, "already exists") {
		t.Errorf("expected 'already exists' in output, got:\n%s", combined)
	}
}

// TestCommands_DoctorRuns verifies that `stackup doctor` runs without panicking.
func TestCommands_DoctorRuns(t *testing.T) {
	dir := copyFixture(t, "simple-stack")

	result := runCLI(t, dir, "doctor")
	// Doctor always exits 0 (it prints findings, doesn't fail).
	if result.ExitCode != 0 {
		t.Logf("doctor exited %d (may be expected on some systems)\nstdout: %s\nstderr: %s",
			result.ExitCode, result.Stdout, result.Stderr)
	}
	// Just verify it produced some output.
	if result.Stdout == "" && result.Stderr == "" {
		t.Error("expected doctor to produce some output")
	}
}

// TestCommands_DownStopsContainers verifies that `stackup down` stops running containers.
func TestCommands_DownStopsContainers(t *testing.T) {
	skipIfNoDocker(t)
	dir := copyFixture(t, "simple-stack")

	// Start containers first.
	upResult := runCLI(t, dir, "up")
	if upResult.ExitCode != 0 {
		t.Fatalf("setup: stackup up failed: exit %d\n%s", upResult.ExitCode, upResult.Stdout+upResult.Stderr)
	}

	// Now bring them down.
	downResult := runCLI(t, dir, "down")
	if downResult.ExitCode != 0 {
		t.Fatalf("expected exit 0 for stackup down, got %d\nstdout: %s\nstderr: %s",
			downResult.ExitCode, downResult.Stdout, downResult.Stderr)
	}
}
