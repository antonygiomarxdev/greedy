# Agent Context for Greedy

Sovereign, local-first algorithmic trading engine. Single static binary. AI-native via MCP.

## Architecture

```
CLI (greedy serve) → daemon with exchange registry + supervisor + MCP server
MCP Server (stdio)  → AI agents control bots via 14 tools, 4 resources, 3 prompts
```

- One process: `greedy serve`. Daemon + MCP share registry and supervisor.
- Bots run autonomously in goroutines. AI controls them via MCP tools.
- All state persists to SQLite (`~/.greedy/greedy.db`).

### Stream Layer

When `greedy serve` runs:
- **PriceStreamer**: One fetch goroutine per symbol, shared across all bots. Lock-free atomic cache. 100ms interval.
- **MarketTracker**: Ring buffer per symbol with Δ% tracking. Circuit breaker activates if price moves >5% in 30s (cooldown 60s).
- **Debouncer**: Per-bot rate limiting. Default 5s cooldown, max 10 orders/30s. Configurable per strategy.
- **Idempotency**: ClientOrderID write-ahead reservation prevents duplicate orders on retry. Format: `{botID}-{unixMilli}-{seq}`.

## MCP Tools (14 total)

| Tool | Exchange param | Description |
|------|---------------|-------------|
| get_ticker | ✅ | Current price |
| get_order_book | ✅ | Order book depth |
| get_candles | ✅ | OHLCV history |
| place_order | ✅ | Market/limit orders |
| cancel_order | ✅ | Cancel by order ID |
| get_positions | ✅ | Positions + P&L |
| get_balances | ✅ | Account balances |
| get_order_history | ❌ | Query filled orders |
| start_bot | ❌ | Launch bot (YAML or inline) |
| stop_bot | ❌ | Stop running bot |
| list_bots | ❌ | Active bots + P&L |
| set_credential | ❌ | Store API keys |
| list_credentials | ❌ | List stored keys |
| delete_credential | ❌ | Remove stored keys |

Default exchange is `paper` when `exchange` is omitted.

## MCP Resources

| URI | Description |
|-----|-------------|
| `portfolio://summary` | Full portfolio snapshot: balances, positions, total P&L |
| `bot://{id}/status` | Bot status, strategy config, P&L, open orders |
| `bot://{id}/history` | Complete order history for a bot |
| `market://prices/{symbol}` | Real-time price for any symbol |

## MCP Prompts

| Name | Description |
|------|-------------|
| `analyze_portfolio` | Portfolio risk and exposure analysis |
| `review_trades` | Today's trading activity with P&L breakdown |
| `suggest_strategy` | Suggest DCA/GRID params based on market conditions |

## Bot Lifecycle

```
STOPPED → STARTING → RUNNING → STOPPING → STOPPED
                    ↘ PAUSED ↗
                    ↘ ERROR (fatal)
```

- Bots persist their status in SQLite.
- On daemon restart, exchange state (balances, positions) is restored. Strategy state restarts fresh.
- Bot IDs format: `{strategyType}-{symbol}` or user-assigned in YAML.

## Starting a Bot

Inline (no YAML):
```json
{"type": "dca", "symbol": "BTCUSDT", "exchange": "binance", "params": {"base_order_size": 100, "frequency": "1h"}}
```

Or via YAML:
```json
{"strategy_file": "examples/dca_btc.yaml"}
```

## Symbol Formats

- Paper/Coinbase: `BTC-USD`
- Binance: `BTCUSDT`

## Credentials

- Set via MCP: `set_credential` tool
- Set via CLI: `greedy credential set --exchange binance --label default --api-key XXX --api-secret YYY`
- Safer via env: `greedy credential set --exchange coinbase --label default --api-key-env COINBASE_KEY --api-secret-env COINBASE_SECRET`
- Requires `GREEDY_MASTER_PASSWORD` env var on both server and CLI

## Order Persistence

All orders placed by bots are automatically persisted in SQLite `orders` table. Query via `get_order_history` tool or `bot://{id}/history` resource.

## Paper Trading Defaults

| Parameter | Value |
|-----------|-------|
| Initial USD balance | $100,000 |
| Default fee rate | 0.1% |
| Default BTC-USD price | $50,000 |
| Tick interval | 100ms |

## Known Limitations

- `SubscribeOrderBook` returns "not yet implemented" for Binance and Coinbase.
- `ListPositions` on Binance returns nil (spot-only exchange).
- Argon2id key derivation uses nil salt — upgrading master password requires re-encrypting all stored credentials.
- Prebuilt binaries available via releases; no Homebrew tap yet.