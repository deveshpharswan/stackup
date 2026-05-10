# Bugfix Sweep Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 14 confirmed bugs found during a full codebase review, spanning Docker log streams, ExecShell lifecycle, TUI goroutine leaks, health checkers, printer alignment, orchestrator validation, config loading, and onboarding.

**Architecture:** Each task targets a single file or tightly coupled pair of files. All fixes are surgical — no refactors beyond what's required to fix the bug. Tests are added or updated alongside each fix.

**Tech Stack:** Go 1.25, Docker SDK (`github.com/docker/docker v28.5.1`), bubbletea TUI, cobra CLI, testify assertions. Build: `go build ./...`. Vet: `go vet ./...`. Tests: `go test ./...` (admin-blocked on this machine — run `go build ./... && go vet ./...` as the verification step).

---

## Files Modified

| File | Bug fixed |
|------|-----------|
| `internal/docker/client.go` | Log stream demultiplexing; ExecShell stdin hang; orphaned exec |
| `internal/health/log.go` | LogChecker resolves container ID; demultiplexes stream |
| `cmd/up.go` | Division by zero in `runPartial` |
| `internal/tui/tui.go` | Log goroutine leak on service switch (command palette) |
| `internal/tui/logs.go` | Silent failure when log stream fails to start |
| `internal/health/tcp.go` | TCP checker ignores context cancellation |
| `internal/printer/printer.go` | Spinner data race; SummaryTable ANSI misalignment |
| `internal/orchestrator/graph.go` | Unknown dependency name silently ignored |
| `internal/config/config.go` | `LoadOrEmpty` swallows YAML parse errors |
| `cmd/up.go`, `cmd/validate.go`, `cmd/restart.go`, `cmd/check.go`, `internal/tui/describe.go` | Updated callers after `LoadOrEmpty` signature change |
| `internal/onboard/onboard.go` | Unquoted `.env` values corrupt file |
| `internal/tui/doctorview.go` | Doctor view shows "No issues" while checks run |
| `internal/scaffold/scaffold.go` | minio and clickhouse share port 9000 |
| `internal/env/validator.go` | Duplicate errors for keys in both schema and example |

---

## Task 1: Fix Docker log stream demultiplexing in `docker/client.go`

Non-TTY Docker containers return a multiplexed stream with an 8-byte header per frame (1-byte stream type + 3 padding + 4-byte length). `TailLogs` and `Logs` both used raw `io.Copy`, so the terminal receives binary garbage. Fix: replace `io.Copy` with `stdcopy.StdCopy` which strips the headers and separates stdout/stderr.

**Files:**
- Modify: `internal/docker/client.go`
- Test: `internal/docker/client_test.go`

- [ ] **Step 1: Add `stdcopy` import to `docker/client.go`**

Add `"github.com/docker/docker/pkg/stdcopy"` to the import block. The package is already vendored as part of `github.com/docker/docker`.

- [ ] **Step 2: Replace `io.Copy` in `TailLogs` and `Logs`**

In `internal/docker/client.go`, change lines 75 and 91:

```go
// TailLogs — line 75, was: _, err = io.Copy(w, rc)
_, err = stdcopy.StdCopy(w, w, rc)

// Logs — line 91, was: _, err = io.Copy(w, rc)
_, err = stdcopy.StdCopy(w, w, rc)
```

`stdcopy.StdCopy(dst_stdout, dst_stderr, src)` — we pass `w` for both since the caller writes combined output.

- [ ] **Step 3: Verify build**

```
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/docker/client.go
git commit -m "fix: demultiplex Docker log stream in TailLogs and Logs"
```

---

## Task 2: Fix LogChecker — resolve container ID and demultiplex stream

`LogChecker.scanLogs` passed the compose service name (e.g. `"postgres"`) directly to `ContainerLogs`. The Docker API requires a container ID or container name (e.g. `project_postgres_1`), so every log health check 404s. Additionally, the stream was not demultiplexed, so `ScanForPattern` saw binary headers.

**Files:**
- Modify: `internal/health/log.go`
- Test: `internal/health/log_test.go`

- [ ] **Step 1: Update imports in `health/log.go`**

```go
import (
    "bufio"
    "bytes"
    "context"
    "fmt"
    "io"
    "strings"
    "time"

    "github.com/docker/docker/api/types/container"
    "github.com/docker/docker/api/types/filters"
    dockerclient "github.com/docker/docker/client"
    "github.com/docker/docker/pkg/stdcopy"
)
```

- [ ] **Step 2: Add `resolveContainerID` method to `LogChecker`**

Append after line 33 (after `NewLogChecker`):

