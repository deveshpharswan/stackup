# Stackup

Smart Docker Compose orchestration for development teams. Wraps your existing `docker-compose.yml` with health-gated startup, `.env` validation, and debug workflows — all configured via a `stackup.yml` sidecar file.

## The Problem

Working with Docker Compose in development is painful in ways that compound daily:

1. **Services crash on startup.** Your API starts before Postgres accepts connections. Docker Compose `depends_on` only waits for the container to *start*, not for the service inside to be *ready*. You get connection refused errors, restart loops, and wasted time waiting for things to stabilize.

2. **Environment drift breaks things silently.** A teammate adds `STRIPE_KEY` to the code but forgets to tell you. Your app boots, hits the Stripe call 10 minutes later, and blows up. No one diffs `.env.example` manually every morning.

3. **Debug workflows are friction-heavy.** To check logs you need `docker compose logs -f api`. To get a shell: `docker compose exec api bash`. To restart one service: `docker compose restart api` — then hope it's healthy. Each command requires remembering the exact service name and flags.

4. **No single source of truth for service health.** Some services need an HTTP health endpoint, others just need a TCP port open, others use Docker's native `HEALTHCHECK`. There's no standard place to define "what does healthy mean?" for each service in your stack.

5. **Onboarding is slow.** New developers clone the repo, run `docker compose up`, watch it crash, ask Slack what they're missing, get told to check `.env.example`, copy it, fill in values, restart, hit another error. Stackup makes this a single command with clear feedback.

## How Stackup Solves This

### Health-Gated Startup

Services are grouped into **tiers** based on `depends_on` in your compose file. Stackup starts each tier and **waits for health checks to pass** before proceeding to the next. Your API doesn't start until Postgres and Redis are genuinely accepting connections.

```
Tier 0: postgres, redis     ← start first, wait until healthy
Tier 1: api                 ← only starts after tier 0 is confirmed healthy
Tier 2: web                 ← only starts after tier 1 is confirmed healthy
```

Three health check strategies:
- **HTTP** — polls a URL until it returns 2xx (perfect for APIs with `/health` endpoints)
- **TCP** — attempts a socket connection until the port accepts (databases, caches)
- **Docker** — reads the container's native `HEALTHCHECK` status (when the image defines one)

Each check retries at a configurable interval with a configurable timeout. If a service doesn't become healthy in time, Stackup stops immediately and tells you exactly which service failed and why.

### Pre-Flight Environment Validation

Before any container starts, Stackup:

1. **Diffs `.env` against `.env.example`** — catches missing keys instantly
2. **Validates types** (optional) — ensures `PORT` is actually a number, `DATABASE_URL` is a valid URL, `DEBUG` is a boolean

This means environment problems surface in 100ms with a clear error message, not 30 seconds later as a cryptic stack trace buried in container logs.

```
$ stackup up

→ Pre-flight
  ✗ API_KEY: missing (required by .env.example)
  ✗ PORT: expected int, got "abc"

Error: pre-flight validation failed
```

### Developer Debug Commands

Stackup provides shortcuts that work by service name from your compose file:

| Command | What it replaces | Added value |
|---------|-----------------|-------------|
| `stackup logs api` | `docker compose logs -f api` | Simpler syntax |
| `stackup shell api` | `docker compose exec api bash` | Auto-detects bash/sh |
| `stackup restart api` | `docker compose restart api` | Re-runs health check after restart, confirms service is healthy |
| `stackup run seed` | `docker compose exec api npm run db:seed` | Named commands in config, no need to remember service + command |

### Scaffold Generator

`stackup init` reads your existing `docker-compose.yml` and `.env.example` and generates a starter `stackup.yml` with all services and env vars pre-filled. You just need to set the correct health check type for each service.

### Zero Config Baseline

Stackup works without a `stackup.yml` — it still diffs `.env` against `.env.example`. The sidecar config only adds health checks, type validation, and custom commands. You can adopt it incrementally.

## Example Output

```
$ stackup up

→ Pre-flight
  ✓ .env validated (4 keys, 0 missing)
  ✓ DATABASE_URL — valid url
  ✓ PORT — valid int

→ Starting tier
  ✓ postgres     healthy  [tcp:5432]  2.3s
  ✓ redis        healthy  [docker]    1.1s

→ Starting tier  (depends on: postgres, redis)
  ✓ api          healthy  [http:http://localhost:8080/health]  4.7s

✓ Stack ready in 8.1s
```

## Quick Start

```bash
# Install (requires Go 1.22+)
go install github.com/stackup-dev/stackup@latest

# Generate a starter config from your existing compose file
stackup init

# Review and customise health checks in the generated stackup.yml
# Then validate environment and start everything
stackup up
```

## Commands

| Command | Description |
|---------|-------------|
| `stackup up` | Validate `.env`, then start services in health-gated tier order |
| `stackup down` | Stop and remove all containers (runs `docker compose down --remove-orphans`) |
| `stackup validate` | Check `.env` without starting services |
| `stackup status` | Show container states (service, state, status) |
| `stackup init` | Generate `stackup.yml` from `docker-compose.yml` and `.env.example` |
| `stackup logs <svc>` | Stream logs for a service (`-f` to follow) |
| `stackup shell <svc>` | Open an interactive shell (tries `bash`, falls back to `sh`) |
| `stackup restart <svc>` | Restart a service and re-run its health check before reporting success |
| `stackup run <cmd>` | Run a named command from `stackup.yml` inside its configured container |
| `stackup version` | Print version, commit SHA, and build date |

