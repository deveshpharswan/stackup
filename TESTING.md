# Testing Guide

## Prerequisites

| Requirement | Linux | macOS | Windows |
|-------------|-------|-------|---------|
| Go 1.21+    | `sudo apt install golang-go` or [go.dev](https://go.dev) | `brew install go` | [go.dev](https://go.dev) |
| Docker Engine + Compose plugin | `sudo apt install docker.io docker-compose-plugin` | Docker Desktop | Docker Desktop |
| bash (for smoke test) | built-in | built-in | Git Bash or WSL |

Verify prerequisites:
```bash
go version          # go1.21+
docker compose version  # Docker Compose version v2+
```

---

## Option A — Go E2E Test Suite (permanent, runs in CI)

Build the binary first, then run all tests:

```bash
go test ./tests/e2e/... -v -timeout 10m
```

Run a specific test file:

```bash
go test ./tests/e2e/... -v -run TestHealthHTTP -timeout 5m
go test ./tests/e2e/... -v -run TestComposeDiscovery -timeout 5m
go test ./tests/e2e/... -v -run TestCommands -timeout 3m
```

Run only tests that do NOT need Docker (env, commands, discovery subset):

```bash
go test ./tests/e2e/... -v -run "TestEnvGate|TestCommands_Version|TestCommands_Init|TestComposeDiscovery_Missing|TestComposeDiscovery_FlagOverride" -timeout 2m
```

**On Windows:** `go test` will fail with Windows Defender blocking the binary in the temp dir. Use Linux (WSL) or macOS, or run inside Docker:

```bash
# From repo root, run tests inside a Linux container
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$(pwd)":/workspace -w /workspace golang:1.21 \
  go test ./tests/e2e/... -v -timeout 10m
```

---

## Option D — Bash Smoke Test (one-shot, delete when done)

Build the binary:

```bash
go build -o stackup .
```

Run the smoke test:

```bash
bash smoke-test.sh ./stackup
```

On a machine where the binary is already built elsewhere:

```bash
bash smoke-test.sh /path/to/stackup
```

On Windows with Git Bash:

```bash
go build -o stackup.exe .
bash smoke-test.sh ./stackup.exe
```

**Delete the smoke test when satisfied:**

```bash
git rm smoke-test.sh
git commit -m "chore: remove smoke test after validation"
git push
```

---

## Troubleshooting

**"docker compose not available"** — Install Docker Desktop (Mac/Windows) or `docker-compose-plugin` (Linux). Ensure Docker daemon is running.

**Port conflicts** — The test suite uses ports 15432, 16379–16382, 18080–18086. Free these up before running:

```bash
# Check what's using a port
sudo lsof -i :18080
# Or on Windows
netstat -ano | findstr :18080
```

**Slow first run** — First run pulls Docker images (~100MB total). Subsequent runs use cached images.

**Tests hang** — Each test has a 120s timeout. If it hangs, check Docker daemon is running and images are accessible.

**Windows Defender blocks binary** — Option A (`go test`) builds a binary in a temp dir. Windows Defender may block it. Use Option D (smoke test) from Git Bash, which tests a pre-built binary.
