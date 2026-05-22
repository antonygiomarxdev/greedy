# Vertical Slicing Refactor Plan

## Estructura Final

```
cmd/greedy/main.go                  → composition root (wires everything)
internal/
├── shared/                          → dominio transversal compartido
│   ├── exchange.go                  (Exchange interface)
│   ├── types.go                     (Order, Ticker, Candle, OrderBook, OrderSide, etc)
│   ├── defaults.go                  (DefaultFeeRate, DefaultBasePrice, constants)
│   ├── market.go                    (MarketLifecycleManager interface)
│   ├── bot_config.go                (BotConfig interface)
│   └── errors.go                    (ErrSymbolNotFound, etc)
│
├── trading/                         → FEATURE: ejecución de bots
│   ├── domain.go                    (Strategy interface, Signal, BotState, Action, Status)
│   ├── bot.go                       (Bot runtime)
│   ├── supervisor.go                (orquestador)
│   ├── strategy/                    (DCA, GRID, Signal + builders + registry)
│   ├── usecase/                     (market_data, place_order, start_bot)
│   └── delivery/                    (RunCommand, StatusCommand CLI handlers)
│
├── backtest/                        → FEATURE: backtesting
│   ├── domain.go                    (Report, Trade, EquityPoint, Candle csv type)
│   ├── engine.go
│   ├── loader.go
│   ├── report.go
│   └── delivery/                    (BacktestCommand CLI handler)
│
├── mcp/                             → FEATURE: protocolo MCP
│   ├── domain.go                    (Command interface, tool name constants)
│   ├── server.go
│   ├── transport.go
│   ├── commands.go
│   ├── resources.go
│   └── delivery/                    (MCPServeCommand CLI handler)
│
├── infrastructure/                  → adapters de shared/
│   ├── paper/                       (PaperExchange)
│   ├── config/                      (YAML loader + config structs)
│   └── db/                          (SQLite)
│
├── crypto/                          → utilidad compartida
└── version/                         → build info
```

## Commit 1: Extract shared/ + move utilities (zero behavior change)

1. Create `internal/shared/types.go` — merge from `domain/exchange/types.go` + `domain/exchange_types.go`
2. Create `internal/shared/exchange.go` — Exchange interface  
3. Create `internal/shared/defaults.go` — constants + MarketLifecycleManager
4. Create `internal/shared/bot_config.go` — BotConfig interface (from domain/bot/types.go)
5. Create `internal/shared/errors.go` — sentinel errors
6. Add type aliases in old domain/ packages → shared/
7. Move `infrastructure/exchange/paper/` → `infrastructure/paper/`
8. Update all imports to use shared/ and new paper/ path
9. Build + test must pass

## Commit 2: Vertical slice restructure

1. `internal/trading/domain.go` — merge domain/bot/strategy.go + domain/bot/types.go + domain/strategy.go
2. `internal/trading/usecase/` — move get_market_data.go, place_order.go, start_bot.go
3. `internal/trading/delivery/cli.go` — merge cli/run.go + cli/status.go
4. `internal/backtest/domain.go` — rename types.go
5. `internal/backtest/delivery/cli.go` — move cli/backtest.go
6. `internal/mcp/domain.go` — merge tool names + Command interface
7. `internal/mcp/delivery/cli.go` — move cli/mcp_serve.go
8. Update cmd/greedy/main.go to import from feature delivery/ packages
9. Delete old packages: domain/, domain/bot/, domain/exchange/, domain/tool/, cli/
10. Delete internal/trading/strategy.go (type aliases), domain/bot/bot.go (unused interfaces)
11. Update all cross-references
12. Build + test must pass
