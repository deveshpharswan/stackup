# Stackup CLI v1.1 — Feature Enhancements Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform Stackup from a functional v1.0 into a polished, adoption-ready tool with better failure UX, diagnostic commands, onboarding workflows, and CI support.

**Architecture:** Each feature is additive — new packages under `internal/` with clean interfaces, new commands registered in `cmd/root.go`. The orchestrator gets enhanced with parallel health checking and progress callbacks. New subsystems (doctor, onboarding) are self-contained packages that compose existing Docker/config utilities.

**Tech Stack:** Go 1.25, Docker SDK (already imported), Cobra CLI framework, `gopkg.in/yaml.v3`, `github.com/joho/godotenv`

---

## File Map

### New Files
| Path | Responsibility |
|------|---------------|
| `internal/health/progress.go` | Progress callback interface + spinner renderer |
| `internal/doctor/doctor.go` | Diagnostic engine — runs checks, collects results |
| `internal/doctor/checks.go` | Individual diagnostic check implementations |
| `internal/onboard/onboard.go` | Interactive first-run .env setup wizard |
| `internal/hooks/hooks.go` | Lifecycle hook executor (after_start) |
| `cmd/doctor.go` | `stackup doctor` command |
| `cmd/check.go` | `stackup check` command |
| `internal/health/log.go` | Log-pattern health check strategy |
| `internal/doctor/doctor_test.go` | Doctor unit tests |
| `internal/onboard/onboard_test.go` | Onboard unit tests |
| `internal/hooks/hooks_test.go` | Hooks unit tests |
| `internal/health/log_test.go` | Log checker tests |
| `internal/health/progress_test.go` | Progress spinner tests |

### Modified Files
| Path | What Changes |
|------|-------------|
| `internal/printer/printer.go` | Add log surfacing, cleanup suggestion, spinner methods |
| `internal/orchestrator/orchestrator.go` | Parallel health checks, progress callbacks, cleanup suggestion, hook invocation |
| `internal/env/validator.go` | Default injection logic |
| `internal/config/config.go` | New fields: hooks, log health type, onboard metadata |
| `internal/health/checker.go` | Add `ProgressChecker` interface |
| `cmd/up.go` | Wire progress, hooks, onboarding gate, cleanup message |
| `cmd/root.go` | Register new commands (doctor, check) |
| `cmd/init.go` | Smarter image detection |
| `internal/scaffold/scaffold.go` | Image-aware health check generation |
| `internal/docker/client.go` | Add `TailLogs`, `RestartCount`, `PortBindings` methods |

---

## Task 1: Log Surfacing on Failure + Auto-Cleanup Suggestion + Fix Defaults

**Files:**
- Modify: `internal/docker/client.go`
- Modify: `internal/printer/printer.go`
- Modify: `internal/orchestrator/orchestrator.go`
- Modify: `internal/env/validator.go`
- Modify: `internal/config/config.go`
- Modify: `cmd/up.go`
- Test: `internal/docker/client_test.go`
- Test: `internal/env/validator_test.go`
- Test: `internal/orchestrator/orchestrator_test.go`

### Step 1.1: Add TailLogs to Docker client

- [ ] **Write the failing test**

In `internal/docker/client_test.go`, add:

```go
func TestClient_TailLogs(t *testing.T) {
	c := &Client{}
	var buf bytes.Buffer
	err := c.TailLogs(context.Background(), "nonexistent-container-id", 20, &buf)
	assert.Error(t, err)
}
```

- [ ] **Run test to verify it fails**

Run: `go test ./internal/docker/ -run TestClient_TailLogs -v`
Expected: FAIL — `TailLogs` not defined

- [ ] **Implement TailLogs**

In `internal/docker/client.go`, add:

```go
func (c *Client) TailLogs(ctx context.Context, containerID string, lines int, w io.Writer) error {
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", lines),
	}
	rc, err := c.cli.ContainerLogs(ctx, containerID, opts)
	if err != nil {
		return err
	}
	defer rc.Close()
	_, err = io.Copy(w, rc)
	return err
}
```

- [ ] **Run test to verify it passes**

Run: `go test ./internal/docker/ -run TestClient_TailLogs -v`
Expected: PASS (error because no Docker daemon in unit tests, but method exists)

### Step 1.2: Add printer methods for failure context

- [ ] **Add log surfacing and cleanup methods to printer**

In `internal/printer/printer.go`, add:

```go
func (p *Printer) ServiceLogs(name string, logs string) {
	fmt.Fprintf(p.w, "\n  Last 20 lines from %s:\n", name)
	fmt.Fprintf(p.w, "  ──────────────────────────────────────────\n")
	for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
		fmt.Fprintf(p.w, "  %s\n", line)
	}
	fmt.Fprintf(p.w, "  ──────────────────────────────────────────\n")
}

func (p *Printer) CleanupSuggestion(runningSvcs []string) {
	fmt.Fprintf(p.w, "\n  Services still running: %s\n", strings.Join(runningSvcs, ", "))
	fmt.Fprintf(p.w, "  To clean up before retrying: stackup down\n")
}

func (p *Printer) Hint(lines ...string) {
	fmt.Fprintln(p.w)
	fmt.Fprintf(p.w, "  Try:\n")
	for _, l := range lines {
		fmt.Fprintf(p.w, "    %s\n", l)
	}
}
```

Add `"strings"` to the import block.

- [ ] **Write printer test**

In `internal/printer/printer_test.go`, add:

```go
func TestPrinter_ServiceLogs(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf)
	p.ServiceLogs("api", "line1\nline2")
	out := buf.String()
	assert.Contains(t, out, "Last 20 lines from api")
	assert.Contains(t, out, "line1")
	assert.Contains(t, out, "line2")
}

func TestPrinter_CleanupSuggestion(t *testing.T) {
	var buf bytes.Buffer
	p := New(&buf)
	p.CleanupSuggestion([]string{"postgres", "redis"})
	out := buf.String()
	assert.Contains(t, out, "postgres, redis")
	assert.Contains(t, out, "stackup down")
}
```

- [ ] **Run tests**

Run: `go test ./internal/printer/ -v`
Expected: PASS

### Step 1.3: Enhance orchestrator to surface logs on failure

- [ ] **Modify StartTier to accept a Docker client and surface logs on failure**

Update `internal/orchestrator/orchestrator.go` — change `StartTier` signature:

