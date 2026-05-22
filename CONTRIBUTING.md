# Contributing to Greedy

## Development Setup

```bash
git clone https://github.com/antonygiomarxdev/greedy.git
cd greedy
go mod download
```

## Running Tests

```bash
go test -race -count=1 ./...
```

## Linting

```bash
golangci-lint run --timeout=2m
```

## Pre-commit Hooks

Install pre-commit to run checks before every commit:

```bash
pip install pre-commit
pre-commit install
```

## Commit Convention

This project follows [Conventional Commits](https://www.conventionalcommits.org/):

| Prefix | Use |
|--------|-----|
| `feat:` | New feature |
| `fix:` | Bug fix |
| `docs:` | Documentation only |
| `chore:` | Maintenance (deps, version bumps) |
| `test:` | Adding or updating tests |
| `refactor:` | Code changes without features or fixes |
| `ci:` | CI/CD changes |

Examples:
```
feat: add rate limiter for exchange requests
fix: prevent race condition in supervisor shutdown
docs: update architecture.md with vertical slicing
```

## Architecture

Greedy uses Clean Architecture with vertical slicing. Each feature
(`trading/`, `backtest/`, `mcp/`) is self-contained. Cross-cutting types
live in `shared/`. Adapters live in `infrastructure/`.

Before contributing, read `docs/architecture.md`.

## PR Checklist

- [ ] All tests pass: `go test -race ./...`
- [ ] Lint is clean: `golangci-lint run`
- [ ] New code follows the existing architecture pattern
- [ ] Commit messages follow conventional commits format
