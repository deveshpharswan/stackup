# Stackup

Smart Docker Compose orchestration for development teams.

Stackup wraps your existing `docker-compose.yml` with health-gated startup, `.env` validation, automated diagnostics, and developer shortcuts — configured via a `stackup.yml` sidecar file.

## Why

Docker Compose `depends_on` only waits for containers to *start*, not for services to be *ready*. Your API connects to Postgres before it accepts connections. You get cryptic errors, restart loops, and lost time.

Stackup fixes this by grouping services into tiers and waiting for health checks to pass before starting the next tier.

## Install

```bash
go install github.com/deveshpharswan/stackup@latest
```

Or build from source:

```bash
git clone https://github.com/deveshpharswan/stackup.git
cd stackup
go build -ldflags "-X main.version=dev" -o stackup .
```

## Quick Start

```bash
# Generate config from your existing compose file
stackup init

# Edit stackup.yml to set correct health check types
# Then start everything with validation
stackup up
```

## What Happens When You Run `stackup up`

```text
$ stackup up

→ Pre-flight
  ✓ .env validated (4 keys, 0 missing)
  ✓ DATABASE_URL — valid url
  ✓ PORT — valid int

→ Starting tier
  ✓ postgres     healthy  [tcp:5432]  2.3s
  ✓ redis        healthy  [tcp:6379]  1.1s

→ Starting tier  (depends on: postgres, redis)
  ✓ api          healthy  [http:http://localhost:8080/health]  4.7s
    → hook: Run migrations
    ✓ Run migrations

✓ Stack ready in 8.1s
```

If something fails, Stackup shows you why immediately:

```text
  ✗ api          failed: http check timed out after 60s
  ┌── logs: api ──
  │ Error: Cannot connect to postgres: ECONNREFUSED 127.0.0.1:5432
  └────

  ⚠ Services still running: postgres, redis
    To clean up: stackup down
  Try:
    • stackup doctor
    • stackup logs api
```

## Commands

| Command | Description |
| ------- | ----------- |
| `stackup up` | Validate env, start services in health-gated tier order |
| `stackup down` | Stop all containers |
| `stackup validate` | Check `.env` without starting services |
| `stackup status` | Show running container states |
| `stackup doctor` | Automated diagnostics (port conflicts, crash loops, env drift) |
| `stackup check` | CI-friendly health check (exit 0 = healthy, exit 2 = unhealthy) |
| `stackup init` | Generate `stackup.yml` from your compose file |
| `stackup logs <svc>` | Stream logs for a service |
| `stackup shell <svc>` | Open interactive shell in a container |
| `stackup restart <svc>` | Restart and re-check health |
| `stackup run <cmd>` | Run a named command from config |

## Configuration

Create `stackup.yml` alongside your `docker-compose.yml`:

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
  api:
    health:
      type: http
      url: http://localhost:8080/health
      timeout: 60s
  redis:
    health:
      type: docker
      timeout: 20s

commands:
  seed:
    service: api
    run: "npm run db:seed"
```

## Health Check Types

| Type | Use Case | Required Fields |
| ---- | -------- | --------------- |
| `http` | APIs with health endpoints | `url` |
| `tcp` | Databases, caches | `host`, `port` |
| `docker` | Images with built-in HEALTHCHECK | — |
| `log` | Services that log readiness | `pattern` |

```yaml
# Log-based health check example (for older databases)
postgres:
  health:
    type: log
    pattern: "database system is ready to accept connections"
    timeout: 30s
```

## Diagnostics

```text
$ stackup doctor

  ✗ Port 5432 is already in use
    Service "postgres" expects port 5432 but it is occupied
    Fix: lsof -i :5432

  ⚠ Environment drift detected
    Keys in .env.example but not in .env: STRIPE_KEY, NEW_RELIC_KEY
    Fix: Add missing keys to .env

  ⚠ Localhost reference in DATABASE_URL may not work inside containers
    Found "localhost:5432" — inside Docker, use the service name "postgres" instead
    Fix: Replace localhost:5432 with postgres:5432 in DATABASE_URL
```

## CI Usage

```bash
stackup up
stackup check --quiet    # exit 0 if healthy, exit 2 if not
stackup check --format json --service postgres
```

## First-Time Onboarding

When `.env` doesn't exist, `stackup up` automatically walks new developers through creating it:

```text
Welcome to Stackup! It looks like you don't have a .env file yet.

The following environment variables are needed:
  DATABASE_URL (example: postgres://localhost/db)
  PORT [default: 3000]

Create your .env now? [Y/n]
```

## Project Structure

```text
main.go                 Entry point
cmd/                    CLI commands (Cobra)
internal/
  config/               stackup.yml parser
  constants/            Shared path constants
  env/                  .env validation (diff + type checking)
  orchestrator/         Dependency graph + health-gated startup
  health/               Health checkers (HTTP, TCP, Docker, Log)
  docker/               Docker SDK wrapper
  doctor/               Automated diagnostic checks
  hooks/                Lifecycle hook executor
  onboard/              First-run interactive setup
  printer/              Terminal output formatting
  scaffold/             stackup init generator
```

## Prerequisites

- Go 1.22+ (build from source)
- Docker Engine with `docker compose` v2
- A `docker-compose.yml` in your project

## License

MIT