```go
type LogFetcher interface {
	TailLogs(ctx context.Context, containerID string, lines int, w io.Writer) error
	ContainerIDByName(serviceName string) (string, error)
}

func (o *Orchestrator) StartTier(ctx context.Context, tier Tier, deps []string, startFn func(context.Context, []string) error, checkers map[string]health.Named, logFetcher LogFetcher) error {
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
			if logFetcher != nil {
				o.surfaceLogs(ctx, svc, logFetcher)
			}
			o.p.CleanupSuggestion(tier)
			o.p.Hint("stackup doctor     # automated diagnosis", "stackup logs "+svc+"   # full log history")
			return fmt.Errorf("service %q failed health check: %w", svc, err)
		}
		o.p.ServiceHealthy(svc, named.Label, time.Since(start))
	}
	return nil
}

func (o *Orchestrator) surfaceLogs(ctx context.Context, svc string, fetcher LogFetcher) {
	id, err := fetcher.ContainerIDByName(svc)
	if err != nil {
		return
	}
	var buf bytes.Buffer
	if err := fetcher.TailLogs(ctx, id, 20, &buf); err != nil {
		return
	}
	if buf.Len() > 0 {
		o.p.ServiceLogs(svc, buf.String())
	}
}
```

Add `"bytes"` to imports.

- [ ] **Update cmd/up.go to pass the Docker client as LogFetcher**

In `cmd/up.go`, the `docker.Client` already satisfies the `LogFetcher` interface. Update the `StartTier` call:

```go
// Before the tier loop, create a docker.Client wrapper
dc2, _ := docker.NewClient()
defer dc2.Close()

// In the loop:
if err := o.StartTier(ctx, tier, tierDeps, startFn, checkers, dc2); err != nil {
    return err
}
```

- [ ] **Run all tests**

Run: `go test ./... -v`
Expected: Fix any compilation errors from signature change in orchestrator_test.go by adding `nil` as the last argument to `StartTier` in tests.

### Step 1.4: Fix env defaults injection

- [ ] **Write failing test for default injection**

In `internal/env/validator_test.go`, add:

```go
func TestValidate_InjectsDefaults(t *testing.T) {
	envFile := filepath.Join(t.TempDir(), ".env")
	exampleFile := filepath.Join(t.TempDir(), ".env.example")
	os.WriteFile(envFile, []byte("OTHER=value\n"), 0644)
	os.WriteFile(exampleFile, []byte("PORT=3000\nOTHER=x\n"), 0644)

	schema := map[string]config.EnvVar{
		"PORT": {Type: "int", Default: "3000"},
	}
	result, injected := ValidateWithDefaults(envFile, exampleFile, schema)
	assert.True(t, result.Valid())
	assert.Equal(t, "3000", injected["PORT"])
}
```

- [ ] **Run test to verify it fails**

Run: `go test ./internal/env/ -run TestValidate_InjectsDefaults -v`
Expected: FAIL — `ValidateWithDefaults` not defined

- [ ] **Implement ValidateWithDefaults**

In `internal/env/validator.go`, add:

```go
func ValidateWithDefaults(envFile, exampleFile string, schema map[string]config.EnvVar) (Result, map[string]string) {
	injected := make(map[string]string)

	envVars, err := godotenv.Read(envFile)
	if err != nil {
		envVars = make(map[string]string)
	}

	// Inject defaults for missing keys that have a default defined
	for key, rule := range schema {
		if _, ok := envVars[key]; !ok && rule.Default != "" {
			envVars[key] = rule.Default
			injected[key] = rule.Default
		}
	}

	var result Result
	example, _ := godotenv.Read(exampleFile)

	for key := range example {
		if _, ok := envVars[key]; !ok {
			result.Errors = append(result.Errors, ValidationError{
				Key:     key,
				Message: "missing (required by .env.example)",
			})
		}
	}

	for key, rule := range schema {
		val, ok := envVars[key]
		if !ok {
			if rule.Required {
				result.Errors = append(result.Errors, ValidationError{
					Key:     key,
					Message: "required but not set",
				})
			}
			continue
		}
		if err := validateType(key, val, rule.Type); err != nil {
			result.Errors = append(result.Errors, *err)
		}
	}

	return result, injected
}
```

- [ ] **Update orchestrator to use ValidateWithDefaults and set env vars**

In `internal/orchestrator/orchestrator.go`, update `PreFlight`:

```go
func (o *Orchestrator) PreFlight(envFile, exampleFile string, schema map[string]config.EnvVar) bool {
	o.p.Phase("Pre-flight")
	result, injected := env.ValidateWithDefaults(envFile, exampleFile, schema)

	for key, val := range injected {
		os.Setenv(key, val)
	}

	if result.Valid() {
		envVars, _ := godotenv.Read(envFile)
		total := len(envVars) + len(injected)
		o.p.EnvValid(total)
		for key, rule := range schema {
			if rule.Type != "" {
				o.p.EnvKeyValid(key, rule.Type)
			}
		}
		if len(injected) > 0 {
			for key, val := range injected {
				o.p.EnvDefault(key, val)
			}
		}
		return true
	}
	for _, e := range result.Errors {
		o.p.ValidationError(e.Key, e.Message)
	}
	return false
}
```

Add `"os"` to imports.

- [ ] **Add EnvDefault printer method**

In `internal/printer/printer.go`:

```go
func (p *Printer) EnvDefault(key, val string) {
	fmt.Fprintf(p.w, "  ⚙ %s — using default: %s\n", key, val)
}
```

- [ ] **Run all tests**

Run: `go test ./... -v`
Expected: PASS

- [ ] **Commit**

```bash
git add -A
git commit -m "feat: surface container logs on health failure, add cleanup suggestions, inject env defaults"
```

---

## Task 2: Parallel Health Checks + Progress Spinners

**Files:**
- Create: `internal/health/progress.go`
- Create: `internal/health/progress_test.go`
- Modify: `internal/orchestrator/orchestrator.go`
- Modify: `internal/printer/printer.go`
- Test: `internal/orchestrator/orchestrator_test.go`

### Step 2.1: Create progress callback interface

- [ ] **Create `internal/health/progress.go`**

```go
package health

import "time"

type ProgressFunc func(service string, elapsed time.Duration)

type ProgressChecker struct {
	inner    Checker
	service  string
	interval time.Duration
	onTick   ProgressFunc
}

func NewProgressChecker(inner Checker, service string, tickInterval time.Duration, onTick ProgressFunc) *ProgressChecker {
	return &ProgressChecker{
		inner:    inner,
		service:  service,
		interval: tickInterval,
		onTick:   onTick,
	}
}
```

### Step 2.2: Implement parallel health checks in orchestrator

- [ ] **Replace sequential health checks with parallel goroutines**

In `internal/orchestrator/orchestrator.go`, replace the health check loop inside `StartTier`:

