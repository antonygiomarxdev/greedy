# Greedy — AI Onboarding

An AI agent with zero prior knowledge should read this document to understand
everything needed to operate Greedy via MCP.

## What Greedy Is

Sovereign, local-first algorithmic trading engine. Single static binary.
AI-native via MCP protocol. Runs locally — no cloud, no telemetry.

**Current mode**: Paper trading (simulated market). Real exchange connectors pending.

## Architecture

```
CLI (greedy serve) → starts daemon with exchange + strategies + MCP server
MCP Server (stdio)  → AI tools can start/stop/monitor bots
```

- One process: `greedy serve`. Daemon + MCP share the same exchange and supervisor.
- Bots run autonomously in goroutines. AI controls via MCP tools.
- All state persists to SQLite (`~/.greedy/greedy.db`).

## Paper Trading Defaults

| Parameter | Value |
|-----------|-------|
| Initial USD balance | $100,000 |
| Default fee rate | 0.1% (0.001) |
| Default BTC-USD price | $50,000 |
| Tick interval | 100ms |
| Starting market | BTC-USD (only, until `add_market`) |

## Symbol Convention

All symbols use `BASE-QUOTE` format: `BTC-USD`, `ETH-USD`, `SOL-USD`.
Quote currency is always the second part. USD is the default quote.

## MCP Tools (12 total)

### Market Data
- **`get_ticker`** — Current price. Takes `symbol` (e.g. BTC-USD).
- **`get_order_book`** — Bids/asks with optional `depth` limit.
- **`get_candles`** — OHLCV data. `interval`: 1m, 5m, 15m, 1h, 4h, 1d.

### Trading
- **`place_order`** — Market or limit. `side`: buy/sell. `type`: market/limit.
  For market orders, omit `price`. Paper exchange fills instantly.
- **`cancel_order`** — Cancel by order ID.

### Account
- **`get_positions`** — All positions with quantity, avg entry price, P&L.
- **`get_balances`** — All asset balances (USD, BTC, ETH, etc.).

### Bot Management
- **`start_bot`** — Start from a YAML strategy file path (`strategy_file`).
  Supported types: dca, grid, signal. Bot runs in daemon, places orders autonomously.
- **`stop_bot`** — Stop a running bot by `bot_id`. Cancels all open orders.
- **`list_bots`** — All active bots with strategy, symbol, status. Use first,
  then `get_bot_status` for details.
- **`get_bot_status`** — Detailed status, config, P&L, open orders for one bot.

### Configuration
- **`add_market`** — Add a new trading symbol with simulated random-walk price feed.
  Auto-registered in price streamer so bots can trade it immediately.

### Tool Relationships

```
add_market → get_ticker → place_order
list_bots → get_bot_status
start_bot → list_bots → stop_bot
```

## Bot Lifecycle

```
STOPPED → STARTING → RUNNING → STOPPING → STOPPED
                    ↘ PAUSED ↗
                    ↘ ERROR (fatal)
```

- Bots persist their status in SQLite.
- On daemon restart, exchange state (balances, positions) is restored.
  Strategy state (DCA initial price, GRID levels) restarts fresh.
- `greedy status` CLI command lists all bots from SQLite.

## MCP Resources

- `portfolio://summary` — Live balances + positions JSON.
- `market://prices/{symbol}` — Live price for any trading symbol.
- `bot://{id}/status` — Bot status with open orders and P&L.

## Bot IDs

Format: `{strategyType}-{symbol}` or user-assigned in YAML.
Bot ID must be unique. Used in all bot management commands.

## Stream Layer

When `greedy serve` runs:
- **PriceStreamer**: One fetch goroutine per symbol, shared across all bots.
  Lock-free atomic cache for tick reads. 100ms interval.
- **MarketTracker**: Ring buffer per symbol with Δ% tracking. Circuit breaker
  activates if price moves >5% in 30s (cooldown 60s).
- **Debouncer**: Per-bot rate limiting. Default 5s cooldown, max 10 orders/30s.
  Configurable per strategy in YAML.
- **Idempotency**: ClientOrderID write-ahead reservation prevents duplicate orders
  on retry. Format: `{botID}-{unixMilli}-{seq}`.

## Known Limitations

- **Paper only**: No real exchange connections yet. All trades simulated.
- **Single client MCP**: stdio is 1:1. Multiple AI tools can't connect simultaneously.
- **No portfolio valuation**: Multi-currency P&L not calculated (Phase 2).
- **Strategy state not persisted**: DCA initialPrice, GRID levels reset on restart.
- **BTC-USD only at startup**: Other symbols must be added via `add_market` or YAML config.
