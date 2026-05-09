# Contributing to Stackup

Thanks for your interest in contributing to Stackup! This guide will help you get started.

## Development Setup

```bash
git clone https://github.com/deveshpharswan/stackup.git
cd stackup
go mod download
make build
```

**Requirements:**
- Go 1.22+
- Docker Engine with `docker compose` v2
- golangci-lint (for linting)

## Making Changes

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/my-feature`
3. Make your changes
4. Run tests: `make test`
5. Run linter: `make lint`
6. Commit with a descriptive message (see below)
7. Push and open a Pull Request

## Commit Messages

We follow [Conventional Commits](https://www.conventionalcommits.org/):

```
feat: add new health check type
fix: resolve timeout in TCP checker
docs: update README with new command
test: add parallel health check tests
refactor: extract polling utility
```

## Code Style

- Run `golangci-lint run` before committing
- Follow standard Go conventions (gofmt, govet)
- Keep interfaces small (1-3 methods)
- Add `t.Parallel()` to independent tests
- No comments unless the "why" is non-obvious

## Testing

```bash
make test        # Unit tests
make lint        # go vet + golangci-lint
```

Tests must pass before merging. If you add a new feature, add tests for it.

## Project Structure

```
cmd/             CLI commands (thin wrappers)
internal/        Core logic (not importable externally)
  config/        YAML config parsing + validation
  constants/     Shared constants
  docker/        Docker SDK wrapper
  doctor/        Diagnostic checks
  env/           .env validation
  health/        Health check implementations
  hooks/         Lifecycle hook executor
  onboard/       First-run setup wizard
  orchestrator/  Dependency graph + startup engine
  printer/       Terminal output formatting
  scaffold/      Config generator
testdata/        Test fixtures
```

## Reporting Bugs

Open an issue with:
- What you expected to happen
- What actually happened
- Steps to reproduce
- Your `stackup version` output

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
