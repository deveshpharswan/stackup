# Stackup

**Stop debugging Docker startup failures. Start building.**

[Install](#install) · [Quick Start](#quick-start) · [Commands](#commands) · [Configuration](#configuration)

---

## The Problem

You run `docker compose up`. Your API tries to connect to Postgres — but Postgres isn't ready yet. You get:

```text
Error: ECONNREFUSED 127.0.0.1:5432
```

Docker Compose `depends_on` only waits for containers to **start**, not for services to be **ready**. So every morning you restart things, wait, check logs, restart again. New team members spend their first day fighting this instead of shipping code.

## The Fix

Stackup wraps your existing `docker-compose.yml` and adds what's missing:

- **Health-gated startup** — services start only after their dependencies are actually ready (not just running)
- **Environment validation** — catches missing `.env` keys before anything starts
- **Failure diagnostics** — when something breaks, tells you exactly why and how to fix it
- **Interactive TUI** — a real-time dashboard for your entire stack (like k9s for your dev environment)

No changes to your `docker-compose.yml` required. Add a `stackup.yml` sidecar and go.

---

## Install

### macOS / Linux (quick)

```bash
curl -sSL https://raw.githubusercontent.com/deveshpharswan/stackup/master/install.sh | sh
```

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/deveshpharswan/stackup/master/install.ps1 | iex
```

### macOS (Homebrew)

```bash
brew install deveshpharswan/tap/stackup
```

### Windows (Scoop)

```powershell
scoop bucket add stackup https://github.com/deveshpharswan/scoop-stackup
scoop install stackup
```

### Manual Download

Download the binary for your platform from [GitHub Releases](https://github.com/deveshpharswan/stackup/releases/latest), extract it, and add it to your PATH.

| Platform | File |
| -------- | ---- |
| macOS (Apple Silicon) | `stackup_*_darwin_arm64.tar.gz` |
| macOS (Intel) | `stackup_*_darwin_amd64.tar.gz` |
| Linux (x64) | `stackup_*_linux_amd64.tar.gz` |
| Linux (ARM) | `stackup_*_linux_arm64.tar.gz` |
| Windows (x64) | `stackup_*_windows_amd64.zip` |
| Windows (ARM) | `stackup_*_windows_arm64.zip` |

### From Source

```bash
go install github.com/deveshpharswan/stackup@latest
```

> Note: `stackup version` shows "dev" when built this way. Use a released binary for proper version info.

### Verify

```bash
stackup version
```

> **Requires:** Docker Desktop (Windows/macOS) or Docker Engine (Linux) with `docker compose` v2.

---

## Quick Start

**1. You already have a `docker-compose.yml`** — Stackup works alongside it.

**2. Generate a config:**

```bash
stackup init
```

This creates `stackup.yml` by detecting your services and their health check types automatically (Postgres → TCP 5432, Redis → TCP 6379, etc).

**3. Start your stack:**

```bash
stackup up
```

That's it. Stackup handles the rest.

---

## What It Looks Like

### Successful startup

```text
$ stackup up

→ Pre-flight
  ✓ .env validated (4 keys, 0 missing)
  ✓ DATABASE_URL — valid url
  ✓ PORT — valid int

→ Starting tier 1
  ✓ postgres     healthy  [tcp:5432]         2.3s
  ✓ redis        healthy  [tcp:6379]         1.1s

→ Starting tier 2  (depends on: postgres, redis)
  ✓ api          healthy  [http:8080/health] 4.7s
    → hook: Run migrations ✓

✓ Stack ready in 8.1s
```

### When something fails

```text
$ stackup up

→ Starting tier 2
  ✗ api          failed — http check timed out after 60s

  --- logs: api ---------------------------------
  Error: Cannot connect to postgres: ECONNREFUSED
  at TCPConnectWrap.afterConnect (net.js:1141:16)
  -----------------------------------------------

  ⚠ Services still running: postgres, redis
    To clean up: stackup down

  Try:
    • stackup doctor    — run diagnostics
    • stackup logs api  — see full logs
```

No more guessing. You see the error, the context, and the next step.

---

## Interactive TUI

```bash
stackup ui
```

A real-time terminal dashboard (like lazydocker / k9s) for your dev stack:

```text
 Stackup                              5 services | 3 tiers
------------------------------------------------------------
 SERVICE       STATUS      HEALTH     UPTIME
 postgres      running     healthy    2h 15m
 redis         running     healthy    2h 15m
 api           running     healthy    1h 03m
 worker        running     healthy    1h 03m
 nginx         running     healthy    58m

 [enter] logs  [r] restart  [d] describe  [g] graph  [?] help
------------------------------------------------------------
```

**Features:**

- Live service status with health indicators
- CPU/memory sparklines per service (real-time resource usage)
- Error zoom — press `e` to show only unhealthy services
- Stream logs for any service (with color-coded output)
- Restart services and re-run health checks
- Dependency graph visualization
- Doctor diagnostics panel
- Filter services with `/`
- Keyboard-driven — no mouse needed

---

## Commands

| Command | What it does |
| ------- | ------------ |
| `stackup up` | Validate env → start services in health-gated order |
| `stackup up --only api,db` | Start only named services and their dependencies |
| `stackup down` | Stop all containers |
| `stackup ui` | Open interactive terminal dashboard |
| `stackup status` | Show running container states |
| `stackup doctor` | Run diagnostics (port conflicts, env drift, crash loops) |
| `stackup check` | CI health check — exit 0 if healthy, exit 2 if not |
| `stackup init` | Auto-generate `stackup.yml` from your compose file |
| `stackup validate` | Check `.env` without starting services |
| `stackup logs <svc>` | Stream logs for a service (`-f` to follow) |
| `stackup shell <svc>` | Open interactive shell inside a container |
| `stackup restart <svc>` | Restart a service and re-run its health check |
| `stackup run <cmd>` | Run a named command defined in config |

---

## Configuration

Create `stackup.yml` next to your `docker-compose.yml`:

```yaml
version: "1"

env:
  schema:
    DATABASE_URL:
      type: url
      required: true
    PORT:
      type: int
      default: "3000"
    LOG_LEVEL:
      type: string
      default: "info"

services:
  postgres:
    health:
      type: tcp
      host: localhost
      port: 5432
      timeout: 30s
    hooks:
      after_start:
        - name: "Run migrations"
          service: api
          run: "npm run db:migrate"

  redis:
    health:
      type: tcp
      host: localhost
      port: 6379

  api:
    health:
      type: http
      url: http://localhost:8080/health
      timeout: 60s

commands:
  seed:
    service: api
    run: "npm run db:seed"
  test:
    service: api
    run: "npm test"
```

### Health Check Types

| Type | When to use | Config |
| ---- | ----------- | ------ |
| `tcp` | Databases, caches, message brokers | `host` + `port` |
| `http` | APIs with health endpoints | `url` |
| `docker` | Images with built-in `HEALTHCHECK` | — (uses Docker's native check) |
| `log` | Services that print readiness to stdout | `pattern` (regex) |

### Log-Based Health Check Example

For services that don't expose a port but log when they're ready:

```yaml
services:
  worker:
    health:
      type: log
      pattern: "ready to accept connections"
      timeout: 30s
```

---

## Diagnostics

```bash
stackup doctor
```

```text
$ stackup doctor

  ✗ Port 5432 is already in use
    Service "postgres" expects port 5432 but it is occupied
    Fix: lsof -i :5432

  ⚠ Environment drift detected
    Keys in .env.example but not in .env: STRIPE_KEY, NEW_RELIC_KEY
    Fix: Add missing keys to .env

  ⚠ Localhost in DATABASE_URL won't work inside containers
    Found "localhost:5432" — use service name "postgres" instead
    Fix: Replace localhost:5432 with postgres:5432

  ✓ No crash loops detected
  ✓ All health check ports are reachable
```

---

## Team Onboarding

When a new developer clones the repo and runs `stackup up` without a `.env` file, Stackup walks them through setup interactively:

```text
Welcome to Stackup! You don't have a .env file yet.

The following variables are needed:
  DATABASE_URL  (example: postgres://localhost/db)
  PORT          [default: 3000]
  LOG_LEVEL     [default: info]

Create .env now? [Y/n]
```

No more Slack messages asking "what goes in `.env`?"

---

## CI Usage

```yaml
# In your CI pipeline
- run: stackup up
- run: stackup check --quiet
# exit 0 = all healthy, exit 2 = something's wrong

# Check specific service as JSON
- run: stackup check --format json --service postgres
```

---

## How It Works

1. **Reads** your `docker-compose.yml` to understand service dependencies
2. **Validates** `.env` against the schema in `stackup.yml`
3. **Sorts** services into startup tiers using topological sort
4. **Starts** each tier and polls health checks in parallel
5. **Waits** for all checks in a tier to pass before starting the next
6. **Runs hooks** (like migrations) after services become healthy
7. **Reports** failures with logs and suggested fixes

---

## Prerequisites

- **Windows/macOS:** Docker Desktop (includes `docker compose` v2)
- **Linux:** Docker Engine with `docker compose` v2 plugin
- A `docker-compose.yml` in your project
- Go 1.23+ (only if building from source)

## License

MIT
