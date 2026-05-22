# Greedy

Sovereign, local-first algorithmic trading engine. Single static binary. AI-native via MCP.

## Philosophy

- **Local-First**: 100% local execution. No cloud, no telemetry, no third-party servers.
- **Data Sovereignty**: API keys encrypted at rest (NaCl secretbox + Argon2id). Keys never leave your machine.
- **Ultra-Low Latency**: Go-native goroutines per bot. Tick-to-decision in microseconds.
- **Failsafe**: Per-bot isolation. Crash in one strategy never touches others.
- **AI-Native**: MCP server built-in. Any AI agent (Claude Desktop, Cursor, etc.) can monitor, create, and control trading strategies.

## Quick Start

```bash
make build
./build/greedy run --strategy examples/dca_btc.yaml
# Press Ctrl+C to stop gracefully
```

## AI Agent Control (MCP)

Greedy exposes an MCP (Model Context Protocol) server over stdio. Any MCP-compatible AI agent can control it natively.

### What an AI Agent Can Do

Run `greedy mcp-serve` and the agent gets 10 tools instantly:

| Tool | Use |
|------|-----|
| `get_ticker` | Check a price: *"What's ETH-USD at?"* |
| `get_order_book` | See order book depth |
| `get_candles` | Fetch OHLCV history |
| `place_order` | Trade: *"Buy 0.1 BTC market"* |
| `cancel_order` | Cancel an open order |
| `get_positions` | Show positions + unrealized P&L |
| `get_balances` | Show account balances |
| `start_bot` | Launch a strategy from YAML |
| `stop_bot` | Stop a running bot |
| `list_bots` | All active bots + status |

### E2E Walkthrough (20 JSON-RPC calls verified)

```
Agent → greedy:  initialize                          → capabilities: tools + resources + prompts
Agent → greedy:  tools/list                          → 10 tools with schemas
Agent → greedy:  tools/call get_ticker BTC-USD       → price: 50000
Agent → greedy:  tools/call get_balances             → USD: 100,000
Agent → greedy:  tools/call place_order buy 0.01 BTC → filled at 50049.99
Agent → greedy:  tools/call get_positions            → 0.01 BTC @ 50100 avg
Agent → greedy:  tools/call get_balances             → USD: 99,498.50 + 0.01 BTC
Agent → greedy:  tools/call start_bot dca_backtest   → started: bot dca-btc-backtest
Agent → greedy:  tools/call list_bots                → 1 bot running (DCA)
Agent → greedy:  tools/call start_bot grid_eth       → started: bot grid-eth-01
Agent → greedy:  tools/call start_bot signal_btc     → started: bot signal-btc-01
Agent → greedy:  tools/call list_bots                → 3 bots running (DCA + GRID + Signal)
Agent → greedy:  tools/call stop_bot <id>            → stopped: bot removed
Agent → greedy:  tools/call list_bots                → 0 bots
```

Full test script: `cat e2e_mcp_test.json | ./build/greedy mcp-serve`

### Claude Desktop Setup

1. Build: `make build`
2. Copy config: `cp examples/claude_desktop_config.json ~/.config/Claude/claude_desktop_config.json`
3. Update the binary path and `GREEDY_HOME` in the config
4. Restart Claude Desktop

Claude can now say: *"Start a DCA bot buying $100 of BTC every hour"* and Greedy executes it.

## Strategy Configuration

### DCA (Dollar-Cost Averaging)

```yaml
id: dca-btc-01
name: "BTC DCA Bot"
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
- [ ] **Phase 6**: Real exchange connectors (Coinbase, Binance)

**71 tests, 4 CI jobs, zero external runtime dependencies.**

## Requirements

- Go 1.22+
- Single static binary. No Docker required.

## License

MIT