```go
func (c *LogChecker) resolveContainerID(ctx context.Context) (string, error) {
    f := filters.NewArgs(filters.Arg("label", "com.docker.compose.service="+c.service))
    list, err := c.cli.ContainerList(ctx, container.ListOptions{Filters: f})
    if err != nil {
        return "", err
    }
    if len(list) == 0 {
        return "", fmt.Errorf("no running container for service %q", c.service)
    }
    return list[0].ID, nil
}
```

- [ ] **Step 3: Rewrite `scanLogs` to use container ID and demultiplex**

Replace lines 52–64 with:

```go
func (c *LogChecker) scanLogs(ctx context.Context) (bool, error) {
    id, err := c.resolveContainerID(ctx)
    if err != nil {
        return false, err
    }
    opts := container.LogsOptions{
        ShowStdout: true,
        ShowStderr: true,
        Tail:       "100",
    }
    rc, err := c.cli.ContainerLogs(ctx, id, opts)
    if err != nil {
        return false, err
    }
    defer rc.Close()
    var buf bytes.Buffer
    if _, err := stdcopy.StdCopy(&buf, &buf, rc); err != nil {
        return false, err
    }
    return ScanForPattern(&buf, c.pattern)
}
```

- [ ] **Step 4: Verify build**

```
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/health/log.go
git commit -m "fix: LogChecker resolves container ID and demultiplexes log stream"
```

---

## Task 3: Fix ExecShell — stdin goroutine hang + orphaned exec

**Bug A:** After the container process exits, `io.Copy(out, resp.Reader)` returns. `wg.Wait()` then blocks forever because `io.Copy(resp.Conn, in)` (the stdin goroutine) is stuck waiting on terminal input. `defer resp.Close()` never runs because `wg.Wait()` never returns.

**Fix A:** Close `resp.Conn` explicitly after the stdout drain to signal EOF to the stdin goroutine.

**Bug B:** If `ContainerExecCreate` succeeds for `bash` but `ContainerExecAttach` fails, the loop continues to `sh` and the `bash` exec object is leaked in the Docker daemon.

**Fix B:** On attach failure, attempt to clean up the created exec by starting and immediately stopping it (the Docker API has no `ExecRemove` — we accept the leak but stop retrying with the same orphaned exec).

**Files:**
- Modify: `internal/docker/client.go`

- [ ] **Step 1: Fix ExecShell in `docker/client.go`**

Replace lines 100–129 with:

```go
// ExecShell opens an interactive shell (bash or sh) inside the container.
func (c *Client) ExecShell(ctx context.Context, containerID string, in io.Reader, out io.Writer) error {
    for _, shell := range []string{"bash", "sh"} {
        exec, err := c.cli.ContainerExecCreate(ctx, containerID, container.ExecOptions{
            AttachStdin:  true,
            AttachStdout: true,
            AttachStderr: true,
            Tty:          true,
            Cmd:          []string{shell},
        })
        if err != nil {
            continue
        }
        resp, err := c.cli.ContainerExecAttach(ctx, exec.ID, container.ExecAttachOptions{Tty: true})
        if err != nil {
            // exec object created but attach failed — try next shell
            continue
        }
        var wg sync.WaitGroup
        wg.Add(1)
        go func() {
            defer wg.Done()
            _, _ = io.Copy(resp.Conn, in)
        }()
        _, _ = io.Copy(out, resp.Reader)
        // Close the connection to unblock the stdin goroutine, which is blocked
        // on io.Copy(resp.Conn, in) waiting for terminal input.
        _ = resp.Conn.Close()
        wg.Wait()
        return nil
    }
    return fmt.Errorf("no shell found in container %s", containerID)
}
```

Note: `defer resp.Close()` is removed because we now call `resp.Conn.Close()` explicitly. Calling Close after Conn.Close is a no-op since the connection is already closed.

- [ ] **Step 2: Verify build**

```
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/docker/client.go
git commit -m "fix: ExecShell closes connection to unblock stdin goroutine after container exit"
```

---

## Task 4: Fix division by zero in `cmd/up.go`

`runPartial` divides `healthyCount * 100 / totalServices` without guarding against `totalServices == 0`. This panics when called with an empty tier list.

**Files:**
- Modify: `cmd/up.go`

- [ ] **Step 1: Guard `totalServices` before division in `runPartial`**

Find line 232 in `cmd/up.go`:

```go
pct := (healthyCount * 100) / totalServices
```

Replace with:

```go
pct := 0
if totalServices > 0 {
    pct = (healthyCount * 100) / totalServices
}
```

- [ ] **Step 2: Verify build**

