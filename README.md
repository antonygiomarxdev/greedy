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

# Run a DCA strategy
./build/greedy run --strategy examples/dca_btc.yaml

# Press Ctrl+C to gracefully stop (cancels all open orders)
```

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
├── cmd/greedy/          # Single binary entry point
├── internal/
│   ├── config/          # YAML strategy loading + validation
│   ├── crypto/          # NaCl secretbox + Argon2id encryption
│   ├── db/              # SQLite with WAL mode, migrations, repositories
│   ├── exchange/        # Exchange interface + canonical types
│   │   └── paper/       # Paper trading engine with matching
│   ├── bot/             # Strategy interface, state machine, supervisor
│   │   └── strategy/    # DCA, GRID, Signal strategies
│   └── mcp/             # MCP server for AI integration (Phase 2)
├── examples/            # Strategy YAML examples
└── Makefile
```

## Roadmap

- [x] **Phase 1**: Core engine, paper trading, DCA strategy, SQLite, CLI
- [ ] **Phase 2**: MCP server (AI-native control via Claude Desktop)
- [ ] **Phase 3**: GRID + Signal strategies
- [ ] **Phase 4**: Backtesting engine
- [ ] **Phase 5**: Web UI (embedded Svelte)
- [ ] **Phase 6**: Real exchange connectors (Coinbase, Binance)

## Requirements

- Go 1.22+
- Zero external dependencies to run (single static binary)

## License

MIT