## Configuration

Create a `stackup.yml` alongside your `docker-compose.yml`:

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
    DEBUG:
      type: bool

services:
  postgres:
    health:
      type: tcp
      host: localhost
      port: 5432
      timeout: 30s
      interval: 2s
  api:
    health:
      type: http
      url: http://localhost:8080/health
      timeout: 60s
      interval: 3s
  redis:
    health:
      type: docker
      timeout: 20s

commands:
  seed:
    service: api
    run: "npm run db:seed"
  migrate:
    service: api
    run: "npm run db:migrate"
```

### Health Check Types

| Type | Description | Required fields | Optional fields |
|------|-------------|-----------------|-----------------|
| `http` | Polls a URL until it returns a 2xx status code | `url` | `timeout`, `interval` |
| `tcp` | Attempts TCP connection until a port accepts | `host`, `port` | `timeout`, `interval` |
| `docker` | Reads Docker's native `HEALTHCHECK` status from container inspect | — | `timeout`, `interval` |

**Defaults:**
- `timeout`: 30s (how long to keep retrying before declaring failure)
- `interval`: 2s (pause between retry attempts)

### Env Schema

The `env.schema` section is optional. If omitted, Stackup still diffs `.env` against `.env.example` to catch missing keys — it just won't validate types.

#### Schema Fields

| Field | Type | Description |
|-------|------|-------------|
| `type` | string | Validation type: `int`, `bool`, or `url` |
| `required` | bool | If `true`, startup fails when the key is missing from `.env` |
| `default` | string | Documented default value (informational, not injected) |

#### Supported Types

| Type | Valid examples | Invalid examples |
|------|---------------|------------------|
| `int` | `3000`, `0`, `-1` | `abc`, `3.14`, `` (empty) |
| `bool` | `true`, `false`, `1`, `0` | `yes`, `no`, `on` |
| `url` | `postgres://localhost:5432/db`, `http://api.example.com` | `not-a-url`, `/relative/path` |

### Commands Section

Define named commands that run inside a specific service container:

```yaml
commands:
  seed:
    service: api          # Which compose service to exec into
    run: "npm run db:seed" # Command to execute
```

Run with: `stackup run seed`

## How It Works

```
stackup up
│
├── 1. Pre-flight validation
│   ├── Read .env and .env.example
│   ├── Report any keys in .env.example missing from .env
│   └── If schema defined: validate each key's type
│       └── STOP on first validation failure
│
├── 2. Parse docker-compose.yml
│   ├── Extract service names
│   └── Extract depends_on relationships
│
├── 3. Build startup tiers (topological sort)
│   ├── Tier 0: services with no dependencies (e.g., postgres, redis)
│   ├── Tier 1: services depending only on tier 0 (e.g., api)
│   └── Tier N: services depending on tiers 0..N-1
│
└── 4. Start each tier sequentially
    ├── Run: docker compose up -d <services in tier>
    ├── Poll health checks for each service in the tier
    │   ├── Success → print "healthy" with elapsed time
    │   └── Timeout → print "failed", STOP immediately
    └── Once all healthy → proceed to next tier
```

### Failure Behaviour

- **Missing `.env` key:** Pre-flight fails, no containers start
- **Invalid type:** Pre-flight fails, no containers start
- **Health check timeout:** The failing service is reported, startup aborts. Already-running containers from previous tiers remain up (use `stackup down` to clean up)
- **Cycle in dependencies:** Detected during graph build, reported before any containers start

## `stackup init`

Generates a starter `stackup.yml` by inspecting your project:

1. Reads `docker-compose.yml` → extracts all service names
2. Reads `.env.example` → lists all env var keys as `required: true`
3. Outputs a `stackup.yml` with placeholder health checks (`type: tcp`)

You then edit the generated file to set correct health check types, URLs, and ports for each service.

**Will not overwrite** an existing `stackup.yml` — delete it first if you want to regenerate.

## Building from Source

```bash
# Build binary with embedded version info
make build
# Output: ./stackup (or stackup.exe on Windows)

# Run linter
make lint

# Run e2e tests (requires Docker daemon running)
make e2e

# Clean build artifacts
make clean
```

### Manual build without Make

```bash
go build -ldflags "-X main.version=0.1.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%d)" -o stackup .
```

## Prerequisites

- **Go 1.22+** (for building from source)
- **Docker Engine** with the `docker compose` CLI plugin (v2)
- A `docker-compose.yml` in your project root

## Project Structure

```
main.go                 Entry point (version/commit/date injected via ldflags)
cmd/                    Cobra CLI command definitions
internal/
  config/               stackup.yml YAML parser and types
  env/                  .env validation engine (diff + type checking)
  orchestrator/         Dependency graph (Kahn's algorithm) + health-gated startup
  health/               Health check implementations (HTTP, TCP, Docker)
  docker/               Docker SDK wrapper (exec, logs, restart, inspect)
  printer/              Coloured terminal progress output
  scaffold/             stackup init generator + compose file parser
test/e2e/               End-to-end integration tests (build tag: e2e)
testdata/               Shared test fixtures
Makefile                Build, test, lint commands
```

## License

MIT
