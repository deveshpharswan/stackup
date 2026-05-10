package e2e_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// CLIResult holds the captured output of a CLI invocation.
type CLIResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// runCLI invokes the stackup binary in dir with the given arguments.
// It captures stdout and stderr and returns the exit code.
func runCLI(t *testing.T, dir string, args ...string) CLIResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, stackupBin, args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			code = exitErr.ExitCode()
		} else {
			code = 1
		}
	}
	return CLIResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: code,
	}
}

// copyFixture copies testdata/<name> into a new temp dir and returns the path.
// The test's cleanup will remove the temp dir automatically via t.TempDir.
func copyFixture(t *testing.T, name string) string {
	t.Helper()
	src := filepath.Join("testdata", name)
	dst := t.TempDir()
	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyFixture %s: %v", name, err)
	}
	return dst
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0644)
	})
}

// skipIfNoDocker skips the test if Docker is not available.
func skipIfNoDocker(t *testing.T) {
	t.Helper()
	if !hasDocker {
		t.Skip("docker compose not available — skipping container test")
	}
}

// cleanupContainers runs `stackup down` in dir as cleanup.
func cleanupContainers(t *testing.T, dir string) {
	t.Helper()
	runCLI(t, dir, "down")
}

// writeTestFile writes content to name inside dir.
func writeTestFile(t *testing.T, dir, name, content string) error {
	t.Helper()
	return os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
}
