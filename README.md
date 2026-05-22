# Greedy

Sovereign, local-first algorithmic trading engine. Single static binary. CLI-native with MCP AI integration.

## Philosophy

- **Local-First**: 100% local execution. No cloud dependencies, no telemetry, no third-party servers.
- **Data Sovereignty**: API keys encrypted at rest with NaCl secretbox + Argon2id. Your keys never leave your machine.
- **Ultra-Low Latency**: Go-native goroutines per bot. Tick-to-decision in microseconds. No interpreter overhead.
- **Failsafe**: Per-bot isolation. Crash in one strategy never touches others. Graceful shutdown cancels all open orders.
- **AI-Native**: MCP (Model Context Protocol) server built-in. Claude Desktop and other AI agents can monitor, create, and control trading strategies via natural language.

## Quick Start

```bash
# Build
make build

# Run a DCA strategy (paper trading)
./build/greedy run --strategy examples/dca_btc.yaml

# Press Ctrl+C to gracefully stop (cancels all open orders)
```

## AI-Native Trading (MCP)

Greedy ships with a built-in MCP server. Connect it to Claude Desktop for natural language trading control.

### Claude Desktop Setup

1. Build the binary: `make build`
2. Copy the config: `cp examples/claude_desktop_config.json ~/.config/Claude/claude_desktop_config.json`
3. Adjust the binary path and `GREEDY_HOME` in the config
4. Restart Claude Desktop

### Available Tools

| Tool | Description |
|------|-------------|
| `get_ticker` | Current price for a symbol |
| `get_order_book` | Bid/ask order book |
| `get_candles` | Historical OHLCV candles |
| `place_order` | Place market/limit orders |
| `cancel_order` | Cancel an open order |
| `get_positions` | Current positions with P&L |
| `get_balances` | Account balances |
| `start_bot` | Launch a strategy from YAML |
| `stop_bot` | Stop a running bot |
| `list_bots` | All active bots with status |

### Example Prompts for Claude

- "What's the BTC-USD price right now?"
- "Start a DCA bot for ETH-USD buying $50 every hour"
- "Show me all my positions and unrealized P&L"
- "Stop the BTC DCA bot"
- "Place a limit buy order for 0.01 BTC at $48,000"

## Strategy Configuration

```yaml
# examples/dca_btc.yaml
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

## Architecture

```
greedy/
├── cmd/greedy/              # Single binary entry point
├── internal/
│   ├── config/              # YAML strategy loading + validation
│   ├── crypto/              # NaCl secretbox + Argon2id encryption
│   ├── db/                  # SQLite with WAL mode, migrations, repositories
│   ├── exchange/            # Exchange interface + canonical types
│   │   └── paper/           # Paper trading engine with matching engine
│   ├── bot/                 # Strategy interface, state machine, supervisor
│   │   └── strategy/        # DCA, GRID, Signal strategies
│   └── mcp/                 # MCP server (JSON-RPC 2.0 stdio transport)
├── examples/                # Strategy YAML + Claude Desktop config
└── Makefile
```

## Roadmap

Track progress on [GitHub Issues](https://github.com/antonygiomarxdev/greedy/issues).

- [x] **Phase 1**: Core engine, paper trading, DCA strategy, SQLite, CLI
- [x] **Phase 2**: MCP server with 10 AI tools (AI-native control)
- [x] **Phase 3**: GRID + Signal strategies with multi-bot concurrency
- [x] **Phase 4**: Backtesting engine with CSV data and rich metrics
- [ ] **Phase 6**: Real exchange connectors (Coinbase, Binance)

## Requirements

- Go 1.22+
- Zero external dependencies to run (single static binary)

## License

MIT
