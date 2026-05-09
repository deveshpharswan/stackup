package scaffold_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stackup-dev/stackup/internal/scaffold"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerate(t *testing.T) {
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
	def := scaffold.DetectHealthDefault("mycompany/custom-app:latest")
	assert.Nil(t, def)
}
