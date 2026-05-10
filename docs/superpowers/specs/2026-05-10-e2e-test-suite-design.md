# E2E Test Suite Design

## Problem Statement

Stackup has ~30 unit tests covering internal logic, but no tests that exercise the CLI end-to-end against real Docker containers. This means regressions in health checks, startup sequencing, compose file discovery, and the .env gate go undetected until a user hits them. The test suite must run on any machine with Docker and Go installed, with no additional tools required.

## Goals

1. Option A: A permanent Go E2E suite (`tests/e2e/`) that runs in CI and catches regressions
2. Option D: A self-contained bash smoke test (`smoke-test.sh`) for one-shot manual validation on any machine — deleted after use
3. A `TESTING.md` guide covering prerequisites and run commands for every platform
4. All fixtures use only official pre-built Docker Hub images (no build steps, fast pulls)

## Non-Goals

- TUI rendering tests (no automated approach is practical)
- Performance benchmarks
- Windows-native PowerShell smoke test (Git Bash / WSL covers Windows)

---

## File Map

| File | Purpose |
|------|---------|
| `tests/e2e/main_test.go` | `TestMain`: builds stackup binary into temp dir before suite runs |
| `tests/e2e/helpers_test.go` | `runCLI()`, `tempFixture()`, `mustWriteFile()`, `waitForCleanup()` |
| `tests/e2e/compose_discovery_test.go` | compose.yaml / compose.yml / docker-compose.yaml / docker-compose.yml / --compose-file flag / missing file error |
| `tests/e2e/env_gate_test.go` | onboarding skipped, onboarding triggered, validate command (text + JSON) |
| `tests/e2e/health_http_test.go` | HTTP check passes, HTTP check times out on unreachable service |
| `tests/e2e/health_tcp_test.go` | TCP check passes on open port, fails on closed port |
| `tests/e2e/health_log_test.go` | log pattern found, log pattern times out |
| `tests/e2e/ordering_test.go` | tier 2 starts after tier 1 healthy, after_start hook runs |
| `tests/e2e/flags_test.go` | --only, --profile, --partial (partial success exit code 3) |
| `tests/e2e/commands_test.go` | stackup init, stackup doctor, stackup down, stackup run, --version |
| `tests/e2e/testdata/simple-stack/compose.yaml` | nginx:alpine + redis:7-alpine, no stackup.yml |
| `tests/e2e/testdata/http-health/compose.yaml` | nginx:alpine |
| `tests/e2e/testdata/http-health/stackup.yml` | HTTP health check on port 18080 |
| `tests/e2e/testdata/tcp-health/compose.yaml` | redis:7-alpine |
| `tests/e2e/testdata/tcp-health/stackup.yml` | TCP health check on port 16379 |
| `tests/e2e/testdata/log-health/compose.yaml` | nginx:alpine |
| `tests/e2e/testdata/log-health/stackup.yml` | log health check, pattern: `"ready for start up"` |
| `tests/e2e/testdata/multi-tier/compose.yaml` | redis + postgres:15-alpine + nginx (3-tier dependency chain) |
| `tests/e2e/testdata/multi-tier/stackup.yml` | TCP checks on all three services |
| `tests/e2e/testdata/profiles/compose.yaml` | nginx + redis with profile annotations |
| `tests/e2e/testdata/profiles/stackup.yml` | profiles: backend: [redis], frontend: [nginx] |
| `tests/e2e/testdata/with-env/compose.yaml` | nginx:alpine |
| `tests/e2e/testdata/with-env/stackup.yml` | env.schema with one required key |
| `tests/e2e/testdata/with-env/.env.example` | APP_PORT=8080 |
| `smoke-test.sh` | Option D: bash smoke test — deleted after manual validation |
| `TESTING.md` | Prerequisites and run commands for Linux, macOS, Windows, CI |

---

## Section 1: Go E2E Suite (Option A)

### TestMain — binary build

`tests/e2e/main_test.go` uses `TestMain` to build the stackup binary once before all tests:

```go
var stackupBin string

func TestMain(m *testing.M) {
    tmp, err := os.MkdirTemp("", "stackup-e2e-*")
    if err != nil {
        panic(err)
    }
    defer os.RemoveAll(tmp)
    stackupBin = filepath.Join(tmp, "stackup")
    if runtime.GOOS == "windows" {
        stackupBin += ".exe"
    }
    cmd := exec.Command("go", "build", "-o", stackupBin, "../../main.go")
    cmd.Dir = "."
    if out, err := cmd.CombinedOutput(); err != nil {
        panic(fmt.Sprintf("build failed: %s\n%s", err, out))
    }
    os.Exit(m.Run())
}
```

### Core helper — `runCLI`

```go
type CLIResult struct {
    Stdout   string
    Stderr   string
    ExitCode int
}

func runCLI(t *testing.T, dir string, args ...string) CLIResult {
    t.Helper()
    ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
    defer cancel()
    cmd := exec.CommandContext(ctx, stackupBin, args...)
    cmd.Dir = dir
    var stdout, stderr bytes.Buffer
    cmd.Stdout = &stdout
    cmd.Stderr = &stderr
    err := cmd.Run()
    code := 0
    if err != nil {
        var exitErr *exec.ExitError
        if errors.As(err, &exitErr) {
            code = exitErr.ExitCode()
        }
    }
    return CLIResult{
        Stdout:   stdout.String(),
        Stderr:   stderr.String(),
        ExitCode: code,
    }
}
```

### Fixture helper — `copyFixture`

Each test gets its own isolated temp directory with a recursive copy of the fixture:

```go
func copyFixture(t *testing.T, name string) string {
    t.Helper()
    src := filepath.Join("testdata", name)
    dst := t.TempDir()
    if err := copyDir(src, dst); err != nil {
        t.Fatalf("copyFixture %s: %v", name, err)
    }
    return dst
}

func copyDir(src, dst string) error {
    return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
        if err != nil {
            return err
        }
        rel, _ := filepath.Rel(src, path)
        target := filepath.Join(dst, rel)
        if d.IsDir() {
            return os.MkdirAll(target, 0755)
        }
        data, err := os.ReadFile(path)
        if err != nil {
            return err
        }
        return os.WriteFile(target, data, 0644)
    })
}
```

### Teardown — `stackup down`

Every test that starts containers runs `stackup down` in a `t.Cleanup` to prevent container leaks:

```go
t.Cleanup(func() {
    runCLI(t, dir, "down")
})
```

### Example test — compose discovery

```go
func TestComposDiscovery_FindsComposeDotYaml(t *testing.T) {
    dir := copyFixture(t, "simple-stack")
    t.Cleanup(func() { runCLI(t, dir, "down") })

    result := runCLI(t, dir, "up")
    if result.ExitCode != 0 {
        t.Fatalf("expected exit 0, got %d\nstdout: %s\nstderr: %s",
            result.ExitCode, result.Stdout, result.Stderr)
    }
}

func TestComposeDiscovery_MissingFileReturnsError(t *testing.T) {
    dir := t.TempDir() // empty dir, no compose file
    result := runCLI(t, dir, "up")
    if result.ExitCode == 0 {
        t.Fatal("expected non-zero exit when no compose file found")
    }
    if !strings.Contains(result.Stderr+result.Stdout, "no compose file found") {
        t.Errorf("expected 'no compose file found' in output, got: %s", result.Stdout+result.Stderr)
    }
}
```

### Example test — HTTP health check

```go
func TestHealthHTTP_PassesWhenNginxIsUp(t *testing.T) {
    dir := copyFixture(t, "http-health")
    t.Cleanup(func() { runCLI(t, dir, "down") })

    result := runCLI(t, dir, "up")
    if result.ExitCode != 0 {
        t.Fatalf("expected healthy exit 0, got %d\n%s", result.ExitCode, result.Stdout+result.Stderr)
    }
    if !strings.Contains(result.Stdout, "healthy") {
        t.Errorf("expected 'healthy' in output, got: %s", result.Stdout)
    }
}
```

### Port allocation strategy

All test fixtures use high-numbered ports (16379, 18080, 15432) to avoid conflicts with locally running services. Each fixture uses a unique port range so parallel test runs don't collide.

---

## Section 2: Bash Smoke Test (Option D)

`smoke-test.sh` is a single self-contained bash script that:

1. Checks prerequisites (docker, docker compose, the stackup binary path passed as $1)
2. Creates a temp working directory per test group, writes fixture files inline
3. Runs every CLI command, asserts on exit code and output
4. Prints colored PASS / FAIL per test with the failing command shown on FAIL
5. Runs `stackup down` and `docker compose down` after each group for cleanup
6. Prints a summary: `Passed: N/M` at the end, exits 1 if any test failed

Usage:
```bash
bash smoke-test.sh ./stackup
# or
bash smoke-test.sh /usr/local/bin/stackup
```

The script writes its own compose fixtures inline (heredoc) so it has zero external file dependencies — the script is fully self-contained.

Test groups in the smoke test:
1. **Compose Discovery** (5 tests): compose.yaml found, compose.yml found, docker-compose.yaml found, --compose-file flag, missing file error
2. **env Gate** (4 tests): no onboarding without schema, onboarding with .env.example, validate passes, validate --output json
3. **Health Checks** (7 tests): HTTP pass, HTTP timeout, TCP pass, TCP fail, log pass, log timeout, docker healthcheck
4. **Startup Sequencing** (4 tests): tier ordering, --only, --profile, --partial exit code 3
5. **CLI Commands** (7 tests): stackup init, stackup doctor, stackup down, stackup run, stackup restart, stackup validate, --version

Total: ~27 tests.

---

## Section 3: TESTING.md

The guide covers:
- Prerequisites per platform (Docker, Go, bash)
- How to build the binary
- How to run Option A (`go test ./tests/e2e/... -v -timeout 5m`)
- How to run a specific test file or test name
- How to run Option D (`bash smoke-test.sh ./stackup`)
- How to delete the smoke test after validation
- Troubleshooting: Docker not running, port conflicts, Windows Defender

---

## Testing the tests

The Go E2E suite itself is verified by:
1. `go build ./tests/e2e/...` — compilation check
2. Running the suite against a known-good state: all tests should pass
3. Running the suite with a deliberately broken stackup.yml — relevant tests should fail

The smoke test is verified by running it against the built binary on Linux (GitHub Actions ubuntu-latest) and checking exit code 0.

---

## Fixture Image Choices

| Fixture | Image | Why |
|---------|-------|-----|
| simple-stack, http-health, log-health | nginx:alpine | ~7MB, HTTP server on port 80, logs "ready for start up" |
| tcp-health, multi-tier (cache) | redis:7-alpine | ~10MB, TCP on 6379, fast startup |
| multi-tier (db) | postgres:15-alpine | ~80MB, TCP on 5432, realistic DB tier |
| log-health | nginx:alpine | Logs "ready for start up" — real log line from nginx startup |

All images are official Docker Hub images with no licensing restrictions.