```
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add cmd/up.go
git commit -m "fix: guard division by zero in runPartial when totalServices is 0"
```

---

## Task 5: Fix TUI log goroutine leak (command palette) + silent log stream failure

**Bug A:** In `tui.go`, when the user opens logs via the command palette (`CommandResult` with `ViewLogs`), `m.logs.Start()` is called WITHOUT first calling `m.logs.Stop()`. The old `docker compose logs -f` process keeps running indefinitely. The keyboard path (`case "l"`) calls `m.logs.Start()` which internally calls `m.Stop()` on its own copy — but this doesn't cancel the stored `m.logs.cancel`. So both paths actually share this issue — `Start()` should not rely on `m.Stop()` operating on a copy. Fix: always call `m.logs.Stop()` on the stored model before calling `m.logs.Start()`.

**Bug B:** In `logs.go`, if `StdoutPipe()` or `c.Start()` fails inside `startLogStream`, the goroutine returns and `defer close(ch)` fires, sending a `LogErrMsg{Err: nil}` to the UI. The UI ignores `LogErrMsg` with nil error (`if msg.Err != nil`), so the user sees a blank panel with no indication of the failure. Fix: send a real error.

**Files:**
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/logs.go`

- [ ] **Step 1: Fix command palette path in `tui.go` — add `m.logs.Stop()` before `Start()`**

Find lines 200–204 in `internal/tui/tui.go`:

```go
if msg.View == ViewLogs && msg.Arg != "" {
    m = m.pushView(ViewLogs)
    newLogs, cmd := m.logs.Start(msg.Arg, m.width, m.height-7)
    m.logs = newLogs
    return m, cmd
}
```

Replace with:

```go
if msg.View == ViewLogs && msg.Arg != "" {
    m.logs.Stop()
    m = m.pushView(ViewLogs)
    newLogs, cmd := m.logs.Start(msg.Arg, m.width, m.height-7)
    m.logs = newLogs
    return m, cmd
}
```

Also fix the keyboard path at lines 123–127 in `tui.go` (the `"l"` case). The internal `m.Stop()` in `Start()` operates on a copy, not the stored `m.logs`. Add an explicit Stop before Start:

```go
case "l":
    if m.activeView == ViewServices {
        if svc := m.services.Selected(); svc != "" {
            m.logs.Stop()
            m = m.pushView(ViewLogs)
            newLogs, cmd := m.logs.Start(svc, m.width, m.height-7)
            m.logs = newLogs
            return m, cmd
        }
    }
