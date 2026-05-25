# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- CHANGELOG.md following Keep a Changelog format (#13)
- Homebrew tap via GoReleaser brews config (#13)

## [0.16.3] - 2026-05-25

### Fixed
- CI: handle resp.Body.Close() error explicitly (gosec G104)
- CI: restore orderRequest struct, refactor PlaceOrder to use typed map builder
- Build: replace broken .goreleaser.yml (homebrew-tap 403) with .goreleaser.yaml

### Changed
- Release: archive naming `greedy-$os-$arch.tar.gz`, binary matches archive name
- CI: trigger on `v*` tag pushes in addition to PR/branch
- Docs: AGENTS.md consolidated as single AI onboarding document (removed SYSTEM.md, vertical-slicing-plan.md)
- Docs: strategy-schema.md updated for Binance/Coinbase exchange support
- CI: ruleset requires codeowner review + status checks for non-admin PRs
- CI: CODEOWNERS set to @antonygiomarxdev

## [0.1.6] - 2026-05-22

### Added
- Vertical slice architecture: `trading/`, `backtest/`, `mcp/` feature directories
- Cross-cutting domain types package (`internal/shared/`) with Exchange, Order, Ticker, etc.
- Strategy builder pattern with typed registry (no more switch/magic strings)
- Command pattern for MCP tools (12 tool implementations)
- Order confirmer/filled listener callbacks for strategies
- Signal strategy with entry/exit triggers and take-profit/stop-loss levels
- MCP add_market and get_bot_status tools
- CI build matrix with arm64 and Go version alignment
- GoReleaser config and release workflow
- Version package with ldflags injection

### Changed
- Domain types consolidated into `internal/shared/` (Exchange, Order, Ticker, etc.)
- Bot runtime moved to `internal/trading/` with strategies as sub-package
- CLI delivery moved into feature directories (`trading/delivery/`, `backtest/delivery/`, `mcp/delivery/`)
- Paper exchange moved from `infrastructure/exchange/paper/` to `infrastructure/paper/`
- architecture.md updated to reflect vertical slice structure

### Removed
- Old horizontal-layer packages: `domain/`, `domain/bot/`, `domain/exchange/`, `domain/tool/`, `cli/`

## [0.1.5] - 2026-05-22

### Added
- Initial MCP server with 10 tools
- GRID and Signal strategies
- Backtesting engine with CSV loader
- Paper exchange with static, random walk, and CSV replay feeds
- Multi-bot supervisor with restart policies
- SQLite persistence for bots and configs
- YAML strategy file loader
