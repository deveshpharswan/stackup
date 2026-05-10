# Stackup UX Fixes Design

## Problem Statement

Stackup fails on projects that follow modern Docker conventions (compose.yaml filename, no .env file), showing interactive prompts and file-not-found errors before the user sees any value. The TUI dashboard shows 0% CPU/memory and broken uptime, undermining trust in the tool.

## Goals

1. Work out-of-the-box on any docker compose project with no stackup.yml required
2. Stop blocking startup when .env is not needed
3. Show a useful TUI (real stats, real uptime)
4. Make the value proposition (health-gated startup) visible even when not yet configured

## Non-Goals

- Rewriting the health check engine
- Supporting remote Docker hosts
- Persisting stats history across TUI sessions

---

## Section 1: Compose File Auto-Discovery

### Problem

`constants.DefaultComposeFile = "docker-compose.yml"` is hardcoded at every call site. Modern Docker uses `compose.yaml` as the canonical name. All 40+ docker/awesome-compose examples use `compose.yaml`.

### Design

**New function** `FindComposeFile(dir string) string` in `internal/constants/paths.go`:

```go
var composeFileNames = []string{
    "compose.yaml",
    "compose.yml",
    "docker-compose.yaml",
    "docker-compose.yml",
}

func FindComposeFile(dir string) string {
    for _, name := range composeFileNames {
        path := filepath.Join(dir, name)
        if _, err := os.Stat(path); err == nil {
            return path
        }
    }
    return ""
}
```

**Persistent `--compose-file` / `-f` flag** on the root cobra command. Stored in a package-level variable `rootComposeFile`. All subcommands read this variable; if empty they call `FindComposeFile(".")`.

**Error when not found**: `"no compose file found (looked for compose.yaml, compose.yml, docker-compose.yaml, docker-compose.yml)"`.

**Call sites to update**:
- `cmd/root.go` — add persistent flag
- `cmd/up.go` — replace `constants.DefaultComposeFile` with resolved path
- `cmd/init.go` — allow init to work when compose file has any supported name
- `internal/doctor/checks.go` — `CheckPortConflicts`, `CheckLocalhostMisuse`, `runningComposeServices`
- `internal/tui/services.go` — `pollServices` shell command
- `internal/tui/stats.go` — `pollStats` shell command (compose project flag)

The resolved compose file path is threaded through as a string argument to functions that need it. No global state beyond the cobra flag variable.

---

## Section 2: .env Gate Guard

### Problem

`NeedsOnboarding` returns true whenever `.env` is missing — regardless of whether the project uses env vars. `ValidateWithDefaults` returns a hard error if `.env` cannot be read, blocking startup even when there is nothing to validate.

### Design

**Change `NeedsOnboarding` signature**:

```go
// NeedsOnboarding returns true only when .env is absent AND there is something
// worth configuring (schema keys or an .env.example file).
func NeedsOnboarding(envFile, exampleFile string, schema map[string]config.EnvVar) bool {
    if _, err := os.Stat(envFile); err == nil {
        return false // .env exists, no onboarding needed
    }
    if len(schema) > 0 {
        return true
    }
    if _, err := os.Stat(exampleFile); err == nil {
        return true
    }
    return false
}
```

**Change `PreFlight` short-circuit**:

```go
func (o *Orchestrator) PreFlight(envFile, exampleFile string, schema map[string]config.EnvVar) (bool, map[string]string) {
    // Nothing to validate: no schema, no example file
    if len(schema) == 0 {
        if _, err := os.Stat(exampleFile); os.IsNotExist(err) {
            return true, nil
        }
    }
    // ... existing validation logic
}
```

The call site in `cmd/up.go` passes `cfg.Env.Schema` and `constants.DefaultExampleFile` to both functions.

---

## Section 3: Zero-Config Mode Hint

### Problem

When stackup.yml has no health checks, the tool silently runs `docker compose up -d` with no visible value add. Users don't know why they're using stackup.

### Design

In `cmd/up.go`, after building `checkers`, if `len(checkers) == 0` and there are services to start, print a single-line hint before starting:

```
  ℹ  No health checks configured — add health: blocks to stackup.yml to enable
     startup sequencing. See: https://github.com/deveshpharswan/stackup#health-checks
```

This fires once, only when there are services but no checkers. It is informational, not a blocker.

---

## Section 4: Uptime Parsing Fix

### Problem

`parseUptime` uses regex `up\s+(?:about\s+)?(\d+)\s*(second|minute|hour|day)` which requires a digit. Docker outputs `"Up About a minute"` and `"Up About an hour"` — no digits — so uptime is always 0, displayed as `"0s"`.

### Design

Fix `parseUptime` with a two-pass approach:

```go
func parseUptime(status string) time.Duration {
    lower := strings.ToLower(status)
    if !strings.Contains(lower, "up") {
        return 0
    }
    // Natural language cases Docker emits for short uptimes
    if strings.Contains(lower, "about a minute") || strings.Contains(lower, "a minute") {
        return time.Minute
    }
    if strings.Contains(lower, "about an hour") || strings.Contains(lower, "an hour") {
        return time.Hour
    }
    // Numeric: "Up 3 minutes", "Up 37 seconds", "Up 2 hours", "Up 5 days"
    re := regexp.MustCompile(`(\d+)\s*(second|minute|hour|day)`)
    matches := re.FindStringSubmatch(lower)
    if len(matches) < 3 {
        return 0
    }
    n, _ := strconv.Atoi(matches[1])
    switch matches[2] {
    case "second":
        return time.Duration(n) * time.Second
    case "minute":
        return time.Duration(n) * time.Minute
    case "hour":
        return time.Duration(n) * time.Hour
    case "day":
        return time.Duration(n) * 24 * time.Hour
    }
    return 0
}
```

