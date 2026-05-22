# Greedy — Production Readiness Plan

> Proposed: 2026-05-22 | Status: DRAFT

## Overview

3 milestones, 12 issues. Ordered for incremental delivery. Each issue is self-contained with a single commit.

---

## Milestone 1: Distribution & Release Pipeline

### Issue #M1-1: Add version package + ldflags injection
**Labels:** `area/build`, `enhancement`

Create `internal/version/version.go`:
```go
var Version = "dev"; var Commit = "unknown"; var BuildDate = "unknown"
```

Wire into:
- `cmd/greedy/main.go` — `version` subcommand prints all 3 fields
- `internal/mcp/transport.go` — `serverInfo.Version` uses `version.Version`
- `Makefile` — add `LDFLAGS` target with ldflags injection
- `ci.yml` — inject version/commit/date in build step

### Issue #M1-2: Add GoReleaser + release workflow
**Labels:** `area/build`, `area/ci`, `enhancement`

- Create `.goreleaser.yml`: build matrix `linux/darwin/windows` × `amd64/arm64`, CGO_ENABLED=0, SHA256 checksums, `.tar.gz`/`.zip` archives
- Create `.github/workflows/release.yml`: trigger on `v*` tags, runs goreleaser
- Update `.gitignore`: add `dist/`

### Issue #M1-3: Add LICENSE, CHANGELOG, Homebrew tap
**Labels:** `area/meta`, `documentation`

- Create `LICENSE` (MIT) — required for any public distribution
- Create `CHANGELOG.md` (Keep a Changelog format)
- Add Homebrew `brews` section to `.goreleaser.yml` → auto-push to `antonygiomarxdev/homebrew-tap`
- Create the `homebrew-tap` repo on GitHub (empty, goreleaser fills it)

### Issue #M1-4: CI build matrix with ARM64 + Go version alignment
**Labels:** `area/ci`, `enhancement`

- Add `arm64` targets to CI build check (linux/darwin/windows × arm64)
- Align Go version: `go.mod` says 1.23, CI tests 1.23/1.24 → keep both, drop 1.25 references
- Cross-compile check: 6 builds instead of 3

---

## Milestone 2: MCP Documentation & AI Onboarding

### Issue #M2-5: Enhance MCP tool descriptions for AI discoverability
**Labels:** `area/mcp`, `documentation`, `enhancement`

Rewrites every `ToolDef.Description` in `internal/mcp/server.go` with AI-rich detail. Each description includes:
- Symbol format convention (`BASE-QUOTE`, e.g. `BTC-USD`)
- Valid values / enums / defaults
- Side effects and what to expect in responses
- Relationships to other tools (call X before Y)

Before: `"Get current price for a trading symbol"`
After: `"Get current bid/ask/last price for a symbol. Symbols use FOREX convention (e.g. 'BTC-USD'). Returns Ticker with Bid/Ask/Last/Volume. Paper mode simulates prices via random walk starting at $50,000 for BTC-USD."`

Also adds `enum` constraints to `InputSchema` where applicable (e.g. `side: ["buy", "sell"]`, `type: ["market", "limit"]`).

### Issue #M2-6: Implement resources/read and prompts/get handlers
**Labels:** `area/mcp`, `enhancement`

Currently `resources/read` and `prompts/get` return errors. Implement both:

**Resources** (live state as JSON):
- `greedy://positions` — current positions
- `greedy://balances` — account balances
- `greedy://bots` — active bots
- `greedy://markets` — available symbols

**Prompts** (guided workflows):
- `create-dca-strategy` — guides AI through creating DCA YAML
- `create-grid-strategy` — same for grid
- `analyze-positions` — AI analyzes positions and suggests actions

Add handlers to `transport.go` switch statement. Resources return `application/json`, prompts return text.

### Issue #M2-7: Fix start_bot bugs + add missing MCP tools
**Labels:** `area/mcp`, `bug`, `enhancement`

**Bug fix:** `server.go:handleStartBot` doesn't parse `safety_orders` from params (diverges from CLI `buildStrategy`). Extract strategy-building into shared `internal/config/strategy_builder.go` function used by both callers.

**New tools:**
1. `add_market` — add new symbol to paper exchange (params: `symbol`, `start_price`, `volatility`)
2. `get_bot_status` — detailed single-bot state (params: `bot_id`)

**Signal trigger gap:** The signal strategy waits for external triggers via `Trigger()` method. Without a trigger MCP tool, Signal is unusable via MCP. Two options:
- Option A: Add `trigger_signal` tool (explicit trigger, AI controls timing)
- Option B: Implement EMA crossover internally in Signal (self-triggering)

### Issue #M2-8: Create SYSTEM.md + strategy YAML schema reference
**Labels:** `area/mcp`, `documentation`

