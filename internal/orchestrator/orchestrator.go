package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/stackup-dev/stackup/internal/config"
	"github.com/stackup-dev/stackup/internal/env"
	"github.com/stackup-dev/stackup/internal/health"
	"github.com/stackup-dev/stackup/internal/printer"
)

type Orchestrator struct {
	p *printer.Printer
}

func New(p *printer.Printer) *Orchestrator {
	return &Orchestrator{p: p}
}

func (o *Orchestrator) PreFlight(envFile, exampleFile string, schema map[string]config.EnvVar) bool {
	o.p.Phase("Pre-flight")
	result := env.Validate(envFile, exampleFile, schema)
	if result.Valid() {
		envVars, _ := godotenv.Read(envFile)
		o.p.EnvValid(len(envVars))
		for key, rule := range schema {
			if rule.Type != "" {
				o.p.EnvKeyValid(key, rule.Type)
			}
		}
		return true
	}
	for _, e := range result.Errors {
		o.p.ValidationError(e.Key, e.Message)
	}
	return false
}

func (o *Orchestrator) StartTier(ctx context.Context, tier Tier, deps []string, startFn func(context.Context, []string) error, checkers map[string]health.Named) error {
	label := "Starting tier"
	if len(deps) > 0 {
		label += fmt.Sprintf("  (depends on: %s)", strings.Join(deps, ", "))
	}
	o.p.Phase(label)

	if err := startFn(ctx, tier); err != nil {
		return fmt.Errorf("failed to start tier %v: %w", []string(tier), err)
	}

	for _, svc := range tier {
		named, ok := checkers[svc]
		if !ok {
			continue
		}
		start := time.Now()
		if err := named.Checker.Check(ctx); err != nil {
			o.p.ServiceFailed(svc, err)
			return fmt.Errorf("service %q failed health check: %w", svc, err)
		}
		o.p.ServiceHealthy(svc, named.Label, time.Since(start))
	}
	return nil
}
