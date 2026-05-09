package onboard

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deveshpharswan/stackup/internal/config"
)

func TestNeedsOnboarding_MissingEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if !NeedsOnboarding(envPath) {
		t.Error("expected NeedsOnboarding to return true for missing .env")
	}
}

func TestNeedsOnboarding_ExistingEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	if err := os.WriteFile(envPath, []byte("FOO=bar\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if NeedsOnboarding(envPath) {
		t.Error("expected NeedsOnboarding to return false for existing .env")
	}
}

func TestOnboarder_Run_CreatesEnvFile(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	examplePath := filepath.Join(dir, ".env.example")

	// Write an example file with two keys.
	if err := os.WriteFile(examplePath, []byte("API_KEY=example_key\nDB_HOST=localhost\n"), 0644); err != nil {
		t.Fatal(err)
	}

	schema := map[string]config.EnvVar{
		"API_KEY": {Type: "string", Required: true},
		"DB_HOST": {Type: "string", Required: true},
	}

	// Simulate user input: confirm, then provide values for each key.
	input := "y\nmy_api_key\nmy_db_host\n"
	var out bytes.Buffer
	ob := New(&out, strings.NewReader(input), schema)

	if err := ob.Run(envPath, examplePath); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// Verify .env was created with correct content.
	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("failed to read .env: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "API_KEY=my_api_key") {
		t.Errorf("expected API_KEY=my_api_key in .env, got: %s", content)
	}
	if !strings.Contains(content, "DB_HOST=my_db_host") {
		t.Errorf("expected DB_HOST=my_db_host in .env, got: %s", content)
	}
}

func TestOnboarder_Run_UsesDefaults(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	examplePath := filepath.Join(dir, ".env.example")

	if err := os.WriteFile(examplePath, []byte("PORT=3000\n"), 0644); err != nil {
		t.Fatal(err)
	}

	schema := map[string]config.EnvVar{
		"PORT": {Type: "string", Required: false, Default: "8080"},
	}

	// User confirms, then presses enter (empty) to accept default.
	input := "y\n\n"
	var out bytes.Buffer
	ob := New(&out, strings.NewReader(input), schema)

	if err := ob.Run(envPath, examplePath); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("failed to read .env: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "PORT=8080") {
		t.Errorf("expected PORT=8080 (default) in .env, got: %s", content)
	}
}

func TestOnboarder_Run_Cancelled(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	examplePath := filepath.Join(dir, ".env.example")

	if err := os.WriteFile(examplePath, []byte("FOO=bar\n"), 0644); err != nil {
		t.Fatal(err)
	}

	schema := map[string]config.EnvVar{}

	// User declines.
	input := "n\n"
	var out bytes.Buffer
	ob := New(&out, strings.NewReader(input), schema)

	err := ob.Run(envPath, examplePath)
	if err == nil {
		t.Fatal("expected error when user cancels, got nil")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("expected cancel error, got: %v", err)
	}

	// Verify .env was NOT created.
	if _, statErr := os.Stat(envPath); !os.IsNotExist(statErr) {
		t.Error("expected .env to not exist after cancellation")
	}
}