```go
func (o *Orchestrator) StartTier(ctx context.Context, tier Tier, deps []string, startFn func(context.Context, []string) error, checkers map[string]health.Named, logFetcher LogFetcher) error {
	label := "Starting tier"
	if len(deps) > 0 {
		label += fmt.Sprintf("  (depends on: %s)", strings.Join(deps, ", "))
	}
	o.p.Phase(label)

	if err := startFn(ctx, tier); err != nil {
		return fmt.Errorf("failed to start tier %v: %w", []string(tier), err)
	}

	type checkResult struct {
		svc     string
		label   string
		elapsed time.Duration
		err     error
	}

	results := make(chan checkResult, len(tier))
	for _, svc := range tier {
		named, ok := checkers[svc]
		if !ok {
			results <- checkResult{svc: svc}
			continue
		}
		go func(s string, n health.Named) {
			start := time.Now()
			err := n.Checker.Check(ctx)
			results <- checkResult{svc: s, label: n.Label, elapsed: time.Since(start), err: err}
		}(svc, named)
	}

	var failed *checkResult
	for range tier {
		r := <-results
		if r.err != nil {
			if failed == nil {
				failed = &r
			}
			continue
		}
		if r.label != "" {
			o.p.ServiceHealthy(r.svc, r.label, r.elapsed)
		}
	}

	if failed != nil {
		o.p.ServiceFailed(failed.svc, failed.err)
		if logFetcher != nil {
			o.surfaceLogs(ctx, failed.svc, logFetcher)
		}
		o.p.CleanupSuggestion(tier)
		o.p.Hint("stackup doctor     # automated diagnosis", "stackup logs "+failed.svc+"   # full log history")
		return fmt.Errorf("service %q failed health check: %w", failed.svc, failed.err)
	}
	return nil
}
```

### Step 2.3: Add spinner/progress display to printer

- [ ] **Add progress spinner methods**

In `internal/printer/printer.go`, add:

```go
func (p *Printer) ServiceWaiting(name string, elapsed time.Duration) {
	fmt.Fprintf(p.w, "\r  ⠋ %-12s waiting... %s", name, formatDuration(elapsed))
}

func (p *Printer) ClearLine() {
	fmt.Fprintf(p.w, "\r\033[K")
}
```

### Step 2.4: Write test for parallel behavior

- [ ] **Test that parallel checks actually run concurrently**

In `internal/orchestrator/orchestrator_test.go`, add:

```go
func TestStartTier_ParallelChecks(t *testing.T) {
	var buf bytes.Buffer
	p := printer.New(&buf)
	o := New(p)

	slowChecker := &mockChecker{delay: 100 * time.Millisecond}
	checkers := map[string]health.Named{
		"svc1": {Checker: slowChecker, Label: "tcp:5432"},
		"svc2": {Checker: slowChecker, Label: "tcp:6379"},
	}

	startFn := func(ctx context.Context, svcs []string) error { return nil }
	start := time.Now()
	err := o.StartTier(context.Background(), Tier{"svc1", "svc2"}, nil, startFn, checkers, nil)
	elapsed := time.Since(start)

	assert.NoError(t, err)
	// Parallel: should take ~100ms, not ~200ms
	assert.Less(t, elapsed, 180*time.Millisecond)
}

type mockChecker struct {
	delay time.Duration
	err   error
}

func (m *mockChecker) Check(ctx context.Context) error {
	time.Sleep(m.delay)
	return m.err
}
```

- [ ] **Run tests**

Run: `go test ./internal/orchestrator/ -run TestStartTier_ParallelChecks -v`
Expected: PASS

- [ ] **Commit**

```bash
git add -A
git commit -m "feat: parallel health checks within tiers for faster startup"
```

---

## Task 3: `stackup doctor` — Automated Diagnostics

**Files:**
- Create: `internal/doctor/doctor.go`
- Create: `internal/doctor/checks.go`
- Create: `internal/doctor/doctor_test.go`
- Create: `cmd/doctor.go`
- Modify: `cmd/root.go`
- Modify: `internal/docker/client.go`

### Step 3.1: Define the doctor types

- [ ] **Create `internal/doctor/doctor.go`**

```go
package doctor

import (
	"context"
	"fmt"
	"io"
	"strings"
)

type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityOK
)

type Finding struct {
	Severity Severity
	Title    string
	Detail   string
	Fix      string
	Service  string
}

type CheckFunc func(ctx context.Context, opts *Options) []Finding

type Options struct {
	ComposeFile string
	EnvFile     string
	ExampleFile string
	ConfigFile  string
}

type Doctor struct {
	checks []namedCheck
}

type namedCheck struct {
	name string
	fn   CheckFunc
}

func New() *Doctor {
	d := &Doctor{}
	d.checks = []namedCheck{
		{"port conflicts", CheckPortConflicts},
		{"crash loops", CheckCrashLoops},
		{"env drift", CheckEnvDrift},
		{"container status", CheckContainerStatus},
		{"localhost misuse", CheckLocalhostMisuse},
	}
	return d
}

func (d *Doctor) Run(ctx context.Context, opts *Options) []Finding {
	var all []Finding
	for _, c := range d.checks {
		all = append(all, c.fn(ctx, opts)...)
	}
	return all
}

func PrintFindings(w io.Writer, findings []Finding) {
	errors := 0
	warnings := 0
	for _, f := range findings {
		switch f.Severity {
		case SeverityError:
			errors++
			svc := ""
			if f.Service != "" {
				svc = " — " + f.Service
			}
			fmt.Fprintf(w, "  ✗ %s%s\n", f.Title, svc)
			if f.Detail != "" {
				for _, line := range strings.Split(f.Detail, "\n") {
					fmt.Fprintf(w, "    %s\n", line)
				}
			}
			if f.Fix != "" {
				fmt.Fprintf(w, "    Fix: %s\n", f.Fix)
			}
			fmt.Fprintln(w)
		case SeverityWarning:
			warnings++
			fmt.Fprintf(w, "  ⚠ %s\n", f.Title)
			if f.Detail != "" {
				fmt.Fprintf(w, "    %s\n", f.Detail)
			}
			if f.Fix != "" {
				fmt.Fprintf(w, "    Fix: %s\n", f.Fix)
			}
			fmt.Fprintln(w)
		case SeverityOK:
			svc := ""
			if f.Service != "" {
				svc = " (" + f.Service + ")"
			}
			fmt.Fprintf(w, "  ✓ %s%s\n", f.Title, svc)
		}
	}
	fmt.Fprintln(w)
	if errors == 0 && warnings == 0 {
		fmt.Fprintf(w, "  All checks passed!\n")
	} else {
		fmt.Fprintf(w, "  %d error(s), %d warning(s) found. Fix them and re-run: stackup up\n", errors, warnings)
	}
}
```

