package config_test

import (
	"os"
	"testing"

	"github.com/deveshpharswan/stackup/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_ValidFile(t *testing.T) {
	t.Parallel()
	cfg, err := config.Load("../../testdata/valid-stackup.yml")
	require.NoError(t, err)
	assert.Equal(t, "1", cfg.Version)

	dbURL := cfg.Env.Schema["DATABASE_URL"]
	assert.Equal(t, "url", dbURL.Type)
	assert.True(t, dbURL.Required)

	port := cfg.Env.Schema["PORT"]
	assert.Equal(t, "int", port.Type)
	assert.Equal(t, "3000", port.Default)

	pg := cfg.Services["postgres"]
	require.NotNil(t, pg.Health)
	assert.Equal(t, "tcp", pg.Health.Type)
	assert.Equal(t, "localhost", pg.Health.Host)
	assert.Equal(t, 5432, pg.Health.Port)
	assert.Equal(t, "30s", pg.Health.Timeout)

	api := cfg.Services["api"]
	assert.Equal(t, "http", api.Health.Type)
	assert.Equal(t, "http://localhost:8080/health", api.Health.URL)

	redis := cfg.Services["redis"]
	assert.Equal(t, "docker", redis.Health.Type)

	seed := cfg.Commands["seed"]
	assert.Equal(t, "api", seed.Service)
	assert.Equal(t, "npm run db:seed", seed.Run)
}

func TestLoad_MissingFile(t *testing.T) {
	t.Parallel()
	_, err := config.Load("nonexistent.yml")
	assert.Error(t, err)
}

func TestLoadOrEmpty_MissingFile(t *testing.T) {
	t.Parallel()
	cfg, err := config.LoadOrEmpty("nonexistent.yml")
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Empty(t, cfg.Services)
	assert.Empty(t, cfg.Commands)
}

func TestLoadOrEmpty_MalformedYAML(t *testing.T) {
	t.Parallel()
	f, err := os.CreateTemp("", "stackup-*.yml")
	require.NoError(t, err)
	defer os.Remove(f.Name())
	_, _ = f.WriteString(": invalid: yaml: [\n")
	f.Close()

	_, err = config.LoadOrEmpty(f.Name())
	assert.Error(t, err)
}
