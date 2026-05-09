//go:build e2e

package e2e_test

import (
	"bytes"
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/deveshpharswan/stackup/internal/config"
	"github.com/deveshpharswan/stackup/internal/health"
	"github.com/deveshpharswan/stackup/internal/orchestrator"
	"github.com/deveshpharswan/stackup/internal/printer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUp_RedisStack(t *testing.T) {
	// Check docker compose is available
	if err := exec.Command("docker", "compose", "version").Run(); err != nil {
		t.Skip("docker compose not available")
	}

	ctx := context.Background()

	// Cleanup on exit
	t.Cleanup(func() {
		cmd := exec.Command("docker", "compose", "-f", "fixtures/docker-compose.yml", "down", "--remove-orphans")
		cmd.Run()
	})

	buf := new(bytes.Buffer)
	p := printer.New(buf)
	cfg, err := config.Load("fixtures/stackup.yml")
	require.NoError(t, err)

	o := orchestrator.New(p)
	ok := o.PreFlight("fixtures/.env", "fixtures/.env.example", cfg.Env.Schema)
	assert.True(t, ok)

	// Start redis via docker compose
	cmd := exec.Command("docker", "compose", "-f", "fixtures/docker-compose.yml", "up", "-d")
	require.NoError(t, cmd.Run())

	// Health check
	checkers := map[string]health.Named{
		"redis": {
			Checker: health.NewTCPChecker("localhost", "16379", 30*time.Second, time.Second),
			Label:   "tcp:16379",
		},
	}

	startFn := func(_ context.Context, _ []string) error { return nil }
	err = o.StartTier(ctx, orchestrator.Tier{"redis"}, nil, startFn, checkers)
	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "redis")
	assert.Contains(t, buf.String(), "healthy")
}