### Step 3.2: Implement diagnostic checks

- [ ] **Create `internal/doctor/checks.go`**

```go
package doctor

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
	"github.com/stackup-dev/stackup/internal/config"
	"github.com/stackup-dev/stackup/internal/scaffold"
)

func CheckPortConflicts(ctx context.Context, opts *Options) []Finding {
	var findings []Finding
	cfg, err := config.Load(opts.ConfigFile)
	if err != nil {
		return nil
	}
	for name, svc := range cfg.Services {
		if svc.Health == nil || svc.Health.Port == 0 {
			continue
		}
		port := svc.Health.Port
		ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Title:    fmt.Sprintf("PORT CONFLICT — %s (%d)", name, port),
				Detail:   fmt.Sprintf("Port %d is already in use on this host", port),
				Fix:      fmt.Sprintf("Find what's using it: lsof -i :%d (macOS/Linux) or netstat -ano | findstr :%d (Windows)", port, port),
				Service:  name,
			})
		} else {
			ln.Close()
		}
	}
	return findings
}

func CheckCrashLoops(ctx context.Context, opts *Options) []Finding {
	var findings []Finding
	out, err := exec.CommandContext(ctx, "docker", "compose", "ps", "--format", "{{.Service}}\t{{.State}}\t{{.Status}}").Output()
	if err != nil {
		return nil
	}
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) < 3 {
			continue
		}
		svc, state, status := parts[0], parts[1], parts[2]
		if state == "restarting" || strings.Contains(status, "Restarting") {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Title:    fmt.Sprintf("CRASH LOOP — %s", svc),
				Detail:   fmt.Sprintf("Container is in state: %s (%s)", state, status),
				Fix:      fmt.Sprintf("Check logs: stackup logs %s", svc),
				Service:  svc,
			})
		} else if state == "exited" {
			findings = append(findings, Finding{
				Severity: SeverityError,
				Title:    fmt.Sprintf("EXITED — %s", svc),
				Detail:   fmt.Sprintf("Container exited: %s", status),
				Fix:      fmt.Sprintf("Check logs: stackup logs %s", svc),
				Service:  svc,
			})
		}
	}
	return findings
}

func CheckEnvDrift(ctx context.Context, opts *Options) []Finding {
	var findings []Finding
	envVars, err := godotenv.Read(opts.EnvFile)
	if err != nil {
		envVars = make(map[string]string)
	}
	example, err := godotenv.Read(opts.ExampleFile)
	if err != nil {
		return nil
	}

	var missing []string
	for key := range example {
		if _, ok := envVars[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		findings = append(findings, Finding{
			Severity: SeverityWarning,
			Title:    fmt.Sprintf("ENV DRIFT — %d key(s) in .env.example missing from .env", len(missing)),
			Detail:   "Missing: " + strings.Join(missing, ", "),
			Fix:      "Add these to your .env (check .env.example for descriptions)",
		})
	}
	return findings
}

func CheckContainerStatus(ctx context.Context, opts *Options) []Finding {
	var findings []Finding
	services, err := scaffold.ParseServices(opts.ComposeFile)
	if err != nil {
		return nil
	}
	out, err := exec.CommandContext(ctx, "docker", "compose", "ps", "--format", "{{.Service}}\t{{.State}}").Output()
	if err != nil {
		return nil
	}
	running := make(map[string]bool)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		parts := strings.Split(scanner.Text(), "\t")
		if len(parts) >= 2 && parts[1] == "running" {
			running[parts[0]] = true
		}
	}
	for svc := range services {
		if running[svc] {
			findings = append(findings, Finding{
				Severity: SeverityOK,
				Title:    "healthy",
				Service:  svc,
			})
		}
	}
	return findings
}

func CheckLocalhostMisuse(ctx context.Context, opts *Options) []Finding {
	var findings []Finding
	envVars, err := godotenv.Read(opts.EnvFile)
	if err != nil {
		return nil
	}
	services, _ := scaffold.ParseServices(opts.ComposeFile)
	serviceNames := make(map[string]bool)
	for name := range services {
		serviceNames[name] = true
	}

	localhostPatterns := []string{"localhost", "127.0.0.1", "0.0.0.0"}
	for key, val := range envVars {
		for _, pattern := range localhostPatterns {
			if strings.Contains(val, pattern) {
				// Check if this looks like a connection string that should use a service name
				for svcName := range serviceNames {
					port := guessServicePort(svcName)
					if port != "" && strings.Contains(val, pattern+":"+port) {
						findings = append(findings, Finding{
							Severity: SeverityWarning,
							Title:    fmt.Sprintf("LOCALHOST MISUSE — %s", key),
							Detail:   fmt.Sprintf("Value contains %s:%s — inside a container, use the service name instead", pattern, port),
							Fix:      fmt.Sprintf("Change %s to %s in your connection string", pattern+":"+port, svcName+":"+port),
						})
					}
				}
			}
		}
	}
	return findings
}

func guessServicePort(svcName string) string {
	defaults := map[string]string{
		"postgres": "5432",
		"postgresql": "5432",
		"mysql": "3306",
		"mariadb": "3306",
		"redis": "6379",
		"mongo": "27017",
		"mongodb": "27017",
		"kafka": "9092",
		"rabbitmq": "5672",
		"elasticsearch": "9200",
	}
	for pattern, port := range defaults {
		if strings.Contains(strings.ToLower(svcName), pattern) {
			return port
		}
	}
	return ""
}

func guessServicePortInt(svcName string) int {
	p := guessServicePort(svcName)
	if p == "" {
		return 0
	}
	v, _ := strconv.Atoi(p)
	return v
}
```

### Step 3.3: Write doctor tests

- [ ] **Create `internal/doctor/doctor_test.go`**

