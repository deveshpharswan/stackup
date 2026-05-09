# stackup — Improvements & Future Features

> This document covers two things: fixes to the current v1.0 plan, and genuinely new ideas that go beyond what was originally scoped. Not all of these should be built. Read them as a menu, not a roadmap.

---

## Part 1 — Improvements to What Already Exists

These are gaps in the current plan that should be fixed before shipping.

---

### 1.1 Surface container logs on health check failure

**Current**: When a service times out, stackup says "timeout after 60s" and stops.  
**Problem**: The developer still has to run `stackup logs api` manually to find out why.  
**Fix**: When a health check fails, automatically print the last 20 lines of that container's logs in the error output.

```
✗ api  timeout after 60s — no response at http://localhost:8080/health

  Last 20 lines from api:
  ──────────────────────────────────────────
  Error: Cannot connect to postgres: ECONNREFUSED 127.0.0.1:5432
  Error: Cannot connect to postgres: ECONNREFUSED 127.0.0.1:5432
  ──────────────────────────────────────────

  Try:
    stackup doctor     # automated diagnosis
    stackup logs api   # full log history
```

This alone eliminates the most common "what went wrong?" debugging loop.

---

### 1.2 Lifecycle hooks (after_start, before_start, on_failure)

**Current**: The `commands` section lets users manually run named commands with `stackup run`. There's no automatic execution tied to service lifecycle.  
**Problem**: Migrations, seed data, cache warmup — all things that logically belong to "postgres just became healthy" — still require manual steps. The README still needs a "now run this" section.  
**Fix**: Add lifecycle hooks to `stackup.yml`.

```yaml
services:
  postgres:
    health:
      type: tcp
      port: 5432
    hooks:
      after_start:
        - service: api
          run: "npm run db:migrate"
        - service: api
          run: "npm run db:seed"
      on_failure:
        - run: "docker compose logs postgres"
```

Hooks run inside a named container, in order, with output shown inline in the startup display. This is what makes stackup feel magical instead of just useful.

---

### 1.3 Named tiers with explicit grouping (optional)

**Current**: Tiers are derived automatically from `depends_on`. The startup output says "Starting tier 0" with no meaningful label.  
**Fix**: Allow optional named tiers for teams that want cleaner output.

```yaml
tiers:
  - name: "Infrastructure"
    services: [postgres, redis, kafka]
  - name: "Application"
    services: [api, worker, scheduler]
  - name: "Frontend"
    services: [web]
```

Startup output becomes:
```
→ Infrastructure (tier 0)
  ✓ postgres  healthy  2.3s
  ✓ redis     healthy  1.1s

→ Application (tier 1)
  ✓ api       healthy  4.7s
```

If no tiers are defined, auto-derive from `depends_on` as today.

---

### 1.4 Log strategy for health checks

**Current**: Health check types are `http`, `tcp`, `docker`. Many real databases (mysql, mongodb, older postgres images) don't have a reliable TCP window or a Docker HEALTHCHECK. The only way to know they're ready is a specific log line.  
**Fix**: Add a `log` strategy.

```yaml
services:
  postgres:
    health:
      type: log
      pattern: "database system is ready to accept connections"
      timeout: 30s
```

This is how many battle-tested tools (wait-for-it, dockerize) work. It handles the cases where TCP lies and HTTP doesn't exist.

---

### 1.5 The `default` field in env schema needs a decision

**Current**: `default: "3000"` exists in the schema but is informational only — stackup doesn't inject it.  
**Problem**: Developers will assume stackup sets the default. They'll remove it from `.env`, get a validation error, and be confused.  
**Fix**: Either inject defaults (stackup writes the value into the process environment if the key is missing), or remove the `default` field entirely. Pick one. Injecting defaults is more useful but requires clear documentation that it's ephemeral (not written back to `.env`).

---

### 1.6 Auto-cleanup suggestion after failed startup

**Current**: When startup fails mid-way, already-running containers stay up. The developer runs `stackup up` again and hits port conflicts.  
**Fix**: On failure, print a clear recovery suggestion at the bottom:

```
✗ Startup failed at tier 1

  Tier 0 services (postgres, redis) are still running.
  To clean up before retrying: stackup down
```

---

## Part 2 — New Features That Solve Real Problems

These don't exist in the current plan. They're worth considering seriously.

---

### 2.1 `stackup doctor` — Automated Diagnostics

This was in the original product spec but is absent from the README. It's the feature most likely to earn word-of-mouth from developers.