Create `SYSTEM.md` at repo root — zero-context AI onboarding document. An AI agent reading this with no prior knowledge should understand:
- What Greedy is and its architecture
- Paper trading defaults ($100K USD, 0.1% fee, $50K BTC)
- FOREX symbol convention (`BASE-QUOTE`)
- All 10+ MCP tools with usage notes
- Bot lifecycle (states, IDs, restart policies)

Create `docs/strategy-schema.md` — canonical YAML reference for DCA, GRID, Signal with all params, types, defaults, valid ranges, and safety order semantics.

Optionally embed `SYSTEM.md` as MCP resource `greedy://onboarding` so agents can retrieve it via `resources/read`.

---

## Milestone 3: Real Exchange Connectors

### Issue #M3-9: ExchangeConfig struct + credential encryption
**Labels:** `area/config`, `area/security`, `enhancement`

Create `ExchangeConfig` struct in `internal/config/config.go`:
```yaml
exchange:
  type: coinbase          # "paper", "coinbase", "binance"
  api_key: "your-key"
  api_secret: "base64-encrypted-secret"
  passphrase: "..."       # Coinbase only
  sandbox: true           # Use sandbox endpoint
```

- Add encrypt/decrypt helper in `internal/config/credentials.go` using existing `crypto.Encrypt`/`Decrypt`
- `BotConfig.Exchange` changes from `string` to `ExchangeConfig` with backward compat (accept `"paper"` string as shorthand)
- Add `--master-password` flag or `GREEDY_MASTER_KEY` env var for decrypting secrets at runtime

### Issue #M3-10: Coinbase REST exchange connector
**Labels:** `area/exchange`, `enhancement`

Create `internal/exchange/coinbase/`:

| File | Purpose |
|------|---------|
| `coinbase.go` | `CoinbaseExchange` struct implementing `exchange.Exchange` |
| `auth.go` | HMAC-SHA256 signing, header construction (CB-ACCESS-KEY/TIMESTAMP/SIGN) |
| `rest.go` | HTTP client, rate limiting, error parsing |

Implement all 13 `Exchange` interface methods. Key mappings:
- `GetTicker` → `GET /products/{id}/ticker`
- `PlaceOrder` → `POST /orders` (market orders use `quote_size`, limit orders use `base_size`)
- `GetCandles` → map `CandleInterval` to Coinbase granularity enums
- `SubscribeOrderBook` → WebSocket (defer to M3-12)

Use stdlib `net/http` only — no external dependencies.

### Issue #M3-11: Binance REST exchange connector
**Labels:** `area/exchange`, `enhancement`

Create `internal/exchange/binance/` — same structure as Coinbase but Binance API:

- Auth: HMAC-SHA256 query parameter signing (different from Coinbase headers)
- REST base: `https://api.binance.com/api/v3`
- Symbol format: `BTCUSDT` (no hyphen) — needs mapping layer
- No passphrase required
- Rate limiting: 1200 weight/minute

### Issue #M3-12: Exchange factory + CLI/MCP integration
**Labels:** `area/exchange`, `area/cli`, `area/mcp`, `enhancement`

- Create `internal/exchange/factory.go`: `func NewExchange(cfg ExchangeConfig, masterPassword string) (Exchange, error)` — returns paper/coinbase/binance based on config type
- Wire factory into `cmd/greedy/main.go` and `internal/mcp/server.go`
- Add `--master-password` flag or `GREEDY_MASTER_KEY` env var
- Add exchange credential management: `greedy config set-exchange --type coinbase --api-key X --api-secret Y`
- Update MCP `start_bot` to pass credentials from config
- Add `list_markets` to MCP tools (shows available symbols per exchange)

---

## Execution Order

```
M1-1 (version) → M1-2 (goreleaser) → M1-3 (license/changelog) → M1-4 (CI arm64)
    ↓
M2-5 (tool descriptions) → M2-7 (bug fixes + new tools) → M2-6 (resources/prompts) → M2-8 (SYSTEM.md)
    ↓
M3-9 (config) → M3-10 (Coinbase) → M3-11 (Binance) → M3-12 (factory + CLI)
```

Each issue = 1 commit. M1 issues can be parallelized. M2 issues build on each other. M3 is strictly sequential.

---

## Verification

- **M1**: `git tag v0.2.0 && git push --tags` → GitHub Release created with binaries for 6 platforms, `brew install antonygiomarxdev/tap/greedy` works
- **M2**: Any AI agent (Claude, Cursor) can connect to `greedy mcp-serve`, read `SYSTEM.md`, list tools, create strategies, trigger signals, and trade — with zero human intervention
- **M3**: `greedy run --strategy examples/dca_coinbase.yaml` trades on Coinbase sandbox with real API keys, positions tracked, fees applied