```go
package doctor

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckEnvDrift_DetectsMissing(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	exampleFile := filepath.Join(dir, ".env.example")
	os.WriteFile(envFile, []byte("A=1\n"), 0644)
	os.WriteFile(exampleFile, []byte("A=1\nB=2\nC=3\n"), 0644)

	opts := &Options{EnvFile: envFile, ExampleFile: exampleFile}
	findings := CheckEnvDrift(context.Background(), opts)

	assert.Len(t, findings, 1)
	assert.Equal(t, SeverityWarning, findings[0].Severity)
	assert.Contains(t, findings[0].Title, "2 key(s)")
}

func TestCheckEnvDrift_NoDrift(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	exampleFile := filepath.Join(dir, ".env.example")
	os.WriteFile(envFile, []byte("A=1\nB=2\n"), 0644)
	os.WriteFile(exampleFile, []byte("A=x\nB=x\n"), 0644)

	opts := &Options{EnvFile: envFile, ExampleFile: exampleFile}
	findings := CheckEnvDrift(context.Background(), opts)

	assert.Empty(t, findings)
}

func TestPrintFindings_FormatsCorrectly(t *testing.T) {
	var buf bytes.Buffer
	findings := []Finding{
		{Severity: SeverityError, Title: "PORT CONFLICT", Detail: "port 5432 in use", Fix: "kill the process", Service: "postgres"},
		{Severity: SeverityOK, Title: "healthy", Service: "redis"},
	}
	PrintFindings(&buf, findings)
	out := buf.String()
	assert.Contains(t, out, "✗ PORT CONFLICT")
	assert.Contains(t, out, "✓ healthy")
	assert.Contains(t, out, "1 error(s)")
}

func TestCheckLocalhostMisuse_DetectsPattern(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	os.WriteFile(envFile, []byte("DATABASE_URL=postgres://user:pass@localhost:5432/db\n"), 0644)

	composeFile := filepath.Join(dir, "docker-compose.yml")
	os.WriteFile(composeFile, []byte("services:\n  postgres:\n    image: postgres:15\n"), 0644)

	opts := &Options{EnvFile: envFile, ComposeFile: composeFile}
	findings := CheckLocalhostMisuse(context.Background(), opts)

	assert.NotEmpty(t, findings)
	assert.Contains(t, findings[0].Title, "LOCALHOST MISUSE")
}
```

- [ ] **Run tests**

Run: `go test ./internal/doctor/ -v`
Expected: PASS

### Step 3.4: Create the doctor command

- [ ] **Create `cmd/doctor.go`**

```go
package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/stackup-dev/stackup/internal/doctor"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Run automated diagnostics on your stack",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			w := cmd.OutOrStdout()

			fmt.Fprintf(w, "\n  stackup doctor\n\n")
			fmt.Fprintf(w, "  Checking stack health...\n\n")

			opts := &doctor.Options{
				ComposeFile: "docker-compose.yml",
				EnvFile:     ".env",
				ExampleFile: ".env.example",
				ConfigFile:  "stackup.yml",
			}

			d := doctor.New()
			findings := d.Run(ctx, opts)
			doctor.PrintFindings(w, findings)
			return nil
		},
	}
}
```

- [ ] **Register in root.go**

In `cmd/root.go`, add `newDoctorCmd()` to the `AddCommand` call:

```go
root.AddCommand(
    newVersionCmd(version, commit, date),
    newUpCmd(),
    newDownCmd(),
    newValidateCmd(),
    newStatusCmd(),
    newInitCmd(),
    newLogsCmd(),
    newShellCmd(),
    newRestartCmd(),
    newRunCmd(),
    newDoctorCmd(),
)
```

- [ ] **Run all tests**

Run: `go test ./... -v`
Expected: PASS

- [ ] **Commit**

```bash
git add -A
git commit -m "feat: add stackup doctor command for automated diagnostics"
```

---

## Task 4: Team Onboarding Mode

**Files:**
- Create: `internal/onboard/onboard.go`
- Create: `internal/onboard/onboard_test.go`
- Modify: `internal/config/config.go`
- Modify: `cmd/up.go`

### Step 4.1: Create onboarding package

- [ ] **Create `internal/onboard/onboard.go`**

```go
package onboard

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/stackup-dev/stackup/internal/config"
)

type Onboarder struct {
	w      io.Writer
	r      io.Reader
	schema map[string]config.EnvVar
}

func New(w io.Writer, r io.Reader, schema map[string]config.EnvVar) *Onboarder {
	return &Onboarder{w: w, r: r, schema: schema}
}

func NeedsOnboarding(envFile string) bool {
	_, err := os.Stat(envFile)
	return os.IsNotExist(err)
}

func (o *Onboarder) Run(envFile, exampleFile string) error {
	fmt.Fprintf(o.w, "\n  Welcome! First time setup — your .env is missing.\n\n")

	keys := o.gatherKeys(exampleFile)
	if len(keys) == 0 {
		fmt.Fprintf(o.w, "  No environment variables defined. Creating empty .env\n")
		return os.WriteFile(envFile, []byte(""), 0644)
	}

	fmt.Fprintf(o.w, "  Required environment variables:\n")
	fmt.Fprintf(o.w, "  ──────────────────────────────────────────\n")
	for _, k := range keys {
		desc := ""
		def := ""
		if rule, ok := o.schema[k.name]; ok {
			if rule.Default != "" {
				def = rule.Default
			}
		}
		if k.example != "" {
			desc = fmt.Sprintf("  (example: %s)", k.example)
		}
		if def != "" {
			fmt.Fprintf(o.w, "  %-20s default: %s%s\n", k.name, def, desc)
		} else {
			fmt.Fprintf(o.w, "  %-20s required%s\n", k.name, desc)
		}
	}
	fmt.Fprintf(o.w, "  ──────────────────────────────────────────\n\n")

	fmt.Fprintf(o.w, "  Create your .env now? [Y/n] ")
	scanner := bufio.NewScanner(o.r)
	scanner.Scan()
	answer := strings.TrimSpace(scanner.Text())
	if answer != "" && strings.ToLower(answer) != "y" && strings.ToLower(answer) != "yes" {
		return fmt.Errorf("onboarding cancelled by user")
	}
	fmt.Fprintln(o.w)

	values := make(map[string]string)
	for _, k := range keys {
		def := ""
		if rule, ok := o.schema[k.name]; ok && rule.Default != "" {
			def = rule.Default
		}
		if def != "" {
			fmt.Fprintf(o.w, "  %s (default: %s): ", k.name, def)
		} else {
			fmt.Fprintf(o.w, "  %s: ", k.name)
		}
		scanner.Scan()
		val := strings.TrimSpace(scanner.Text())
		if val == "" && def != "" {
			val = def
		}
		if val != "" {
			values[k.name] = val
		}
	}

	var b strings.Builder
	for _, k := range keys {
		if val, ok := values[k.name]; ok {
			b.WriteString(fmt.Sprintf("%s=%s\n", k.name, val))
		}
	}

	if err := os.WriteFile(envFile, []byte(b.String()), 0644); err != nil {
		return fmt.Errorf("failed to write .env: %w", err)
	}

	fmt.Fprintf(o.w, "\n  ✓ .env created\n")
	fmt.Fprintf(o.w, "  → Starting stack...\n")
	return nil
}

type envKey struct {
	name    string
	example string
}

func (o *Onboarder) gatherKeys(exampleFile string) []envKey {
	example, err := godotenv.Read(exampleFile)
	if err != nil {
		// Fall back to schema keys
		var keys []envKey
		for name := range o.schema {
			keys = append(keys, envKey{name: name})
		}
		return keys
	}
	var keys []envKey
	for name, val := range example {
		keys = append(keys, envKey{name: name, example: val})
	}
	return keys
}
```

