package orchestrator_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/deveshpharswan/stackup/internal/config"
	"github.com/deveshpharswan/stackup/internal/health"
	"github.com/deveshpharswan/stackup/internal/orchestrator"
	"github.com/deveshpharswan/stackup/internal/printer"
	"github.com/stretchr/testify/assert"
)

type alwaysHealthy struct{}

func (a *alwaysHealthy) Check(_ context.Context) error { return nil }

type alwaysFailing struct{}

func (a *alwaysFailing) Check(_ context.Context) error { return fmt.Errorf("service unavailable") }

func TestPreFlight_Valid(t *testing.T) {
	buf := new(bytes.Buffer)
	o := orchestrator.New(printer.New(buf))
	ok := o.PreFlight(
		"../../testdata/.env.valid",
		"../../testdata/.env.example",
		map[string]config.EnvVar{
			"DATABASE_URL": {Type: "url", Required: true},
			"PORT":         {Type: "int", Required: true},
		},
	)
	assert.True(t, ok)
}

func TestPreFlight_Invalid(t *testing.T) {
	buf := new(bytes.Buffer)
	o := orchestrator.New(printer.New(buf))
	ok := o.PreFlight("../../testdata/.env.missing-key", "../../testdata/.env.example", nil)
	assert.False(t, ok)
	assert.Contains(t, buf.String(), "API_KEY")
}

func TestStartTier_AllHealthy(t *testing.T) {
	buf := new(bytes.Buffer)
	o := orchestrator.New(printer.New(buf))
	checkers := map[string]health.Named{
		"postgres": {Checker: &alwaysHealthy{}, Label: "tcp:5432"},
		"redis":    {Checker: &alwaysHealthy{}, Label: "docker"},
	}
	started := []string{}
	startFn := func(_ context.Context, svcs []string) error {
		started = append(started, svcs...)
		return nil
	}
	err := o.StartTier(context.Background(), orchestrator.Tier{"postgres", "redis"}, nil, startFn, checkers, nil)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"postgres", "redis"}, started)
}

func TestStartTier_HealthCheckFails(t *testing.T) {
	buf := new(bytes.Buffer)
	o := orchestrator.New(printer.New(buf))
	checkers := map[string]health.Named{
		"api": {Checker: &alwaysFailing{}, Label: "http"},
	}
	err := o.StartTier(context.Background(), orchestrator.Tier{"api"}, nil, func(_ context.Context, _ []string) error { return nil }, checkers, nil)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api")
}

// mockChecker is a configurable checker with a delay and optional error.
type mockChecker struct {
	delay time.Duration
	err   error
}

func (m *mockChecker) Check(ctx context.Context) error {
	time.Sleep(m.delay)
	return m.err
}

func TestStartTier_ParallelHealthChecks(t *testing.T) {
	buf := new(bytes.Buffer)
	o := orchestrator.New(printer.New(buf))

	checkers := map[string]health.Named{
		"svc-a": {Checker: &mockChecker{delay: 100 * time.Millisecond}, Label: "tcp"},
		"svc-b": {Checker: &mockChecker{delay: 100 * time.Millisecond}, Label: "tcp"},
	}
	startFn := func(_ context.Context, _ []string) error { return nil }

	start := time.Now()
	err := o.StartTier(context.Background(), orchestrator.Tier{"svc-a", "svc-b"}, nil, startFn, checkers, nil)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	// If sequential, would take >=200ms. Parallel should complete in <180ms.
	assert.Less(t, elapsed, 180*time.Millisecond, "health checks should run in parallel, took %v", elapsed)
	assert.Contains(t, buf.String(), "svc-a")
	assert.Contains(t, buf.String(), "svc-b")
	assert.Contains(t, buf.String(), "healthy")
}

func TestStartTier_ParallelWithOneFailure(t *testing.T) {
	buf := new(bytes.Buffer)
	o := orchestrator.New(printer.New(buf))

	checkers := map[string]health.Named{
		"healthy-svc": {Checker: &mockChecker{delay: 50 * time.Millisecond}, Label: "tcp"},
		"failing-svc": {Checker: &mockChecker{delay: 50 * time.Millisecond, err: fmt.Errorf("connection refused")}, Label: "http"},
	}
	startFn := func(_ context.Context, _ []string) error { return nil }

	err := o.StartTier(context.Background(), orchestrator.Tier{"healthy-svc", "failing-svc"}, nil, startFn, checkers, nil)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failing-svc")
	assert.Contains(t, err.Error(), "connection refused")
	// The healthy service should still be printed
	assert.Contains(t, buf.String(), "healthy-svc")
}