`stackup doctor` runs a set of checks against the current state and tells you what's broken and how to fix it — without you having to know what to look for.

```
$ stackup doctor

  stackup doctor — my-saas-app

  Checking 4 services...

  ✗ PORT CONFLICT — postgres (5432)
    Port 5432 is held by: postgres (PID 3421) — a local process, not Docker
    Fix: sudo kill 3421  OR  change the port mapping in docker-compose.yml

  ✗ CRASH LOOP — api
    Container has restarted 7 times in the last 2 minutes
    Last error: Cannot connect to database at 127.0.0.1:5432
    Note: Inside a container, localhost ≠ your host. Use the service name: postgres:5432
    Fix: Change DATABASE_URL to: postgres://user:pass@postgres:5432/testdb

  ✓ redis     healthy
  ✓ postgres  healthy (when not conflicting with local process)

  ⚠ ENV DRIFT — 2 new vars in .env.example not in your .env
    Missing: STRIPE_KEY, NEW_RELIC_LICENSE_KEY
    Fix: Add these to your .env (ask a teammate for values)

  2 issues found. Fix them and re-run: stackup up
```

Checks to implement:
- Port conflicts (host-level, not just Docker)
- Crash loop detection (restart count from Docker SDK)
- The "localhost inside a container" mistake (extremely common for beginners)
- Missing .env keys vs .env.example
- Container exited immediately (image pull failure, bad command)
- Health check defined in stackup.yml but not matching what the container actually exposes

---

### 2.2 `stackup check` — CI-Friendly Exit Codes

A composable health check for use in CI pipelines.

```bash
# In your CI script:
stackup up
stackup check  # exits 0 if all healthy, 2 if any service is unhealthy
if [ $? -ne 0 ]; then
  echo "Stack unhealthy, aborting tests"
  exit 1
fi
```

Flags:
- `stackup check --service postgres` — check a single service
- `stackup check --format json` — machine-readable output for CI integrations
- `stackup check --quiet` — no output, just exit code

This makes stackup useful in test environments and staging pre-flight checks, not just local dev.

---

### 2.3 `stackup snapshot` — Environment Drift Detection

Saves a baseline of the current environment state (env keys, service configs) and lets you compare later.

```bash
stackup snapshot save          # save current state as baseline
stackup snapshot diff          # compare current state to baseline
```

```
stackup snapshot diff

  Changes since last snapshot (3 days ago):

  ENV — 2 added, 1 removed
  + STRIPE_WEBHOOK_SECRET  (added by: teammate — ask them for the value)
  + FEATURE_FLAG_NEW_UI
  - OLD_ANALYTICS_KEY      (removed — safe to delete from your .env)

  SERVICES — no changes

  Run: stackup env to validate your current .env against requirements
```

The baseline is stored in `.stackup/` (gitignored). The diff only compares key names, not values — so it's safe even if `.env` contains real secrets.

---

### 2.4 Per-environment profiles

Real projects have multiple environments: `dev`, `test`, `staging`. They share the same services but with different configs (different DB names, ports, env files).

```yaml
# stackup.yml
profiles:
  dev:
    env:
      file: .env
  test:
    env:
      file: .env.test
    services:
      postgres:
        health:
          timeout: 10s  # tighter timeout in test
  staging:
    env:
      file: .env.staging
```

```bash
stackup up                    # uses default (dev)
stackup up --profile test     # uses test profile
stackup check --profile test  # useful in CI
```

This replaces the common pattern of maintaining multiple `docker-compose.override.yml` files.

---

### 2.5 Team onboarding mode

When a new developer clones the repo and runs `stackup up` for the first time, stackup detects that `.env` doesn't exist yet and walks them through setup interactively.

```
$ stackup up

  Welcome to my-saas-app!

  First time setup — your .env is missing.

  Required environment variables:
  ──────────────────────────────────────────
  DATABASE_URL    postgres connection string
                  example: postgres://user:pass@localhost:5432/mydb

  STRIPE_KEY      Stripe secret key
                  get it from: https://dashboard.stripe.com/apikeys

  PORT            API port (default: 3000)
  ──────────────────────────────────────────

  Create your .env now? [Y/n] Y

  DATABASE_URL: postgres://user:pass@localhost:5432/testdb
  STRIPE_KEY: sk_test_...
  PORT: (press enter for default: 3000)

  ✓ .env created
  → Starting stack...
```

This replaces the "copy .env.example and fill in values" step in every CONTRIBUTING.md ever written. Pair it with documentation links in `stackup.yml`:

```yaml
env:
  schema:
    STRIPE_KEY:
      type: string
      required: true
      description: "Stripe secret key"
      docs: "https://dashboard.stripe.com/apikeys"
```

---

### 2.6 Service dependency graph visualiser

```bash
stackup graph
```

Renders an ASCII or Mermaid diagram of the dependency graph:

```
stackup graph

  my-saas-app dependency graph

  postgres ──┐
             ├──► api ──► web
  redis    ──┘
             └──► worker

  Tier 0: postgres, redis
  Tier 1: api
  Tier 2: web, worker
```

Useful for understanding large stacks, and for debugging why a service isn't starting (maybe its dependency is in the wrong tier).

---

### 2.7 `stackup update` — Self-update

```bash
stackup update
```

Checks GitHub releases, downloads the new binary, replaces itself. Standard for Go CLIs. Zero friction upgrades.

---

### 2.8 Pre-flight hooks

Beyond environment validation, let teams run arbitrary checks before any container starts:

```yaml
preflight:
  - name: "Docker daemon running"
    cmd: "docker info"
  - name: "Required ports free"
    check: ports_free [5432, 6379, 8080]
  - name: "Sufficient disk space"
    check: disk_free_gb 2
```

This is particularly useful for teams where new developers frequently hit "not enough disk space for Docker images" or "Docker daemon not running" and get confusing errors.

---

### 2.9 Failure playbooks

When a specific service fails, teams often have a known set of fixes. Let them encode this knowledge directly:

```yaml
services:
  postgres:
    on_failure:
      message: |
        postgres failed to start. Common causes:

        1. Port conflict: lsof -i :5432 to find what's using it
        2. Data directory corruption: rm -rf ./data/postgres (loses data)
        3. Wrong password: check POSTGRES_PASSWORD in .env vs DATABASE_URL
      run: "stackup doctor"
```

Instead of "health check timeout", the developer gets the team's institutional knowledge about that specific service.

---

### 2.10 Structured output for tooling integrations

```bash
stackup status --format json
stackup check --format json
```

```json
{
  "stack": "my-saas-app",
  "healthy": false,
  "services": [
    { "name": "postgres", "status": "healthy", "uptime": "2m34s" },
    { "name": "api", "status": "unhealthy", "restarts": 3 }
  ]
}
```

Enables integrations with:
- IDE extensions (show stack status in the status bar)
- Slack bots (ping team when staging stack goes unhealthy)
- Monitoring scripts
- Pre-commit hooks that check the stack is up before running tests

---

## Part 3 — Bigger Swings (V2 Thinking)

These are more ambitious. Worth knowing about, not necessarily building.

---

### 3.1 stackup as a team protocol (not just a personal tool)

Right now stackup is a per-developer tool. The bigger opportunity is making it the team's shared source of truth for "what does this stack need to run?"

Imagine:
- `stackup.yml` defines the canonical stack contract
- CI validates the same health checks as local dev
- Staging environments can run `stackup check` against themselves
- Onboarding is `git clone` + `stackup up` + answer 3 questions

This is less a feature and more a positioning shift. stackup becomes the executable contract between a developer and a stack, not just a convenience wrapper.

---

### 3.2 Plugin system for custom health checks

Let teams define health checks in any language:

```yaml
services:
  kafka:
    health:
      type: plugin
      path: ./scripts/kafka-ready.sh
```

The plugin receives the service name and returns exit 0 (healthy) or non-zero (not yet). This makes stackup extensible to services with unusual health semantics (message queues, search indexes, etc.) without needing to add built-in support for every tool.

---

### 3.3 Reverse mode — stackup for integration tests

```bash
stackup test npm test
```

Starts the stack, runs the test command, tears it down, reports the combined exit code. This replaces the typical CI pattern of `docker compose up -d && sleep 10 && npm test && docker compose down` with something that's actually reliable.

---

## Priority Guide

If forced to pick the 5 most impactful items from this whole list:

1. **Log surfacing on failure** (1.1) — eliminates the most common daily frustration
2. **Lifecycle hooks** (1.2) — the feature that makes stackup feel like magic
3. **stackup doctor** (2.1) — the feature most likely to earn word-of-mouth
4. **Team onboarding mode** (2.5) — solves the actual new developer problem end-to-end
5. **stackup check** (2.2) — extends the tool to CI, doubles the addressable use case

---

*Last updated: project inception. Add ideas here as they come up during development. Delete items that have been shipped and moved into STATUS.md.*
