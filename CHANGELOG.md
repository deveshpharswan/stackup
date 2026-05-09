# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.0] - 2026-05-09

### Added

- Rich colored terminal output with `fatih/color` (auto-detects TTY, respects NO_COLOR)
- Animated braille spinner during health check polling
- Summary table at end of `stackup up` showing all services, status, check type, and timing
- Shell completions for bash, zsh, fish, and PowerShell (`stackup completion`)
- Godoc comments on all exported types and functions across internal packages
- Profile support (`--profile` flag on `up` and `check`) for starting service subsets
- Structured JSON output (`--output json` on `doctor`, `status`, `validate`)
- Partial success mode (`--partial` flag on `up`) — continues on independent tier failures
- `ProfileServices()` config method for profile resolution

### Changed

- Printer rewritten with color-coded output (green=healthy, red=failed, yellow=waiting, cyan=headers)
- Doctor output uses colored icons and dimmed secondary text
- Orchestrator returns `ServiceResult` slices for summary table rendering
- Health check phase shows spinner while waiting, then prints results

## [0.2.0] - 2026-05-09

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
