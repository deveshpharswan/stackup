package e2e_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestEnvGate_ValidatePassesWithValidEnv verifies validate exits 0 when .env matches schema.
func TestEnvGate_ValidatePassesWithValidEnv(t *testing.T) {
	dir := copyFixture(t, "with-env")
	// Write a .env that satisfies the schema (APP_PORT required)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("APP_PORT=8080\n"), 0644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	result := runCLI(t, dir, "validate")
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d\nstdout: %s\nstderr: %s",
			result.ExitCode, result.Stdout, result.Stderr)
	}
}

// TestEnvGate_ValidateFailsWithMissingKey verifies validate exits 1 when a required key is absent.
func TestEnvGate_ValidateFailsWithMissingKey(t *testing.T) {
	dir := copyFixture(t, "with-env")
	// Write .env without APP_PORT
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("OTHER=value\n"), 0644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	result := runCLI(t, dir, "validate")
	if result.ExitCode == 0 {
		t.Fatal("expected non-zero exit when required key is missing")
	}
}

// TestEnvGate_ValidateJSONReturnsValidJSON verifies --output json is parseable JSON.
func TestEnvGate_ValidateJSONReturnsValidJSON(t *testing.T) {
	dir := copyFixture(t, "with-env")
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte("APP_PORT=8080\n"), 0644); err != nil {
		t.Fatalf("write .env: %v", err)
	}

	result := runCLI(t, dir, "validate", "--output", "json")
	if result.ExitCode != 0 {
		t.Fatalf("expected exit 0 for json validate, got %d\nstdout: %s\nstderr: %s",
			result.ExitCode, result.Stdout, result.Stderr)
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(result.Stdout), &out); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nraw: %s", err, result.Stdout)
	}
	if valid, ok := out["valid"].(bool); !ok || !valid {
		t.Errorf("expected {\"valid\": true}, got: %s", result.Stdout)
	}
}

// TestEnvGate_NoSchemaNoExampleSkipsValidation verifies that validate completes
// without panicking when there is no schema and no .env.example.
func TestEnvGate_NoSchemaNoExampleSkipsValidation(t *testing.T) {
	dir := copyFixture(t, "simple-stack") // has no stackup.yml, no .env.example

	result := runCLI(t, dir, "validate")
	// Any exit code <= 1 is acceptable: 0 means validation passed (nothing to check),
	// 1 means could not read .env (also acceptable with no schema).
	// Exit 2+ indicates a crash or unexpected internal error.
	if result.ExitCode > 1 {
		t.Fatalf("expected exit 0 or 1, got %d\nstdout: %s\nstderr: %s",
			result.ExitCode, result.Stdout, result.Stderr)
	}
}
