# Agent Context for Greedy

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

## MCP Resources
- `portfolio://summary` — balances + positions
- `bot://{id}/status` — bot status + P&L
- `bot://{id}/history` — order history for a bot

## Symbol Formats
- Paper/Coinbase: `BTC-USD`
- Binance: `BTCUSDT`

## Starting a Bot

Inline (no YAML):
```json
{"type": "dca", "symbol": "BTCUSDT", "exchange": "binance", "params": {"base_order_size": 100, "frequency": "1h"}}
```

## Credentials
- Set via MCP: `set_credential` tool
- Set via CLI: `greedy credential set --exchange binance --label default --api-key XXX --api-secret YYY`
- Requires `GREEDY_MASTER_PASSWORD` env var on both server and CLI

## Order Persistence
All orders placed by bots are automatically persisted in SQLite `orders` table. Query via `get_order_history` tool or `bot://{id}/history` resource.
