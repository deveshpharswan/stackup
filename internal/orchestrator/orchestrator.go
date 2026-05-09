// Package orchestrator coordinates health-gated service startup in tiered order.
package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/deveshpharswan/stackup/internal/config"
	"github.com/deveshpharswan/stackup/internal/env"
	"github.com/deveshpharswan/stackup/internal/health"
	"github.com/deveshpharswan/stackup/internal/printer"
)

// LogFetcher abstracts container log retrieval for failure diagnostics.
type LogFetcher interface {
	TailLogs(ctx context.Context, containerID string, lines int, w io.Writer) error
	ContainerIDByName(serviceName string) (string, error)
}

// Orchestrator manages the startup sequence and health checking of services.
type Orchestrator struct {
	p *printer.Printer
}

// New creates an Orchestrator that reports progress via the given printer.
func New(p *printer.Printer) *Orchestrator {
	return &Orchestrator{p: p}
}

// PreFlight validates environment variables and injects schema defaults.
// Returns true if validation passes, along with a map of injected default values.
func (o *Orchestrator) PreFlight(envFile, exampleFile string, schema map[string]config.EnvVar) (bool, map[string]string) {
	o.p.Phase("Pre-flight")
	result, injected := env.ValidateWithDefaults(envFile, exampleFile, schema)

	for key, val := range injected {
		o.p.EnvDefault(key, val)
	}

	if result.Valid() {
		envVars, _ := godotenv.Read(envFile)
		o.p.EnvValid(len(envVars) + len(injected))
		for key, rule := range schema {
			if rule.Type != "" {
				o.p.EnvKeyValid(key, rule.Type)
			}
		}
		return true, injected
	}
	for _, e := range result.Errors {
		o.p.ValidationError(e.Key, e.Message)
	}
	return false, nil
}

// StartTier starts all services in a tier and waits for their health checks to pass.
// Health checks run in parallel. On failure, it surfaces container logs and suggests fixes.
func (o *Orchestrator) StartTier(ctx context.Context, tier Tier, deps []string, startFn func(context.Context, []string) error, checkers map[string]health.Named, fetcher LogFetcher) error {
	failed, firstErr, err := o.startTierInternal(ctx, tier, deps, startFn, checkers, fetcher)
	if err != nil {
		return err
	}
	if len(failed) > 0 {
		return fmt.Errorf("service %q failed health check: %w", failed[0], firstErr)
	}
	return nil
}

// StartTierPartial starts all services in a tier and returns the names of failed services
// instead of stopping on first failure. Returns a non-nil error only for startup failures
// (not health check failures).
func (o *Orchestrator) StartTierPartial(ctx context.Context, tier Tier, deps []string, startFn func(context.Context, []string) error, checkers map[string]health.Named, fetcher LogFetcher) ([]string, error) {
	failed, _, err := o.startTierInternal(ctx, tier, deps, startFn, checkers, fetcher)
	return failed, err
}

func (o *Orchestrator) startTierInternal(ctx context.Context, tier Tier, deps []string, startFn func(context.Context, []string) error, checkers map[string]health.Named, fetcher LogFetcher) ([]string, error, error) {
	label := "Starting tier"
	if len(deps) > 0 {
		label += fmt.Sprintf("  (depends on: %s)", strings.Join(deps, ", "))
	}
	o.p.Phase(label)

	if err := startFn(ctx, tier); err != nil {
		return nil, nil, fmt.Errorf("failed to start tier %v: %w", []string(tier), err)
	}

	// Determine which services have health checks.
	type checkTarget struct {
		svc   string
		named health.Named
	}
	var targets []checkTarget
	for _, svc := range tier {
		if named, ok := checkers[svc]; ok {
			targets = append(targets, checkTarget{svc: svc, named: named})
		}
	}
	if len(targets) == 0 {
		return nil, nil, nil
	}

	// Run health checks in parallel.
	type checkResult struct {
		svc     string
		label   string
		elapsed time.Duration
		err     error
	}

	results := make(chan checkResult, len(targets))
	var wg sync.WaitGroup
	wg.Add(len(targets))

	for _, t := range targets {
		go func(svc string, named health.Named) {
			defer wg.Done()
			start := time.Now()
			err := named.Checker.Check(ctx)
			results <- checkResult{
				svc:     svc,
				label:   named.Label,
				elapsed: time.Since(start),
				err:     err,
			}
		}(t.svc, t.named)
	}

	// Close channel once all goroutines complete.
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results, printing healthy services as they arrive.
	var failures []checkResult
	for r := range results {
		if r.err != nil {
			failures = append(failures, r)
		} else {
			o.p.ServiceHealthy(r.svc, r.label, r.elapsed)
		}
	}

	var failedNames []string
	var firstErr error
	for _, f := range failures {
		o.p.ServiceFailed(f.svc, f.err)
		o.surfaceLogs(ctx, f.svc, fetcher)
		failedNames = append(failedNames, f.svc)
		if firstErr == nil {
			firstErr = f.err
		}
	}

	if len(failedNames) > 0 {
		o.p.Hint("stackup doctor", "stackup logs <service>")
	}

	return failedNames, firstErr, nil
}

func (o *Orchestrator) surfaceLogs(ctx context.Context, svc string, fetcher LogFetcher) {
	if fetcher == nil {
		return
	}
	containerID, err := fetcher.ContainerIDByName(svc)
	if err != nil {
		return
	}
	var buf bytes.Buffer
	if err := fetcher.TailLogs(ctx, containerID, 20, &buf); err != nil {
		return
	}
	if buf.Len() > 0 {
		o.p.ServiceLogs(svc, buf.String())
	}
}
