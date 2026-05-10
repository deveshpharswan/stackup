# Testing Guide

## Prerequisites

- **Go 1.21+** — [go.dev](https://go.dev) or `brew install go` (macOS) or `sudo apt install golang-go` (Linux)
- **Docker Engine + Compose plugin** — Docker Desktop (Mac/Windows) or `sudo apt install docker.io docker-compose-plugin` (Linux)

Verify prerequisites:
```bash
go version          # go1.21+
docker compose version  # Docker Compose version v2+
```

---

## Running the E2E Test Suite

Run all tests:

```bash
go test ./tests/e2e/... -v -timeout 10m
```

Run a specific area:

```bash
go test ./tests/e2e/... -v -run TestHealthHTTP -timeout 5m
go test ./tests/e2e/... -v -run TestComposeDiscovery -timeout 5m
go test ./tests/e2e/... -v -run TestCommands -timeout 3m
```

Run only tests that do NOT need Docker:

```bash
go test ./tests/e2e/... -v -run "TestEnvGate|TestCommands_Version|TestCommands_Init|TestComposeDiscovery_Missing|TestComposeDiscovery_FlagOverride" -timeout 2m
```

**On Windows:** `go test` will fail with Windows Defender blocking the binary in the temp dir. Use Linux (WSL) or macOS, or run inside Docker:

```bash
docker run --rm -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$(pwd)":/workspace -w /workspace golang:1.21 \
  go test ./tests/e2e/... -v -timeout 10m
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

**Windows Defender blocks binary** — `go test` builds a binary in a temp dir that Defender may block. Run tests on Linux or macOS instead.
