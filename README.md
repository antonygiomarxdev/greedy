# Greedy

[![CI](https://github.com/antonygiomarxdev/greedy/actions/workflows/ci.yml/badge.svg)](https://github.com/antonygiomarxdev/greedy/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/antonygiomarxdev/greedy/branch/main/graph/badge.svg)](https://codecov.io/gh/antonygiomarxdev/greedy)
[![Go Report Card](https://goreportcard.com/badge/github.com/antonygiomarxdev/greedy)](https://goreportcard.com/report/github.com/antonygiomarxdev/greedy)
[![Go Version](https://img.shields.io/github/go-mod/go-version/antonygiomarxdev/greedy)](https://golang.org/dl/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)
[![Downloads](https://img.shields.io/github/downloads/antonygiomarxdev/greedy/total)](https://github.com/antonygiomarxdev/greedy/releases)

**Sovereign, local-first algorithmic trading engine. Single static binary. AI-native via MCP.**

## Features

- **AI-Native MCP Server**: 14 tools, 4 resources, 3 prompts. Claude Desktop, Cursor, or any MCP client can monitor and control your strategies.
- **Multi-Strategy**: DCA, GRID, Signal — run dozens concurrently with per-bot isolation.
- **Real Exchange Support**: Coinbase, Binance — same interface, zero code changes.
- **Local-First**: 100% local execution. No cloud, no telemetry, no third-party servers.
- **Encrypted at Rest**: API keys encrypted with NaCl secretbox + Argon2id. Keys never leave your machine.
- **Enterprise Crypto**: Argon2id key derivation, NaCl secretbox encryption, constant-time comparisons.
- **Order Persistence**: All orders automatically stored in SQLite WAL — queryable via MCP or CLI.
- **Backtesting**: CSV-driven backtester with Sharpe ratio, max drawdown, win rate.
- **Ultra-Low Latency**: Go-native goroutines per bot. Tick-to-decision in microseconds.
- **Failsafe**: Per-bot isolation. Crash in one strategy never touches others. Circuit breakers prevent runaway trading.
- **Single Binary**: ~15 MB statically linked, zero runtime dependencies.
- **Idempotent Order Placement**: Prevents duplicate orders across restarts.

## Supported Platforms

| OS | Arch | Status |
|---|---|---|
| Linux | amd64 | ✅ Production |
| Linux | arm64 | ✅ Production |
| macOS | amd64 | ✅ Production |
| macOS | arm64 | ✅ Production |
| Windows | amd64 | ✅ Production |

**Requirements**: Go 1.23+ (to build from source), or download the prebuilt binary for your platform.

## Quick Start

### Install

**Binary download** (recommended):
```bash
# Linux amd64
curl -fsSL https://github.com/antonygiomarxdev/greedy/releases/latest/download/greedy-linux-amd64.tar.gz | tar xz
sudo install greedy-linux-amd64 /usr/local/bin/greedy

# macOS arm64 (Apple Silicon)
curl -fsSL https://github.com/antonygiomarxdev/greedy/releases/latest/download/greedy-darwin-arm64.tar.gz | tar xz
sudo install greedy-darwin-arm64 /usr/local/bin/greedy
```

**One-liner** (auto-detect OS/arch):
```bash
curl -fsSL https://github.com/antonygiomarxdev/greedy/releases/latest/download/greedy-linux-amd64.tar.gz \
  | tar xz && sudo install greedy-linux-amd64 /usr/local/bin/greedy
```

**Or via make**:
```bash
sudo make install
```

**Build from source**:
```bash
git clone https://github.com/antonygiomarxdev/greedy.git
cd greedy
make build
```

### Run a Strategy

```bash
greedy run --strategy examples/dca_btc.yaml
# Press Ctrl+C to stop gracefully
```

## MCP Protocol Reference

### Tools (14)

| Tool | Exchange param | Input | Description |
|---|---|---|---|
| `get_ticker` | ✅ | `symbol`, `exchange?` | Current price for a symbol |
| `get_order_book` | ✅ | `symbol`, `depth?`, `exchange?` | Bids + asks order book |
| `get_candles` | ✅ | `symbol`, `interval?`, `limit?`, `exchange?` | OHLCV history |
| `place_order` | ✅ | `symbol`, `side`, `type`, `quantity`, `price?`, `exchange?` | Place market or limit order |
| `cancel_order` | ✅ | `order_id`, `exchange?` | Cancel by order ID |
| `get_positions` | ✅ | `exchange?` | All positions + P&L |
| `get_balances` | ✅ | `exchange?` | Account balances |
| `get_order_history` | ❌ | `bot_id?`, `symbol?`, `limit?` | Historical orders |
| `start_bot` | ❌ | `strategy_file?` or `type` + `symbol` + `params` | Launch a bot |
| `stop_bot` | ❌ | `bot_id` | Stop a running bot |
| `list_bots` | ❌ | (none) | Active bots + P&L |
| `set_credential` | ❌ | `exchange`, `label`, `api_key`, `api_secret`, `passphrase?` | Store encrypted API keys |
| `list_credentials` | ❌ | (none) | List stored credential identifiers |
| `delete_credential` | ❌ | `exchange`, `label` | Remove stored credential |

Default exchange is `paper` when `exchange` is omitted.

### Resources (4)

| URI | Description |
|---|---|
| `portfolio://summary` | Full portfolio snapshot: balances, positions, total P&L |
| `bot://{id}/status` | Bot status, strategy config, P&L, open orders |
| `bot://{id}/history` | Complete order history for a bot |
| `market://prices/{symbol}` | Real-time price for any symbol |

### Prompts (3)

| Name | Description |
|---|---|
| `analyze_portfolio` | Portfolio risk and exposure analysis |
| `review_trades` | Today's trading activity with P&L breakdown |
| `suggest_strategy` | Suggest DCA/GRID params based on market conditions |

### E2E Walkthrough

```
Agent → greedy:  tools/list                          → 14 tools with schemas
Agent → greedy:  tools/call get_ticker BTC-USD       → price: 50000
Agent → greedy:  tools/call get_balances             → USD: 100,000
Agent → greedy:  tools/call place_order buy 0.01 BTC → filled at 50049.99
Agent → greedy:  tools/call get_positions            → 0.01 BTC @ 50100 avg
Agent → greedy:  tools/call get_order_history        → 3 orders found
Agent → greedy:  tools/call start_bot                → started: bot dca-btc-01
Agent → greedy:  tools/call list_bots                → 1 bot running (DCA)
Agent → greedy:  tools/call start_bot grid_eth       → started: bot grid-eth-01
Agent → greedy:  tools/call start_bot signal_btc     → started: bot signal-btc-01
Agent → greedy:  tools/call list_bots                → 3 bots running (+P&L)
Agent → greedy:  tools/call stop_bot <id>            → stopped: bot removed
Agent → greedy:  tools/call list_bots                → 0 bots
```

### Real Exchange Support

Greedy connects to Coinbase and Binance via encrypted credentials. Start the daemon:

```bash
export GREEDY_MASTER_PASSWORD="your-strong-password"
greedy serve --config examples/greedy_with_exchanges.yaml
```

`GREEDY_MASTER_PASSWORD` is required for credential encryption using Argon2id + NaCl secretbox. Without it, only paper trading is available.

Set API keys via MCP:

```json
{"exchange": "coinbase", "label": "default", "api_key": "...", "api_secret": "...", "passphrase": "..."}
```

Or configure exchanges in YAML (credentials must exist in the store):

```yaml
exchanges:
  - name: "Binance Spot"
    provider: binance
    label: default
  - name: "Coinbase"
    provider: coinbase
    label: default
```

**Symbol format**: Binance uses `BTCUSDT` (no dash), Coinbase uses `BTC-USD`. Pass the format your exchange expects.

### Claude Desktop Setup

1. Build or download the binary
2. Copy config: `cp examples/claude_desktop_config.json ~/.config/Claude/claude_desktop_config.json`
3. Update the binary path and `GREEDY_HOME` in the config
4. If using real exchanges, add `GREEDY_MASTER_PASSWORD` to the env
5. Restart Claude Desktop

Claude can now say: *"Start a DCA bot buying $100 of BTC on Binance every hour"* and Greedy executes it.

## Production

### Systemd Service (Linux)

```bash
sudo make install-service          # build + install unit
sudo systemctl enable greedy       # auto-start on boot
sudo systemctl start greedy        # start now
sudo systemctl status greedy       # verify
```

Logs: `journalctl -u greedy -f`

Manual config path: `/etc/systemd/system/greedy.service` (see `build/greedy.service`).

### CLI Credential Management

Set API keys from a headless terminal (no MCP server needed):

```bash
export GREEDY_MASTER_PASSWORD="your-strong-password"

# Direct flags
greedy credential set --exchange binance --label default --api-key XXX --api-secret YYY

# Via env vars (safer — keys don't appear in process list)
greedy credential set --exchange coinbase --label default \
  --api-key-env COINBASE_API_KEY --api-secret-env COINBASE_API_SECRET

greedy credential list
greedy credential get --exchange binance --label default
greedy credential delete --exchange binance --label default
```

All commands use the same encrypted SQLite store as the MCP server. `GREEDY_MASTER_PASSWORD` must match the one used at server startup.

## Security Architecture

### Credential Encryption

```
GREEDY_MASTER_PASSWORD
  → Argon2id (mem=64MB, iter=3, threads=4, salt=nil)
    → 32-byte key
      → NaCl secretbox (XSalsa20-Poly1305)
        → Encrypted blob in SQLite
```

- Keys are **never** logged or exposed via MCP `list_credentials`.
- Store uses SQLite WAL mode — safe concurrent access.
- Salt persistence for key derivation is planned (currently uses nil salt — upgrading requires re-encryption).

### Exchange Isolation

- Per-bot goroutines with independent context cancellation.
- Circuit breaker stops all bot activity during extreme volatility.
- Idempotency keys prevent duplicate orders after restart.
- Debouncer rate-limits order placement per bot.

## Strategy Configuration

**Symbol format**: `BTC-USD` for paper/Coinbase, `BTCUSDT` for Binance.

### DCA (Dollar-Cost Averaging)

```yaml
id: dca-btc-01
name: "BTC DCA Bot"
exchange: paper
strategy:
  type: dca
  symbol: BTC-USD
  params:
    base_order_size: 100
    frequency: "1h"
    max_safety_orders: 10
    safety_orders:
      - price_deviation_pct: -5
        volume_scale: 1.5
      - price_deviation_pct: -10
        volume_scale: 2.0
```

Binance version:
```yaml
id: dca-btc-binance
name: "BTC DCA Bot"
exchange: binance
strategy:
  type: dca
  symbol: BTCUSDT
  params:
    base_order_size: 100
    frequency: "1h"
```

### GRID (Limit Order Grid)

```yaml
id: grid-eth-01
name: "ETH GRID Bot"
strategy:
  type: grid
  symbol: ETH-USD
  params:
    lower_bound: 2000
    upper_bound: 4000
    grid_levels: 10
    order_size: 0.5
```

### Signal (Trigger-Based)

```yaml
id: signal-btc-01
name: "BTC Signal Bot"
strategy:
  type: signal
  symbol: BTC-USD
  params:
    position_size: 1000
    entry_condition: "ema_cross_above"
    exit_condition: "ema_cross_below"
```

## Backtesting

```bash
./build/greedy backtest \
  --strategy examples/dca_backtest.yaml \
  --data examples/btc_sample.csv \
  --report json
```

Reports include: total return, max drawdown, Sharpe ratio, win rate, profit factor.

## Architecture

Greedy follows **Clean Architecture** with **Vertical Slicing**. Every feature is a complete vertical through all layers: domain → infrastructure → use cases → delivery.

```
cmd/greedy/                     # Composition root (DI wiring only)
internal/
  domain/                       # Enterprise business rules
    exchange/                   # Exchange interface + types
    bot/                        # Bot + Supervisor interfaces
    strategy/                   # Strategy interface + config types
    stream/                     # PriceStreamer, MarketTracker, RateLimiter...
  usecases/                     # Application business rules
    start_bot.go                # StartBotUseCase
    place_order.go              # PlaceOrderUseCase (idempotent)
    get_portfolio.go            # GetPortfolioUseCase
  infrastructure/               # Concrete implementations
    exchange/paper/             # Paper trading (10 tests)
    db/                         # SQLite WAL + migrations + repos (11 tests)
    config/                     # YAML loading + validation (18 tests)
    stream/                     # RateLimiter, PriceStreamer, MarketTracker impls
  delivery/                     # Input adapters
    mcp/                        # JSON-RPC 2.0 stdio server (9 tests)
    cli/                        # CLI subcommand handlers
├── docs/
│   └── architecture.md         # Full architecture reference
├── examples/                   # Strategy YAML + Claude config + sample data
├── .github/workflows/ci.yml    # 4 CI jobs (lint, test, security, build)
└── .golangci.yml               # Strict linting config
```

**Key rules:**
- `domain/` imports nothing from other layers
- `usecases/` depends only on `domain/` interfaces
- `infrastructure/` implements `domain/` interfaces
- `delivery/` delegates to use cases
- `cmd/greedy/main.go` is pure DI wiring — zero business logic

Read the full architecture reference: **[docs/architecture.md](docs/architecture.md)**

## Roadmap

Tracked on [GitHub Issues](https://github.com/antonygiomarxdev/greedy/issues).

- [x] **Phase 1**: Core engine, paper trading, DCA, SQLite, CLI
- [x] **Phase 2**: MCP server — 10 tools, 3 resources, 3 prompts
- [x] **Phase 3**: GRID + Signal strategies, multi-bot concurrency
- [x] **Phase 4**: Backtesting engine with CSV data
- [x] **Phase 5**: Real exchange connectors + MCP credentials — 14 tools, inline bot params, exchange targeting, order persistence
- [ ] **Phase 6**: Market making, trailing stops, performance dashboards

**15 test suites, 4 CI jobs (lint, test, security, build), zero external runtime dependencies.**

## Requirements

- Go 1.23+ (to build from source)
- Prebuilt binaries available for Linux (amd64/arm64), macOS (amd64/arm64), Windows (amd64)
- Single static binary. No Docker, no runtime dependencies.

## Environment Variables

| Variable | Required | Description |
|---|---|---|
| `GREEDY_MASTER_PASSWORD` | For real exchanges | Master key for credential encryption/decryption |
| `GREEDY_HOME` | No | Data directory (default: `~/.greedy`) |

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/my-feature`
3. Run tests: `make test`
4. Ensure lint passes: `gofmt -l .` and `go vet ./...`
5. Open a Pull Request

## License

MIT