Fix `formatUptime` to show `"—"` when duration is zero instead of `"0s"`.

---

## Section 5: CPU/Memory Stats via Docker SDK

### Problem

`pollStats` uses `docker ps --format '{{.Label "com.docker.compose.service"}}'` — broken on Windows due to quote escaping. Then calls `docker stats --no-stream` which takes 1–2s and can mix containers from different compose projects.

### Design

Replace shell-based stats with Docker SDK calls:

**`ServicesModel` gets a `*dockerclient.Client` field** injected at construction time from `tui.go`.

**Hybrid approach**: use `docker compose ps --format "{{.ID}}\t{{.Service}}"` for project-scoped container ID → service name mapping (respects current compose project, no label ambiguity with other running projects), then call the Docker SDK `ContainerStats` per container ID for reliable stats.

**`pollStats` becomes a closure** over the Docker client:

```go
func pollStats(dc *dockerclient.Client) tea.Cmd {
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()

        // Get project-scoped container IDs from compose CLI
        psOut, err := exec.CommandContext(ctx, "docker", "compose", "ps",
            "--format", "{{.ID}}\t{{.Service}}").Output()
        if err != nil {
            return StatsUpdateMsg{Stats: nil}
        }
        idToService := make(map[string]string)
        scanner := bufio.NewScanner(strings.NewReader(string(psOut)))
        for scanner.Scan() {
            parts := strings.SplitN(scanner.Text(), "\t", 2)
            if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
                idToService[parts[0]] = parts[1]
            }
        }
        if len(idToService) == 0 {
            return StatsUpdateMsg{Stats: nil}
        }

        stats := make(map[string]ServiceStats)
        for id, svcName := range idToService {
            resp, err := dc.ContainerStats(ctx, id, false)
            if err != nil {
                continue
            }
            var s types.StatsJSON
            if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
                resp.Body.Close()
                continue
            }
            resp.Body.Close()
            stats[svcName] = ServiceStats{
                CPU:    calcCPUPercent(&s),
                Memory: calcMemPercent(&s),
            }
        }
        return StatsUpdateMsg{Stats: stats}
    }
}
```

CPU and memory percentage formulas from Docker's own source:

```go
func calcCPUPercent(s *types.StatsJSON) float64 {
    cpuDelta := float64(s.CPUStats.CPUUsage.TotalUsage) - float64(s.PreCPUStats.CPUUsage.TotalUsage)
    sysDelta := float64(s.CPUStats.SystemUsage) - float64(s.PreCPUStats.SystemUsage)
    numCPUs := float64(s.CPUStats.OnlineCPUs)
    if numCPUs == 0 {
        numCPUs = float64(len(s.CPUStats.CPUUsage.PercpuUsage))
    }
    if sysDelta > 0 && cpuDelta > 0 {
        return (cpuDelta / sysDelta) * numCPUs * 100.0
    }
    return 0
}

// calcMemPercent handles both cgroups v1 (cache key) and v2 (inactive_file key).
func calcMemPercent(s *types.StatsJSON) float64 {
    if s.MemoryStats.Limit == 0 {
        return 0
    }
    // Prefer inactive_file (cgroups v2); fall back to cache (cgroups v1)
    cacheUsage := s.MemoryStats.Stats["inactive_file"]
    if cacheUsage == 0 {
        cacheUsage = s.MemoryStats.Stats["cache"]
    }
    used := float64(s.MemoryStats.Usage) - float64(cacheUsage)
    return (used / float64(s.MemoryStats.Limit)) * 100.0
}
```

`types.StatsJSON` is from `github.com/docker/docker/api/types`.

The Docker client is created once in `tui.go` (already done for other purposes) and passed into `NewServicesModel(dc)`. The `statsTickEvery` poll interval stays at 5 seconds.

---

## File Map

| File | Change |
|------|--------|
| `internal/constants/paths.go` | Add `FindComposeFile`, `composeFileNames` |
| `cmd/root.go` | Add persistent `--compose-file` / `-f` flag |
| `cmd/up.go` | Thread compose file path; add zero-config hint |
| `cmd/init.go` | Use resolved compose file path |
| `internal/onboard/onboard.go` | Update `NeedsOnboarding` signature |
| `internal/orchestrator/orchestrator.go` | Add `PreFlight` short-circuit |
| `internal/tui/services.go` | Pass compose file to `pollServices`; accept `*dockerclient.Client` |
| `internal/tui/stats.go` | Replace shell stats with Docker SDK |
| `internal/tui/tui.go` | Pass Docker client to `NewServicesModel` |
| `internal/doctor/checks.go` | Thread compose file through check functions |
| `cmd/doctor.go` | Pass compose file flag to Options |

## Testing

- Unit tests for `FindComposeFile` (each of the 4 filenames, none found)
- Unit tests for `NeedsOnboarding` (all combinations of .env present/absent, schema/no-schema, example/no-example)
- Unit tests for `parseUptime` covering all Docker output patterns
- Unit tests for `calcCPUPercent` / `calcMemPercent`
- Integration tests for PreFlight short-circuit (skip when nothing to validate)
