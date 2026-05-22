# Greedy Architecture

Sovereign, local-first algorithmic trading engine. Clean Architecture + Vertical Slicing.

## Principles

| Principle | Rule |
|-----------|------|
| **Vertical Slicing** | Each feature is a complete vertical through all layers. Self-contained in its own directory. |
| **Shared Domain** | `shared/` holds ONLY cross-cutting interfaces and types. No business logic. |
| **Feature Autonomy** | A feature never imports another feature's internals. They communicate through `shared/` interfaces. |
| **Dependency Rule** | Features depend on `shared/` interfaces. Infrastructure implements them. `cmd/greedy/main.go` wires everything. |

## Directory Structure

```
cmd/greedy/main.go                  → Composition root (DI wiring ONLY)
internal/
  shared/                           → Cross-cutting domain types
    exchange.go                     (Exchange interface — 13 methods)
    types.go                        (Order, Ticker, Candle, OrderBook, Balance, Position...)
    defaults.go                     (Default constants + MarketLifecycleManager interface)
    bot_config.go                   (BotConfig interface)
    errors.go                       (Sentinel errors)

  trading/                          → FEATURE: Live bot execution
    domain.go                       (Strategy interface, Signal, BotState, Action)
    bot.go                          (Bot runtime — tick loop, order execution)
    supervisor.go                   (Multi-bot orchestration, restart policies)
    strategy/                       (Implementations: DCA, GRID, Signal + builders + registry)
    usecase/                        (StartBot, PlaceOrder, GetMarketData)
    delivery/cli.go                 (CLI: run, status commands)

  backtest/                         → FEATURE: Historical backtesting
    domain.go                       (Backtest Candle type)
    engine.go                       (Simulation engine over CSV data)
    loader.go                       (CSV parser)
    report.go                       (Report formatter: text, JSON)
    delivery/cli.go                 (CLI: backtest command)

  mcp/                              → FEATURE: MCP protocol server (AI agent interface)
    domain.go                       (Command interface, 12 tool name constants)
    server.go                       (Stdio JSON-RPC server)
    transport.go                    (Line-delimited JSON transport)
    commands.go                     (12 tool implementations)
    resources.go                    (MCP resources)
    delivery/cli.go                 (CLI: mcp-serve command)

  infrastructure/                   → Adapters implementing shared/ interfaces
    paper/                          (PaperExchange — simulated market)
    config/                         (YAML strategy loader)
    db/                             (SQLite + repositories)

  crypto/                           → Shared utility (encryption)
  version/                          → Build info (ldflags)
```

## Section Map: Vertical Slicing

### What Each Feature Contains

```
trading/
  ├── domain.go       → Feature-specific domain (Strategy, Signal, BotState)
  ├── bot.go          → Application logic (tick loop, order handling)
  ├── supervisor.go   → Application logic (multi-bot orchestration)
  ├── strategy/       → Domain implementations (DCA, GRID, Signal strategies)
  ├── usecase/        → Use cases (StartBot, PlaceOrder, GetMarketData)
  └── delivery/cli.go → Delivery (CLI handlers for "run" and "status")

backtest/
  ├── domain.go       → Feature-specific types (Candle)
  ├── engine.go       → Application logic (simulation engine)
  ├── loader.go       → Infrastructure (CSV loader)
  ├── report.go       → Delivery (report formatting)
  └── delivery/cli.go → Delivery (CLI handler for "backtest")

mcp/
  ├── domain.go       → Feature-specific domain (Command interface, tool names)
  ├── server.go       → Application logic (JSON-RPC server)
  ├── transport.go    → Infrastructure (stdio transport)
  ├── commands.go     → Application logic (12 tool implementations)
  └── delivery/cli.go → Delivery (CLI handler for "mcp-serve")
```

**Key rule**: Open `trading/` and you see everything about running a bot. Open `backtest/` and you see everything about backtesting. No horizontal-layer hopping.

## Dependency Flow

```
cmd/greedy/main.go  ──wires──►  shared/  ◄──implements──  infrastructure/
       │                            ▲
       │                            │
       └────imports────────────────►│
       │                            │
       ▼                            │
   trading/delivery                 │
   backtest/delivery                │
   mcp/delivery                     │
       │                            │
       └────depends on──────────────┘
            (through shared interfaces)
```

- `shared/` imports NOTHING from `internal/` (zero deps)
- `trading/`, `backtest/`, `mcp/` import `shared/` + `infrastructure/` (through interfaces)
- `infrastructure/paper/` implements `shared.Exchange`
- `infrastructure/config/` implements `shared.BotConfig`
- `cmd/greedy/main.go` is the ONLY place that creates concrete implementations

## Composition Root: cmd/greedy/main.go

```go
func main() {
    // 1. Parse CLI flags → determine subcommand
    // 2. Each subcommand handler:
    //    - Creates infrastructure (paper.New, db.Open, config.Load)
    //    - Creates feature components (trading.NewSupervisor, mcp.NewServer)
    //    - Delegates to feature delivery (tradingdelivery.RunCommand, etc.)
    // 3. NO business logic in main.go — just wiring and delegation
}
```

## Testing Strategy

| Layer | Test Type | Location |
|-------|-----------|----------|
| **Domain interfaces** | No tests needed (pure types) | — |
| **Strategy implementations** | Unit tests | `trading/strategy/dca_test.go` |
| **Application logic** | Integration tests | `trading/bot_test.go`, `infrastructure/paper/paper_test.go` |
| **Delivery** | Manual smoke tests | CLI: `greedy run`, `greedy backtest`, `greedy mcp-serve` |

## Design Patterns Applied

| Pattern | Where | Why |
|---------|-------|-----|
| **Registry + Strategy** | `trading/strategy/registry.go` | Dynamic strategy lookup — no switch/magic strings |
| **Builder** | `trading/strategy/builder_dca.go` | Config → Strategy construction with validation |
| **Command** | `mcp/domain.go` + `mcp/commands.go` | Each MCP tool is a Command with Execute() |
| **Observer** | `trading/domain.go` (OrderConfirmer, OrderFilledListener) | Strategy callbacks without coupling |
| **Factory Method** | `trading/strategy/registry.go` (Build function) | Strategy instantiation through registry |
| **Strategy** | `trading/strategy/dca.go`, `grid.go`, `signal.go` | Interchangeable trading algorithms |
| **Composition Root** | `cmd/greedy/main.go` | Single point of object graph construction |
| **Template Method** | `mcp/commands.go` (struct embedding) | Shared command behavior (Name, Description, Schema) |
| **Dependency Injection** | Throughout | Constructor injection, no global state |
