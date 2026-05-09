package doctor

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckEnvDrift_DetectsMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	envFile := filepath.Join(dir, ".env")
	exampleFile := filepath.Join(dir, ".env.example")

	require.NoError(t, os.WriteFile(envFile, []byte("DB_HOST=localhost\n"), 0644))
	require.NoError(t, os.WriteFile(exampleFile, []byte("DB_HOST=localhost\nDB_PORT=5432\nSECRET_KEY=changeme\n"), 0644))

	opts := &Options{
		EnvFile:     envFile,
		ExampleFile: exampleFile,
	}

	findings := CheckEnvDrift(context.Background(), opts)

	require.Len(t, findings, 1)
	assert.Equal(t, SeverityWarning, findings[0].Severity)
	assert.Contains(t, findings[0].Title, "drift")
	// Must mention both missing keys.
	assert.Contains(t, findings[0].Detail, "DB_PORT")
	assert.Contains(t, findings[0].Detail, "SECRET_KEY")
}

func TestCheckEnvDrift_NoDrift(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	envFile := filepath.Join(dir, ".env")
	exampleFile := filepath.Join(dir, ".env.example")

	content := "DB_HOST=localhost\nDB_PORT=5432\n"
	require.NoError(t, os.WriteFile(envFile, []byte(content), 0644))
	require.NoError(t, os.WriteFile(exampleFile, []byte(content), 0644))

	opts := &Options{
		EnvFile:     envFile,
		ExampleFile: exampleFile,
	}

	findings := CheckEnvDrift(context.Background(), opts)
	assert.Empty(t, findings)
}

func TestPrintFindings_FormatsCorrectly(t *testing.T) {
	t.Parallel()
	findings := []Finding{
		{Severity: SeverityError, Title: "Port 5432 in use", Detail: "postgres", Fix: "lsof -i :5432", Service: "postgres"},
		{Severity: SeverityWarning, Title: "Env drift detected", Detail: "missing KEY_A"},
		{Severity: SeverityOK, Title: "Service redis is running", Service: "redis"},
	}

	var buf bytes.Buffer
	PrintFindings(&buf, findings)
	output := buf.String()

	// Check icons present.
	assert.Contains(t, output, "\xe2\x9c\x97") // ✗ (UTF-8 for ✗)
	assert.Contains(t, output, "\xe2\x9c\x93") // ✓ (UTF-8 for ✓)
	assert.Contains(t, output, "!")

	// Check summary line.
	assert.Contains(t, output, "1 error(s)")
	assert.Contains(t, output, "1 warning(s)")
	assert.Contains(t, output, "1 ok")

	// Check service labels.
	assert.Contains(t, output, "[postgres]")
	assert.Contains(t, output, "[redis]")
}

func TestCheckLocalhostMisuse_DetectsPattern(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	envFile := filepath.Join(dir, ".env")
	composeFile := filepath.Join(dir, "docker-compose.yml")

	envContent := "DATABASE_URL=postgresql://user:pass@localhost:5432/mydb\nREDIS_URL=redis://127.0.0.1:6379\n"
	require.NoError(t, os.WriteFile(envFile, []byte(envContent), 0644))

	composeContent := `services:
  postgres:
    image: postgres:15
  redis:
    image: redis:7
`
	require.NoError(t, os.WriteFile(composeFile, []byte(composeContent), 0644))

	opts := &Options{
		EnvFile:     envFile,
		ComposeFile: composeFile,
	}

	findings := CheckLocalhostMisuse(context.Background(), opts)

	// Should detect both postgres and redis localhost misuse.
	require.GreaterOrEqual(t, len(findings), 2)

	var titles []string
	for _, f := range findings {
		assert.Equal(t, SeverityWarning, f.Severity)
		titles = append(titles, f.Title)
	}

	joined := strings.Join(titles, " | ")
	assert.Contains(t, joined, "DATABASE_URL")
	assert.Contains(t, joined, "REDIS_URL")
}

func TestCheckLocalhostMisuse_NoMatch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	envFile := filepath.Join(dir, ".env")
	composeFile := filepath.Join(dir, "docker-compose.yml")

	// Use service name instead of localhost — no warnings expected.
	envContent := "DATABASE_URL=postgresql://user:pass@postgres:5432/mydb\n"
	require.NoError(t, os.WriteFile(envFile, []byte(envContent), 0644))

	composeContent := `services:
  postgres:
    image: postgres:15
`
	require.NoError(t, os.WriteFile(composeFile, []byte(composeContent), 0644))

	opts := &Options{
		EnvFile:     envFile,
		ComposeFile: composeFile,
	}

	findings := CheckLocalhostMisuse(context.Background(), opts)
	assert.Empty(t, findings)
}

func TestGuessServicePort(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		want string
	}{
		{"postgres", "5432"},
		{"redis", "6379"},
		{"mysql", "3306"},
		{"mongo", "27017"},
		{"unknown-service", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, guessServicePort(tt.name))
		})
	}
}
