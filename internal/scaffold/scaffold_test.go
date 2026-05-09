package scaffold_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deveshpharswan/stackup/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerate(t *testing.T) {
	t.Parallel()
	out, err := scaffold.Generate("../../testdata/docker-compose.yml", "../../testdata/.env.example")
	require.NoError(t, err)
	assert.Contains(t, out, "postgres:")
	assert.Contains(t, out, "redis:")
	assert.Contains(t, out, "api:")
	assert.Contains(t, out, "DATABASE_URL")
	assert.Contains(t, out, "PORT")
	assert.True(t, strings.HasPrefix(strings.TrimSpace(out), "version:"))
}

func TestGenerate_SmartImageDetection(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	composePath := filepath.Join(dir, "docker-compose.yml")
	os.WriteFile(composePath, []byte(`services:
  db:
    image: postgres:15
  cache:
    image: redis:7-alpine
  api:
    build: ./api
`), 0644)
	examplePath := filepath.Join(dir, ".env.example")
	os.WriteFile(examplePath, []byte(""), 0644)

	output, err := scaffold.Generate(composePath, examplePath)
	assert.NoError(t, err)
	assert.Contains(t, output, "port: 5432") // postgres detected
	assert.Contains(t, output, "port: 6379") // redis detected
	assert.Contains(t, output, "# TODO")     // api has no known image
}

func TestDetectHealthDefault_KnownImages(t *testing.T) {
	t.Parallel()
	tests := []struct {
		image string
		port  int
	}{
		{"postgres:15", 5432},
		{"redis:7-alpine", 6379},
		{"mysql:8.0", 3306},
		{"bitnami/mongodb:6.0", 27017},
	}
	for _, tc := range tests {
		def := scaffold.DetectHealthDefault(tc.image)
		assert.NotNil(t, def, "should detect %s", tc.image)
		assert.Equal(t, tc.port, def.Port)
	}
}

func TestDetectHealthDefault_UnknownImage(t *testing.T) {
	t.Parallel()
	def := scaffold.DetectHealthDefault("mycompany/custom-app:latest")
	assert.Nil(t, def)
}

func TestParseServicesRich_Conditions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	composePath := filepath.Join(dir, "docker-compose.yml")
	os.WriteFile(composePath, []byte(`services:
  postgres:
    image: postgres:15
  redis:
    image: redis:7
  api:
    build: ./api
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_started
  worker:
    build: ./worker
    depends_on:
      - postgres
`), 0644)

	deps, err := scaffold.ParseServicesRich(composePath)
	require.NoError(t, err)

	// api should have two deps with conditions
	apiDeps := deps["api"]
	require.Len(t, apiDeps, 2)
	conditions := map[string]string{}
	for _, d := range apiDeps {
		conditions[d.Service] = d.Condition
	}
	assert.Equal(t, "service_healthy", conditions["postgres"])
	assert.Equal(t, "service_started", conditions["redis"])

	// worker uses list syntax — defaults to service_started
	workerDeps := deps["worker"]
	require.Len(t, workerDeps, 1)
	assert.Equal(t, "postgres", workerDeps[0].Service)
	assert.Equal(t, "service_started", workerDeps[0].Condition)

	// postgres and redis have no deps
	assert.Empty(t, deps["postgres"])
	assert.Empty(t, deps["redis"])
}
