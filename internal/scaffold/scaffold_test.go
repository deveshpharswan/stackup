package scaffold_test

import (
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