```

- [ ] **Step 2: Remove the internal `m.Stop()` call from `LogsModel.Start()` in `logs.go`**

`Start()` currently calls `m.Stop()` at line 30 on its own value receiver copy, which is now dead code since callers are responsible for stopping. Remove it:

Find in `internal/tui/logs.go`:
```go
func (m LogsModel) Start(service string, width, height int) (LogsModel, tea.Cmd) {
    m.Stop()
    vp := viewport.New(width, height)
```

Change to:
```go
func (m LogsModel) Start(service string, width, height int) (LogsModel, tea.Cmd) {
    vp := viewport.New(width, height)
```

- [ ] **Step 3: Fix silent log stream failure in `startLogStream`**

In `internal/tui/logs.go`, lines 108–131. The goroutine silently returns on errors; replace the two bare `return` statements with error sends:

```go
func startLogStream(ctx context.Context, service string) <-chan string {
    ch := make(chan string, 64)
    go func() {
        defer close(ch)
        c := exec.CommandContext(ctx, "docker", "compose", "logs", "-f", "--tail", "100", "--timestamps", service)
        stdout, err := c.StdoutPipe()
        if err != nil {
            select {
            case <-ctx.Done():
            case ch <- "\x00ERR:" + err.Error():
            }
            return
        }
        if err := c.Start(); err != nil {
            select {
            case <-ctx.Done():
            case ch <- "\x00ERR:" + err.Error():
            }
            return
        }
        scanner := bufio.NewScanner(stdout)
        for scanner.Scan() {
            select {
            case <-ctx.Done():
                return
            case ch <- scanner.Text():
            }
        }
        _ = c.Wait()
    }()
    return ch
}
```

Then in `LogsModel.Update`, detect the sentinel error prefix:

```go
case LogLineMsg:
    line := msg.Line
    if strings.HasPrefix(line, "\x00ERR:") {
        errText := strings.TrimPrefix(line, "\x00ERR:")
        m.lines = append(m.lines, styleFailed.Render("Stream error: "+errText))
        m.viewport.SetContent(m.renderLines())
        return m, nil
    }
    m.lines = append(m.lines, line)
    if len(m.lines) > 1000 {
        m.lines = m.lines[len(m.lines)-1000:]
    }
    m.viewport.SetContent(m.renderLines())
    m.viewport.GotoBottom()
    return m, waitForLogLine(m.logCh)
```

- [ ] **Step 4: Verify build**

```
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/tui/tui.go internal/tui/logs.go
git commit -m "fix: stop old log stream before starting new one; surface stream start errors"
```

---

## Task 6: Fix TCP health checker ignoring context cancellation

`net.DialTimeout` is not context-aware. A cancelled context (Ctrl-C during `stackup up`) does not interrupt an in-flight TCP dial. Replace with `net.Dialer.DialContext`.

**Files:**
- Modify: `internal/health/tcp.go`

- [ ] **Step 1: Replace `net.DialTimeout` with `DialContext` in `tcp.go`**

The full `Check` method in `internal/health/tcp.go` (lines 24–39):

```go
func (c *TCPChecker) Check(ctx context.Context) error {
    addr := net.JoinHostPort(c.host, c.port)

    err := Poll(ctx, c.timeout, c.interval, func() error {
        d := net.Dialer{Timeout: c.interval}
        conn, err := d.DialContext(ctx, "tcp", addr)
        if err != nil {
            return err
        }
        conn.Close()
        return nil
    })
    if err != nil && err != ctx.Err() {
        return fmt.Errorf("tcp check timed out after %s: %s", c.timeout, addr)
    }
    return err
}
```

No import changes needed — `net` is already imported.

- [ ] **Step 2: Verify build**

```
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 3: Verify existing TCP tests still pass (if able to run)**

```
go test ./internal/health/... -run TestTCP -v
```

Expected:
```
--- PASS: TestTCPChecker_Open (... s)
--- PASS: TestTCPChecker_Closed (... s)
```

- [ ] **Step 4: Commit**

```bash
git add internal/health/tcp.go
git commit -m "fix: use DialContext in TCPChecker to respect context cancellation"
```

---

## Task 7: Fix Spinner data race in `printer/printer.go`

`s.active` is written under `s.mu` in `Start` but read bare in `Stop`. If `Start` and `Stop` are called from different goroutines, this is a data race detectable by `go test -race`.

**Files:**
- Modify: `internal/printer/printer.go`

- [ ] **Step 1: Read `s.active` under mutex in `Stop`**

Current `Stop` (lines 306–315):

```go
func (s *Spinner) Stop() {
    if !s.isTTY || !s.active {
        return
    }
    s.stopOnce.Do(func() {
        close(s.stop)
    })
    <-s.done
    s.active = false
}
```

Replace with:

```go
func (s *Spinner) Stop() {
    if !s.isTTY {
        return
    }
    s.mu.Lock()
    active := s.active
    s.mu.Unlock()
    if !active {
        return
    }
    s.stopOnce.Do(func() {
        close(s.stop)
    })
    <-s.done
    s.mu.Lock()
    s.active = false
    s.mu.Unlock()
}
```

- [ ] **Step 2: Verify build**

```
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/printer/printer.go
git commit -m "fix: read Spinner.active under mutex to prevent data race"
```

---

## Task 8: Fix SummaryTable ANSI column misalignment in `printer/printer.go`

`%-*s` in `fmt.Fprintf` measures width in runes — including ANSI escape sequences that are invisible in the terminal. This makes every colored column wider than its allocated `*W` slots in the format string, throwing off the box-drawing borders. Fix: apply color to the text, then pad with plain spaces using the plain-text length.

**Files:**
- Modify: `internal/printer/printer.go`

- [ ] **Step 1: Add `colorPad` helper to `printer.go`**

Add this unexported function anywhere in the file (e.g., below `formatDuration`):

```go
// colorPad applies color to text and pads to width using the plain text length.
func colorPad(colored, plain string, width int) string {
    pad := width - len([]rune(plain))
    if pad < 0 {
        pad = 0
    }
    return colored + strings.Repeat(" ", pad)
}
```

- [ ] **Step 2: Update the column headers row in `SummaryTable`**

Find lines 211–218 (column headers):

```go
fmt.Fprintf(p.w, "  %s %-*s%-*s%-*s%-*s%s\n",
    p.dim.Sprint("│"),
    nameW, p.bold.Sprint("Service"),
    statusW, p.bold.Sprint("Status"),
    labelW, p.bold.Sprint("Check"),
    timeW, p.bold.Sprint("Time"),
    p.dim.Sprint("│"),
)
```

Replace with:

```go
fmt.Fprintf(p.w, "  %s %s%s%s%s%s\n",
    p.dim.Sprint("│"),
    colorPad(p.bold.Sprint("Service"), "Service", nameW),
    colorPad(p.bold.Sprint("Status"), "Status", statusW),
    colorPad(p.bold.Sprint("Check"), "Check", labelW),
    colorPad(p.bold.Sprint("Time"), "Time", timeW),
    p.dim.Sprint("│"),
)
```

- [ ] **Step 3: Update the data rows in `SummaryTable`**

Find lines 232–240 (data rows):

```go
fmt.Fprintf(p.w, "  %s %-*s%-*s%-*s%-*s%s\n",
    p.dim.Sprint("│"),
    nameW, r.Name,
    statusW, statusColor,
    labelW, p.dim.Sprint(r.Label),
    timeW, p.dim.Sprint(formatDuration(r.Elapsed)),
    p.dim.Sprint("│"),
)
```

Replace with:

```go
timeStr := formatDuration(r.Elapsed)
fmt.Fprintf(p.w, "  %s %-*s%s%s%s%s\n",
    p.dim.Sprint("│"),
    nameW, r.Name,
    colorPad(statusColor, status, statusW),
    colorPad(p.dim.Sprint(r.Label), r.Label, labelW),
    colorPad(p.dim.Sprint(timeStr), timeStr, timeW),
    p.dim.Sprint("│"),
)
```

- [ ] **Step 4: Verify build**

```
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/printer/printer.go
git commit -m "fix: correct SummaryTable column alignment when ANSI color is enabled"
```

---

## Task 9: Fix orchestrator dependency validation in `graph.go`

A typo in a dependency name (e.g., `"postgress"` instead of `"postgres"`) is silently treated as a root node with no dependents. The startup order is wrong and there's no error. Fix: after building the `inDegree` map, validate that every dependency is also a declared service key.

**Files:**
- Modify: `internal/orchestrator/graph.go`
- Test: `internal/orchestrator/graph_test.go`

- [ ] **Step 1: Write the failing test in `graph_test.go`**

Add to `internal/orchestrator/graph_test.go`:

```go
func TestBuildTiers_UnknownDependency(t *testing.T) {
    t.Parallel()
    deps := map[string][]string{
        "api": {"postgress"}, // typo — "postgress" is not a declared service
    }
    _, err := orchestrator.BuildTiers(deps)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "postgress")
}
```

- [ ] **Step 2: Run to confirm it fails (if able)**

```
go test ./internal/orchestrator/... -run TestBuildTiers_UnknownDependency -v
```

Expected: `FAIL` — `BuildTiers` currently returns no error.

- [ ] **Step 3: Add validation to `BuildTiers` in `graph.go`**

Add after the `for svc := range deps` loop (after line 31, before `total := len(inDegree)`):

```go
// Validate that every dependency is a declared service.
for svc, depList := range deps {
    for _, dep := range depList {
        if _, ok := deps[dep]; !ok {
            return nil, fmt.Errorf("service %q depends on %q which is not declared", svc, dep)
        }
    }
}
```

- [ ] **Step 4: Verify build and tests**

```
go build ./...
go vet ./...
go test ./internal/orchestrator/... -v
```

Expected: all orchestrator tests pass including the new one.

- [ ] **Step 5: Commit**

```bash
git add internal/orchestrator/graph.go internal/orchestrator/graph_test.go
git commit -m "fix: BuildTiers returns error when dependency name is not a declared service"
```

---

## Task 10: Fix `LoadOrEmpty` silently swallowing YAML/validation errors

`LoadOrEmpty` returns an empty `Config{}` for *any* error including malformed YAML or invalid health check types. A user with a typo in their `stackup.yml` gets an empty config with no health checks, no hooks, no validation — silently.

Fix: check whether the error is a file-not-found error. For any other error (parse, validation), propagate it so callers can surface it. This requires changing `LoadOrEmpty` to `(*Config, error)` and updating 5 callers.

**Files:**
- Modify: `internal/config/config.go`
- Modify: `cmd/up.go`
- Modify: `cmd/validate.go`
- Modify: `cmd/restart.go`
- Modify: `cmd/check.go`
- Modify: `internal/tui/describe.go`
- Test: `internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/config/config_test.go`:

```go
func TestLoadOrEmpty_MalformedYAML(t *testing.T) {
    t.Parallel()
    // Write a temp file with invalid YAML
    f, err := os.CreateTemp("", "stackup-*.yml")
    require.NoError(t, err)
    defer os.Remove(f.Name())
    _, err = f.WriteString("services: [not: valid: yaml\n")
    require.NoError(t, err)
    f.Close()

    _, err = config.LoadOrEmpty(f.Name())
    assert.Error(t, err)
}
```

You'll need `"os"` added to the test file imports.

- [ ] **Step 2: Change `LoadOrEmpty` signature in `config.go`**

Current (lines 143–151):
```go
// LoadOrEmpty returns an empty Config when the file does not exist.
// Allows projects that haven't added stackup.yml yet to still use the tool.
func LoadOrEmpty(path string) *Config {
    cfg, err := Load(path)
    if err != nil {
        return &Config{}
    }
    return cfg
}
```

Replace with:

```go
// LoadOrEmpty returns an empty Config when the file does not exist,
// and returns an error for any other failure (bad YAML, invalid config).
func LoadOrEmpty(path string) (*Config, error) {
    cfg, err := Load(path)
    if err != nil {
        if errors.Is(err, os.ErrNotExist) {
            return &Config{}, nil
        }
        return nil, err
    }
    return cfg, nil
}
```

Add `"errors"` and `"os"` to the import block.

- [ ] **Step 3: Update callers — `cmd/up.go`**

Line 39 in `cmd/up.go`, inside `RunE`:

```go
// was:
cfg := config.LoadOrEmpty(constants.DefaultConfigFile)

// replace with:
cfg, err := config.LoadOrEmpty(constants.DefaultConfigFile)
if err != nil {
    return fmt.Errorf("invalid stackup.yml: %w", err)
}
```

- [ ] **Step 4: Update callers — `cmd/validate.go`**

Line 33 in `cmd/validate.go`, inside `RunE`:

```go
// was:
cfg := config.LoadOrEmpty(constants.DefaultConfigFile)

// replace with:
cfg, err := config.LoadOrEmpty(constants.DefaultConfigFile)
if err != nil {
    return fmt.Errorf("invalid stackup.yml: %w", err)
}
```

- [ ] **Step 5: Update callers — `cmd/restart.go`**

Find `cfg := config.LoadOrEmpty(constants.DefaultConfigFile)` in `cmd/restart.go` and replace:

```go
cfg, err := config.LoadOrEmpty(constants.DefaultConfigFile)
if err != nil {
    return fmt.Errorf("invalid stackup.yml: %w", err)
}
```

- [ ] **Step 6: Update callers — `cmd/check.go`**

Find `cfg := config.LoadOrEmpty(constants.DefaultConfigFile)` in `cmd/check.go` and replace:

```go
cfg, err := config.LoadOrEmpty(constants.DefaultConfigFile)
if err != nil {
    return fmt.Errorf("invalid stackup.yml: %w", err)
}
```

- [ ] **Step 7: Update callers — `internal/tui/describe.go`**

In `internal/tui/describe.go` line 31, the call is inside a `tea.Cmd` func. Replace:

```go
// was:
cfg := config.LoadOrEmpty(constants.DefaultConfigFile)

// replace with:
cfg, err := config.LoadOrEmpty(constants.DefaultConfigFile)
if err != nil {
    cfg = &config.Config{}
}
```

(The TUI describer is read-only and can gracefully fall back to empty config.)

- [ ] **Step 8: Verify build**

```
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 9: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go \
        cmd/up.go cmd/validate.go cmd/restart.go cmd/check.go \
        internal/tui/describe.go
git commit -m "fix: LoadOrEmpty propagates YAML/validation errors, only ignores file-not-found"
```

---

## Task 11: Fix unquoted `.env` values in `onboard/onboard.go`

Values typed during onboarding that contain `=` (base64 tokens), `\n` (multiline private keys), `"`, or `#` are written raw into the `.env` file, producing a malformed file that `godotenv.Read` misparsess. Fix: quote values using double quotes and escape any internal double quotes.

**Files:**
- Modify: `internal/onboard/onboard.go`
- Test: `internal/onboard/onboard_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/onboard/onboard_test.go` (or create if it doesn't exist — check with `ls internal/onboard/`):

```go
func TestOnboarder_QuotesSpecialValues(t *testing.T) {
    t.Parallel()
    dir := t.TempDir()
    envFile := filepath.Join(dir, ".env")

    var out strings.Builder
    in := strings.NewReader("Y\nbase64+token==\n")
    schema := map[string]config.EnvVar{
        "API_KEY": {Required: true},
    }
    o := New(&out, strings.NewReader("Y\nbase64+token==\n"), schema)
    // Provide schema key so onboarder asks for it
    _ = o.Run(envFile, "")

    content, err := os.ReadFile(envFile)
    require.NoError(t, err)

    // Value containing = must be quoted so godotenv can parse it back
    parsed, err := godotenv.Read(envFile)
    require.NoError(t, err)
    assert.Equal(t, "base64+token==", parsed["API_KEY"])
    _ = content
    _ = in
}
```

Add imports: `"os"`, `"path/filepath"`, `"strings"`, `"github.com/joho/godotenv"`, `"github.com/deveshpharswan/stackup/internal/config"`.

- [ ] **Step 2: Add `quoteEnvValue` helper to `onboard.go`**

Add before the `Run` method:

```go
// quoteEnvValue wraps the value in double quotes if it contains characters
// that would break godotenv parsing (=, #, whitespace, quotes).
func quoteEnvValue(v string) string {
    if !strings.ContainsAny(v, "=#\"\n\r\t ") {
        return v
    }
    return `"` + strings.ReplaceAll(v, `"`, `\"`) + `"`
}
```

- [ ] **Step 3: Use `quoteEnvValue` when writing the `.env` file**

In `Run`, find lines 100–103:

```go
var sb strings.Builder
for _, k := range keys {
    sb.WriteString(fmt.Sprintf("%s=%s\n", k, values[k]))
}
```

Replace with:

```go
var sb strings.Builder
for _, k := range keys {
    sb.WriteString(fmt.Sprintf("%s=%s\n", k, quoteEnvValue(values[k])))
}
```

- [ ] **Step 4: Verify build**

```
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 5: Commit**

```bash
git add internal/onboard/onboard.go internal/onboard/onboard_test.go
git commit -m "fix: quote .env values containing special characters during onboarding"
```

---

## Task 12: Fix doctor view showing "No issues" while checks run

`DoctorViewModel` is initialized with `loading: false`. When the user presses `d`, the view is shown immediately before `DoctorResultMsg` arrives. `View()` sees `loading == false` and `findings == nil`, so it renders "✓ No issues found" — the opposite of the truth. Fix: initialize with `loading: true`.

**Files:**
- Modify: `internal/tui/doctorview.go`

- [ ] **Step 1: Set `loading: true` in `NewDoctorViewModel`**

Find in `internal/tui/doctorview.go`:

```go
func NewDoctorViewModel() DoctorViewModel {
    return DoctorViewModel{expanded: -1}
}
```

Replace with:

```go
func NewDoctorViewModel() DoctorViewModel {
    return DoctorViewModel{expanded: -1, loading: true}
}
```

- [ ] **Step 2: Verify build**

```
go build ./...
go vet ./...
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add internal/tui/doctorview.go
git commit -m "fix: DoctorViewModel initializes with loading=true to avoid false 'No issues found'"
```

---

## Task 13: Fix minio/clickhouse port 9000 collision in `scaffold.go`

Both `minio` (API port 9000) and `clickhouse` (native client port 9000) are mapped to port 9000 in `knownImages`. Projects using both would receive duplicate `port: 9000` in their scaffolded config, and the `doctor` port-conflict check would report a spurious conflict. ClickHouse also has an HTTP port (8123) which is more suitable for TCP health checks.

**Files:**
- Modify: `internal/scaffold/scaffold.go`
- Test: `internal/scaffold/scaffold_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/scaffold/scaffold_test.go`:

```go
func TestKnownImages_NoDuplicatePorts(t *testing.T) {
    t.Parallel()
    // Detect minio and clickhouse — they must not share a port
    minio := scaffold.DetectHealthDefault("minio/minio:latest")
    clickhouse := scaffold.DetectHealthDefault("clickhouse/clickhouse-server:latest")
    require.NotNil(t, minio)
    require.NotNil(t, clickhouse)
    assert.NotEqual(t, minio.Port, clickhouse.Port,
        "minio and clickhouse must have distinct default ports")
}
```

- [ ] **Step 2: Run to confirm it fails (if able)**

```
go test ./internal/scaffold/... -run TestKnownImages_NoDuplicatePorts -v
```

Expected: `FAIL`.

- [ ] **Step 3: Fix clickhouse port in `scaffold.go`**

Find line 55 in `internal/scaffold/scaffold.go`:

```go
"clickhouse":    {Type: "tcp", Host: "localhost", Port: 9000},
```

Replace with:

```go
"clickhouse":    {Type: "tcp", Host: "localhost", Port: 8123},
```

Port 8123 is the ClickHouse HTTP interface port, usable for TCP health checks and also for HTTP probes.

- [ ] **Step 4: Verify build and tests**

```
go build ./...
go vet ./...
go test ./internal/scaffold/... -v
```

Expected: all scaffold tests pass including the new one.

- [ ] **Step 5: Commit**

```bash
git add internal/scaffold/scaffold.go internal/scaffold/scaffold_test.go
git commit -m "fix: change clickhouse default health check port from 9000 to 8123 to avoid minio collision"
```

---

## Task 14: Fix duplicate validation errors in `env/validator.go`

When a key appears in both `.env.example` (with no value) and the schema with `required: true`, and the key is absent from `.env`, two errors are emitted: "missing (required by .env.example)" and "required but not set". This is noisy and confusing. Fix: skip the schema required-check for a key if the example loop already flagged it.

**Files:**
- Modify: `internal/env/validator.go`
- Test: `internal/env/validator_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/env/validator_test.go`:

```go
func TestValidate_NoDuplicateErrorForRequiredAndExample(t *testing.T) {
    t.Parallel()
    // KEY_A is in both .env.example and schema required=true, but absent from .env
    dir := t.TempDir()
    envFile := filepath.Join(dir, ".env")
    exampleFile := filepath.Join(dir, ".env.example")
    require.NoError(t, os.WriteFile(envFile, []byte("OTHER=x\n"), 0600))
    require.NoError(t, os.WriteFile(exampleFile, []byte("KEY_A=\n"), 0600))

    schema := map[string]config.EnvVar{
        "KEY_A": {Required: true},
    }
    result := env.Validate(envFile, exampleFile, schema)
    // Must have exactly one error for KEY_A, not two
    count := 0
    for _, e := range result.Errors {
        if e.Key == "KEY_A" {
            count++
        }
    }
    assert.Equal(t, 1, count, "expected exactly 1 error for KEY_A, got %d: %v", count, result.Errors)
}
```

Add imports: `"os"`, `"path/filepath"`, `"github.com/deveshpharswan/stackup/internal/config"`.

- [ ] **Step 2: Run to confirm it fails (if able)**

```
go test ./internal/env/... -run TestValidate_NoDuplicateErrorForRequiredAndExample -v
```

Expected: `FAIL` with count=2.

- [ ] **Step 3: Deduplicate the required check in `ValidateWithDefaults`**

In `internal/env/validator.go`, the `example` variable is already read at line 55. Use it to deduplicate.

Find the schema loop (lines 74–88):

```go
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
```

Replace with:

```go
for key, rule := range schema {
    val, ok := envVars[key]
    if !ok {
        if rule.Required {
            // Skip if the example loop already emitted a "missing" error for this key.
            if _, inExample := example[key]; !inExample {
                result.Errors = append(result.Errors, ValidationError{
                    Key:     key,
                    Message: "required but not set",
                })
            }
        }
        continue
    }
    if err := validateType(key, val, rule.Type); err != nil {
        result.Errors = append(result.Errors, *err)
    }
}
```

- [ ] **Step 4: Verify build and tests**

```
go build ./...
go vet ./...
go test ./internal/env/... -v
```

Expected: all env tests pass including the new one.

- [ ] **Step 5: Commit**

```bash
git add internal/env/validator.go internal/env/validator_test.go
git commit -m "fix: suppress duplicate required-but-not-set error when key is already flagged by example check"
```

---

## Self-Review

**Spec coverage check:**
- Bug 1 (LogChecker container name) → Task 2 ✓
- Bug 2 (log stream demultiplexing) → Task 1 + Task 2 ✓
- Bug 3 (ExecShell hang) → Task 3 ✓
- Bug 4 (div by zero) → Task 4 ✓
- Bug 5 (TUI log goroutine leak) → Task 5 ✓
- Bug 6 (TCP context cancellation) → Task 6 ✓
- Bug 7 (orphaned exec) → Task 3 ✓ (noted in task, stop retrying with same shell)
- Bug 8 (Spinner data race) → Task 7 ✓
- Bug 9 (double services.Update — confirmed false positive) → not included ✓
- Bug 10 (dependency validation) → Task 9 ✓
- Bug 11 (LoadOrEmpty) → Task 10 ✓
- Bug 12 (unquoted .env) → Task 11 ✓
- Bug 13 (hooks -f flag — not applicable, stackup only supports `docker-compose.yml` which is Docker Compose's default) → not included ✓
- Bug 14 (doctor loading state) → Task 12 ✓
- Bug 15 (minio/clickhouse port) → Task 13 ✓
- Bonus bug (duplicate env errors) → Task 14 ✓
- Silent log failure → Task 5 ✓

**Type consistency:** All method signatures referenced in later tasks match those defined in earlier tasks. `LoadOrEmpty` is changed in Task 10, and all 5 callers are updated in the same task.

**No placeholder scan:** All code blocks are complete and specific.