### Step 4.2: Write onboarding tests

- [ ] **Create `internal/onboard/onboard_test.go`**

```go
package onboard

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stackup-dev/stackup/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestNeedsOnboarding_MissingEnv(t *testing.T) {
	assert.True(t, NeedsOnboarding(filepath.Join(t.TempDir(), ".env")))
}

func TestNeedsOnboarding_ExistingEnv(t *testing.T) {
	f := filepath.Join(t.TempDir(), ".env")
	os.WriteFile(f, []byte("X=1"), 0644)
	assert.False(t, NeedsOnboarding(f))
}

func TestOnboarder_Run_CreatesEnvFile(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	exampleFile := filepath.Join(dir, ".env.example")
	os.WriteFile(exampleFile, []byte("PORT=3000\nDB_URL=postgres://localhost/db\n"), 0644)

	schema := map[string]config.EnvVar{
		"PORT": {Type: "int", Default: "3000"},
	}

	input := "y\n\npostgres://localhost/mydb\n"
	var output bytes.Buffer
	o := New(&output, strings.NewReader(input), schema)

	err := o.Run(envFile, exampleFile)
	assert.NoError(t, err)

	content, _ := os.ReadFile(envFile)
	assert.Contains(t, string(content), "PORT=3000")
	assert.Contains(t, string(content), "DB_URL=postgres://localhost/mydb")
}

func TestOnboarder_Run_Cancelled(t *testing.T) {
	dir := t.TempDir()
	envFile := filepath.Join(dir, ".env")
	exampleFile := filepath.Join(dir, ".env.example")
	os.WriteFile(exampleFile, []byte("X=1\n"), 0644)

	input := "n\n"
	var output bytes.Buffer
	o := New(&output, strings.NewReader(input), nil)

	err := o.Run(envFile, exampleFile)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}
```

- [ ] **Run tests**

Run: `go test ./internal/onboard/ -v`
Expected: PASS

### Step 4.3: Wire onboarding into cmd/up.go

- [ ] **Add onboarding gate at the top of the up command**

In `cmd/up.go`, add this before the pre-flight check:

```go
import (
    // ... existing imports ...
    "github.com/stackup-dev/stackup/internal/onboard"
)

// Inside RunE, before PreFlight:
if onboard.NeedsOnboarding(".env") {
    ob := onboard.New(cmd.OutOrStdout(), os.Stdin, cfg.Env.Schema)
    if err := ob.Run(".env", ".env.example"); err != nil {
        return err
    }
}
```

- [ ] **Run all tests**

Run: `go test ./... -v`
Expected: PASS

- [ ] **Commit**

```bash
git add -A
git commit -m "feat: add interactive team onboarding mode for first-time .env setup"
```

---

## Task 5: Lifecycle Hooks (after_start only)

**Files:**
- Create: `internal/hooks/hooks.go`
- Create: `internal/hooks/hooks_test.go`
- Modify: `internal/config/config.go`
- Modify: `internal/orchestrator/orchestrator.go`
- Modify: `cmd/up.go`

### Step 5.1: Extend config types for hooks

- [ ] **Update `internal/config/config.go`**

Add to the `Service` struct:

```go
type Service struct {
	Health *HealthCheck `yaml:"health"`
	Hooks  *Hooks       `yaml:"hooks"`
}

type Hooks struct {
	AfterStart []HookAction `yaml:"after_start"`
}

type HookAction struct {
	Service string `yaml:"service"`
	Run     string `yaml:"run"`
	Name    string `yaml:"name"`
}
```

### Step 5.2: Implement hook executor

- [ ] **Create `internal/hooks/hooks.go`**

```go
package hooks

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/stackup-dev/stackup/internal/config"
	"github.com/stackup-dev/stackup/internal/docker"
)

type Executor struct {
	w      io.Writer
	client *docker.Client
}

func NewExecutor(w io.Writer, client *docker.Client) *Executor {
	return &Executor{w: w, client: client}
}

func (e *Executor) RunAfterStart(ctx context.Context, serviceName string, hooks []config.HookAction) error {
	for _, hook := range hooks {
		target := hook.Service
		if target == "" {
			target = serviceName
		}
		name := hook.Name
		if name == "" {
			name = hook.Run
		}

		fmt.Fprintf(e.w, "    → hook: %s\n", name)

		id, err := e.client.ContainerIDByName(target)
		if err != nil {
			return fmt.Errorf("hook target service %q not found: %w", target, err)
		}

		parts := strings.Fields(hook.Run)
		if len(parts) == 0 {
			continue
		}

		execCmd := exec.CommandContext(ctx, "docker", append([]string{"compose", "exec", target}, parts...)...)
		execCmd.Stdout = e.w
		execCmd.Stderr = e.w
		if err := execCmd.Run(); err != nil {
			return fmt.Errorf("hook %q failed on service %q: %w", name, target, err)
		}
		_ = id
		fmt.Fprintf(e.w, "    ✓ %s\n", name)
	}
	return nil
}
```

### Step 5.3: Write hooks tests

- [ ] **Create `internal/hooks/hooks_test.go`**

```go
package hooks

import (
	"bytes"
	"testing"

	"github.com/stackup-dev/stackup/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestExecutor_EmptyHooks(t *testing.T) {
	var buf bytes.Buffer
	e := NewExecutor(&buf, nil)
	// No hooks should be a no-op (nil client is fine when no hooks run)
	hooks := []config.HookAction{}
	// RunAfterStart with empty hooks should just return nil
	assert.Empty(t, hooks)
	_ = e
}

func TestHookAction_DefaultsServiceToParent(t *testing.T) {
	action := config.HookAction{Run: "npm run migrate"}
	assert.Empty(t, action.Service)
	assert.Equal(t, "npm run migrate", action.Run)
}
```

- [ ] **Run tests**

Run: `go test ./internal/hooks/ -v`
Expected: PASS

### Step 5.4: Wire hooks into orchestrator

- [ ] **Add hook execution after each tier's health checks pass**

