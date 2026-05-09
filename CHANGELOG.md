# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Health-gated service startup with parallel health checks within tiers
- Environment validation with schema types (url, int, bool) and default injection
- `stackup doctor` command for automated diagnostics (port conflicts, env drift, localhost misuse, crash loops)
- `stackup check` command for CI-friendly health assertions (exit 0/2, JSON output)
- Interactive team onboarding when `.env` is missing
- Lifecycle hooks (`after_start`) for post-startup automation
- Log-based health check type for services without HTTP/TCP readiness
- Smart `stackup init` with image detection for 18+ known Docker images
- Failure diagnostics: auto-surface container logs and suggest cleanup
- Shared `Poll()` utility for health checker implementations
- Config validation on load (type checking, required fields, port ranges)
- Signal handling for graceful Ctrl+C cancellation
- GitHub Actions CI (test, lint, cross-platform build)
- GoReleaser config for multi-platform binary releases
- golangci-lint configuration

### Fixed
- HTTP health checker timeout bug (per-request timeout was too short)
- Global state mutation via os.Setenv (now passes env to child processes)
- Graph algorithm was O(V²+VE), now proper O(V+E) Kahn's with queue
- Cycle detection error now lists involved services
- Flaky parallel health check test (relative threshold instead of hardcoded ms)

### Changed
- `PreFlight` returns injected defaults instead of mutating process environment
- Module path corrected to `github.com/deveshpharswan/stackup`

## [0.1.0] - 2024-12-01

### Added
- Initial release with `stackup up`, `stackup down`, `stackup validate`
- Basic health checks (HTTP, TCP, Docker)
- `.env` validation against `.env.example`
- `stackup init` config generator
- `stackup logs`, `stackup shell`, `stackup restart`, `stackup run`, `stackup status`
