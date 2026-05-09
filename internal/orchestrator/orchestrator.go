package orchestrator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"
	"github.com/stackup-dev/stackup/internal/config"
	"github.com/stackup-dev/stackup/internal/env"
	"github.com/stackup-dev/stackup/internal/health"
	"github.com/stackup-dev/stackup/internal/printer"
)

// LogFetcher abstracts container log retrieval for failure diagnostics.
type LogFetcher interface {
	TailLogs(ctx context.Context, containerID string, lines int, w io.Writer) error
	ContainerIDByName(serviceName string) (string, error)
}

type Orchestrator struct {
	p *printer.Printer
}

func New(p *printer.Printer) *Orchestrator {
	return &Orchestrator{p: p}
}

func (o *Orchestrator) PreFlight(envFile, exampleFile string, schema map[string]config.EnvVar) bool {
	o.p.Phase("Pre-flight")
	result, injected := env.ValidateWithDefaults(envFile, exampleFile, schema)

	// Apply injected defaults to the process environment
	for key, val := range injected {
		os.Setenv(key, val)
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
		return true
	}
	for _, e := range result.Errors {
		o.p.ValidationError(e.Key, e.Message)
	}
	return false
}

func (o *Orchestrator) StartTier(ctx context.Context, tier Tier, deps []string, startFn func(context.Context, []string) error, checkers map[string]health.Named, fetcher LogFetcher) error {
	label := "Starting tier"
	if len(deps) > 0 {
		label += fmt.Sprintf("  (depends on: %s)", strings.Join(deps, ", "))
	}
	o.p.Phase(label)

	if err := startFn(ctx, tier); err != nil {
		return fmt.Errorf("failed to start tier %v: %w", []string(tier), err)
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
		return nil
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
	var firstFailure *checkResult
	for r := range results {
		if r.err != nil {
			if firstFailure == nil {
				failed := r
				firstFailure = &failed
			}
		} else {
			o.p.ServiceHealthy(r.svc, r.label, r.elapsed)
		}
	}

	if firstFailure != nil {
		o.p.ServiceFailed(firstFailure.svc, firstFailure.err)
		o.surfaceLogs(ctx, firstFailure.svc, fetcher)
		o.p.CleanupSuggestion([]string(tier))
		o.p.Hint("stackup doctor", "stackup logs "+firstFailure.svc)
		return fmt.Errorf("service %q failed health check: %w", firstFailure.svc, firstFailure.err)
	}
	return nil
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
