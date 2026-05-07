package orchestrator_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/stackup-dev/stackup/internal/config"
	"github.com/stackup-dev/stackup/internal/health"
	"github.com/stackup-dev/stackup/internal/orchestrator"
	"github.com/stackup-dev/stackup/internal/printer"
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
	err := o.StartTier(context.Background(), orchestrator.Tier{"postgres", "redis"}, nil, startFn, checkers)
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"postgres", "redis"}, started)
}

func TestStartTier_HealthCheckFails(t *testing.T) {
	buf := new(bytes.Buffer)
	o := orchestrator.New(printer.New(buf))
	checkers := map[string]health.Named{
		"api": {Checker: &alwaysFailing{}, Label: "http"},
	}
	err := o.StartTier(context.Background(), orchestrator.Tier{"api"}, nil, func(_ context.Context, _ []string) error { return nil }, checkers)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "api")
}