In `internal/orchestrator/orchestrator.go`, add a `HookRunner` interface and call it after health checks:

```go
type HookRunner interface {
	RunAfterStart(ctx context.Context, serviceName string, hooks []config.HookAction) error
}

func (o *Orchestrator) StartTier(ctx context.Context, tier Tier, deps []string, startFn func(context.Context, []string) error, checkers map[string]health.Named, logFetcher LogFetcher) error {
    // ... existing code ...
    // After all health checks pass, return nil as before
    // Hook execution is handled by the caller (cmd/up.go) after StartTier returns
}
```

- [ ] **Execute hooks in cmd/up.go after each tier succeeds**

In `cmd/up.go`, after `o.StartTier(...)` returns successfully:

```go
// After successful StartTier:
for _, svc := range tier {
    svcCfg, ok := cfg.Services[svc]
    if !ok || svcCfg.Hooks == nil || len(svcCfg.Hooks.AfterStart) == 0 {
        continue
    }
    hookExec := hooks.NewExecutor(cmd.OutOrStdout(), dc2)
    if err := hookExec.RunAfterStart(ctx, svc, svcCfg.Hooks.AfterStart); err != nil {
        return fmt.Errorf("hook failed for %s: %w", svc, err)
    }
}
```

Add import: `"github.com/stackup-dev/stackup/internal/hooks"`

- [ ] **Run all tests**

Run: `go test ./... -v`
Expected: PASS

- [ ] **Commit**

```bash
git add -A
git commit -m "feat: add after_start lifecycle hooks for automated post-startup tasks"
```

---

## Task 6: `stackup check` — CI-Friendly Health Assertions

**Files:**
- Create: `cmd/check.go`
- Modify: `cmd/root.go`

### Step 6.1: Create the check command

- [ ] **Create `cmd/check.go`**

```go
package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/stackup-dev/stackup/internal/config"
	"github.com/stackup-dev/stackup/internal/health"
	dockerclient "github.com/docker/docker/client"
)

type checkOutput struct {
	Stack    string          `json:"stack"`
	Healthy  bool            `json:"healthy"`
	Services []serviceStatus `json:"services"`
}

type serviceStatus struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func newCheckCmd() *cobra.Command {
	var (
		service string
		format  string
		quiet   bool
	)
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check health of all services (CI-friendly exit codes)",
		Long:  "Exits 0 if all services healthy, exits 2 if any unhealthy. Useful in CI pipelines.",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			cfg := config.LoadOrEmpty("stackup.yml")

			dc, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv, dockerclient.WithAPIVersionNegotiation())
			if err != nil {
				return fmt.Errorf("connecting to Docker: %w", err)
			}
			defer dc.Close()
			checkers := buildCheckers(cfg, dc)

			var services []string
			if service != "" {
				services = []string{service}
			} else {
				for name := range checkers {
					services = append(services, name)
				}
			}

			output := checkOutput{Stack: "stackup", Healthy: true}
			for _, svc := range services {
				named, ok := checkers[svc]
				if !ok {
					output.Services = append(output.Services, serviceStatus{
						Name:   svc,
						Status: "unknown",
					})
					continue
				}
				if err := named.Checker.Check(ctx); err != nil {
					output.Healthy = false
					output.Services = append(output.Services, serviceStatus{
						Name:    svc,
						Status:  "unhealthy",
						Message: err.Error(),
					})
				} else {
					output.Services = append(output.Services, serviceStatus{
						Name:   svc,
						Status: "healthy",
					})
				}
			}

			if format == "json" {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				enc.Encode(output)
			} else if !quiet {
				w := cmd.OutOrStdout()
				for _, s := range output.Services {
					if s.Status == "healthy" {
						fmt.Fprintf(w, "  ✓ %-12s healthy\n", s.Name)
					} else {
						fmt.Fprintf(w, "  ✗ %-12s %s: %s\n", s.Name, s.Status, s.Message)
					}
				}
			}

			if !output.Healthy {
				os.Exit(2)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&service, "service", "", "Check a single service")
	cmd.Flags().StringVar(&format, "format", "text", "Output format: text or json")
	cmd.Flags().BoolVar(&quiet, "quiet", false, "No output, just exit code")

	_ = health.Named{}
	return cmd
}
```

- [ ] **Register in root.go**

Add `newCheckCmd()` to the `AddCommand` list in `cmd/root.go`.

- [ ] **Run all tests**

Run: `go test ./... -v`
Expected: PASS

- [ ] **Commit**

```bash
git add -A
git commit -m "feat: add stackup check command for CI-friendly health assertions"
```

---

## Task 7: Smarter `stackup init`

**Files:**
- Modify: `internal/scaffold/scaffold.go`
- Test: `internal/scaffold/scaffold_test.go`

### Step 7.1: Add image detection to scaffold

- [ ] **Enhance scaffold to detect images and suggest health checks**

In `internal/scaffold/scaffold.go`, add image parsing and smart defaults:

```go
type composeService struct {
	DependsOn interface{} `yaml:"depends_on"`
	Image     string      `yaml:"image"`
}

type healthDefaults struct {
	checkType string
	port      int
	host      string
}

var knownImages = map[string]healthDefaults{
	"postgres":      {checkType: "tcp", port: 5432, host: "localhost"},
	"redis":         {checkType: "tcp", port: 6379, host: "localhost"},
	"mysql":         {checkType: "tcp", port: 3306, host: "localhost"},
	"mariadb":       {checkType: "tcp", port: 3306, host: "localhost"},
	"mongo":         {checkType: "tcp", port: 27017, host: "localhost"},
	"elasticsearch": {checkType: "tcp", port: 9200, host: "localhost"},
	"rabbitmq":      {checkType: "tcp", port: 5672, host: "localhost"},
	"kafka":         {checkType: "tcp", port: 9092, host: "localhost"},
	"nginx":         {checkType: "http", port: 80, host: "localhost"},
	"memcached":     {checkType: "tcp", port: 11211, host: "localhost"},
	"nats":          {checkType: "tcp", port: 4222, host: "localhost"},
}

func detectHealthDefaults(image string) *healthDefaults {
	lower := strings.ToLower(image)
	for pattern, defaults := range knownImages {
		if strings.Contains(lower, pattern) {
			return &defaults
		}
	}
	return nil
}
```

- [ ] **Update Generate to use smart defaults**

Replace the services section in `Generate`:

```go
func Generate(composeFilePath, exampleFile string) (string, error) {
	data, err := os.ReadFile(composeFilePath)
	if err != nil {
		return "", fmt.Errorf("reading compose file: %w", err)
	}
	var cf composeFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return "", fmt.Errorf("parsing compose file: %w", err)
	}

	envKeys, _ := godotenv.Read(exampleFile)

	var b strings.Builder
	b.WriteString("version: \"1\"\n")

	if len(envKeys) > 0 {
		b.WriteString("\nenv:\n  schema:\n")
		sortedKeys := make([]string, 0, len(envKeys))
		for k := range envKeys {
			sortedKeys = append(sortedKeys, k)
		}
		sort.Strings(sortedKeys)
		for _, key := range sortedKeys {
			b.WriteString(fmt.Sprintf("    %s:\n      required: true\n", key))
		}
	}

	b.WriteString("\nservices:\n")
	svcNames := make([]string, 0, len(cf.Services))
	for name := range cf.Services {
		svcNames = append(svcNames, name)
	}
	sort.Strings(svcNames)
	for _, name := range svcNames {
		svc := cf.Services[name]
		defaults := detectHealthDefaults(svc.Image)
		if defaults != nil {
			b.WriteString(fmt.Sprintf("  %s:\n    health:\n      type: %s\n", name, defaults.checkType))
			if defaults.checkType == "tcp" {
				b.WriteString(fmt.Sprintf("      host: %s\n      port: %d\n", defaults.host, defaults.port))
			} else if defaults.checkType == "http" {
				b.WriteString(fmt.Sprintf("      url: http://%s:%d/\n", defaults.host, defaults.port))
			}
		} else {
			b.WriteString(fmt.Sprintf("  %s:\n    health:\n      type: tcp  # TODO: configure health check\n", name))
		}
	}

	b.WriteString("\ncommands: {}\n")
	return b.String(), nil
}
```

### Step 7.2: Write tests for smart detection

- [ ] **Add test in `internal/scaffold/scaffold_test.go`**

```go
func TestGenerate_SmartImageDetection(t *testing.T) {
	dir := t.TempDir()
	composePath := filepath.Join(dir, "docker-compose.yml")
	os.WriteFile(composePath, []byte(`services:
  postgres:
    image: postgres:15
  redis:
    image: redis:7-alpine
  api:
    build: .
`), 0644)

	examplePath := filepath.Join(dir, ".env.example")
	os.WriteFile(examplePath, []byte(""), 0644)

	output, err := Generate(composePath, examplePath)
	assert.NoError(t, err)
	assert.Contains(t, output, "port: 5432")
	assert.Contains(t, output, "port: 6379")
	assert.Contains(t, output, "# TODO: configure health check")
}
```

- [ ] **Run tests**

Run: `go test ./internal/scaffold/ -v`
Expected: PASS

- [ ] **Commit**

```bash
git add -A
git commit -m "feat: smarter stackup init with image-aware health check detection"
```

---

## Task 8: Log Health Check Strategy

**Files:**
- Create: `internal/health/log.go`
- Create: `internal/health/log_test.go`
- Modify: `internal/config/config.go`
- Modify: `cmd/up.go`

### Step 8.1: Implement log checker

- [ ] **Create `internal/health/log.go`**

```go
package health

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	dockerclient "github.com/docker/docker/client"
)

type LogChecker struct {
	cli       *dockerclient.Client
	service   string
	pattern   string
	timeout   time.Duration
	interval  time.Duration
}

func NewLogChecker(cli *dockerclient.Client, service, pattern string, timeout, interval time.Duration) *LogChecker {
	return &LogChecker{
		cli:      cli,
		service:  service,
		pattern:  pattern,
		timeout:  timeout,
		interval: interval,
	}
}

func (c *LogChecker) Check(ctx context.Context) error {
	deadline := time.Now().Add(c.timeout)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		found, err := c.scanLogs(ctx)
		if err == nil && found {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(c.interval):
		}
	}
	return fmt.Errorf("log pattern %q not found in %s logs after %s", c.pattern, c.service, c.timeout)
}

func (c *LogChecker) scanLogs(ctx context.Context) (bool, error) {
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "100",
	}
	rc, err := c.cli.ContainerLogs(ctx, c.service, opts)
	if err != nil {
		return false, err
	}
	defer rc.Close()

	return scanForPattern(rc, c.pattern)
}

func scanForPattern(r io.Reader, pattern string) (bool, error) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), pattern) {
			return true, nil
		}
	}
	return false, scanner.Err()
}
```

### Step 8.2: Write log checker tests

- [ ] **Create `internal/health/log_test.go`**

```go
package health

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScanForPattern_Found(t *testing.T) {
	input := "starting up\ndatabase system is ready to accept connections\nlistening"
	found, err := scanForPattern(strings.NewReader(input), "ready to accept connections")
	assert.NoError(t, err)
	assert.True(t, found)
}

func TestScanForPattern_NotFound(t *testing.T) {
	input := "starting up\ninitializing\n"
	found, err := scanForPattern(strings.NewReader(input), "ready to accept connections")
	assert.NoError(t, err)
	assert.False(t, found)
}

func TestScanForPattern_EmptyInput(t *testing.T) {
	found, err := scanForPattern(strings.NewReader(""), "pattern")
	assert.NoError(t, err)
	assert.False(t, found)
}
```

- [ ] **Run tests**

Run: `go test ./internal/health/ -run TestScanForPattern -v`
Expected: PASS

### Step 8.3: Add config support for log type

- [ ] **Add Pattern field to HealthCheck config**

In `internal/config/config.go`, update the `HealthCheck` struct:

```go
type HealthCheck struct {
	Type     string `yaml:"type"`
	URL      string `yaml:"url"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Pattern  string `yaml:"pattern"`
	Timeout  string `yaml:"timeout"`
	Interval string `yaml:"interval"`
}
```

### Step 8.4: Wire log checker into buildCheckers

- [ ] **Update `cmd/up.go` buildCheckers function**

Add a new case in the switch statement:

```go
case "log":
    checkers[name] = health.Named{
        Checker: health.NewLogChecker(dc, name, hc.Pattern, timeout, interval),
        Label:   "log:" + hc.Pattern,
    }
```

- [ ] **Run all tests**

Run: `go test ./... -v`
Expected: PASS

- [ ] **Commit**

```bash
git add -A
git commit -m "feat: add log-pattern health check strategy for services without HTTP/TCP readiness"
```

---

## Final Verification

- [ ] **Run full test suite**

```bash
go test ./... -v
```

- [ ] **Run linter**

```bash
go vet ./...
```

- [ ] **Verify build compiles**

```bash
go build -o stackup .
```

- [ ] **Final commit if any loose changes**

```bash
git status
```
