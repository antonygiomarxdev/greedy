# Greedy Architecture

Sovereign, local-first algorithmic trading engine. Clean Architecture + Vertical Slicing.

## Principles

| Principle | Rule |
|-----------|------|
| **Vertical Slicing** | Each feature is a complete vertical through all layers. Self-contained in its own directory. |
| **Shared Domain** | `shared/` holds ONLY cross-cutting interfaces and types. No business logic. |
| **Feature Autonomy** | A feature never imports another feature's internals. They communicate through `shared/` interfaces. |
| **Dependency Rule** | Features depend on `shared/` interfaces. Infrastructure implements them. `cmd/greedy/main.go` wires everything. |
| **Paper-First** | All features work with PaperExchange. Interfaces allow real exchange swap later. |
| **Lock-Free Hot Path** | Caches on the hot path (bot tick → price read) use `atomic.Value`, not mutexes. |

## Directory Structure

```
cmd/greedy/main.go                  → Composition root (DI wiring ONLY)
internal/
  shared/                           → Cross-cutting domain types
    exchange.go                     (Exchange interface — 13 methods)
    types.go                        (Order, Ticker, Candle, OrderBook, Balance, Position...)
    defaults.go                     (Default constants + MarketLifecycleManager interface)
    errors.go                       (Sentinel errors)

  trading/                          → FEATURE: Live bot execution
    domain.go                       (Strategy interface, Signal, BotState, Action)
    bot.go                          (Bot runtime — tick loop, order execution)
    supervisor.go                   (Multi-bot orchestration, restart policies)
    strategy/                       (Implementations: DCA, GRID, Signal + builders + registry)
    usecase/                        (StartBot, PlaceOrder, GetMarketData)
    delivery/
      cli.go                        (CLI: run, status commands)
      serve.go                      (CLI: serve daemon — wires all components)

  backtest/                         → FEATURE: Historical backtesting
    domain.go                       (Candle type)
    engine.go                       (Simulation engine over CSV data)
    loader.go                       (CSV parser)
    report.go                       (Report formatter: text, JSON)
    delivery/cli.go                 (CLI: backtest command)

  mcp/                              → FEATURE: MCP protocol server (AI agent interface)
    domain.go                       (Command interface, tool name constants)
    server.go                       (Stdio JSON-RPC server, resource/read, prompts/get)
    transport.go                    (Line-delimited JSON transport, 8 RPC handlers)
    commands.go                     (12 tool implementations)
    resources.go                    (MCP resources + prompts)
    delivery/cli.go                 (CLI: mcp-serve command)

  kernel/                           → Cross-cutting persistence
    persistence.go                  (Exchange state snapshot/restore to SQLite)

  infrastructure/                   → Adapters implementing shared/ interfaces
    paper/                          (PaperExchange — simulated market)
    config/                         (YAML strategy loader)
    db/                              (SQLite + repositories + migrations)

  crypto/                           → Shared utility (encryption)
  version/                          → Build info (ldflags)

  --- Stream Layer (vertical slices) ---
  pricestore/                       → FEATURE: SQLite price history
    domain.go                       (PricePoint, PriceStore interface)
    sqlite.go                       (Insert, QueryWindow, Prune)

  pricestreamer/                    → FEATURE: Shared price fetch per symbol
    domain.go                       (CachedTicker, PriceStreamer interface)
    streamer.go                     (RefCount registration, fetch goroutines, atomic cache)

  markettracker/                    → FEATURE: Price tracking + circuit breaker
    domain.go                       (MarketSnap, BreakerConfig, MarketTracker interface)
    tracker.go                      (Ring buffer, Δ% calculation, breaker activation, warm-start restore)

  debouncer/                        → FEATURE: Per-bot order rate limiting
    domain.go                       (Debouncer interface)
    debouncer.go                    (Cooldown + burst window)

  idempotency/                      → FEATURE: At-most-once order execution
    domain.go                       (Store interface)
    sqlite.go                       (Reserve, Confirm, Lookup — ClientOrderID dedup)
```

## Section Map: Vertical Slicing

### Core Features

```
trading/
  ├── domain.go       → Strategy, Signal, BotState, ActionHold/Buy/Sell
  ├── bot.go          → Tick loop (100ms), order handling, reconcile, shutdown
  ├── supervisor.go   → Multi-bot orchestration, WaitGroup lifecycle
  ├── strategy/       → DCA, GRID, Signal implementations + typed registry
  ├── usecase/        → StartBot, PlaceOrder, GetMarketData
  └── delivery/
      ├── cli.go      → run, status CLI commands
      └── serve.go    → serve daemon: wires exchange + stream layer + supervisor + MCP

backtest/
  ├── domain.go       → Candle type
  ├── engine.go       → Simulation engine over CSV data
  ├── loader.go       → CSV parser
  ├── report.go       → Text/JSON report formatter
  └── delivery/cli.go → backtest CLI command

mcp/
  ├── domain.go       → Command interface, tool name constants
  ├── server.go       → JSON-RPC server, tool dispatch, resource/read, prompts/get
  ├── transport.go    → Stdio transport, 8 RPC handlers (init, ping, tools/*, resources/*, prompts/*)
  ├── commands.go     → 12 tool implementations (get_ticker, place_order, start_bot, …)
  ├── resources.go    → Static resource/prompt registration (portfolio://summary, …)
  └── delivery/cli.go → mcp-serve CLI command
```

### Stream Layer Features

```
pricestore/
  ├── domain.go       → PricePoint, PriceStore interface
  └── sqlite.go       → Insert, QueryWindow, Prune — SQLite-backed

pricestreamer/
  ├── domain.go       → CachedTicker (atomic.Value), PriceStreamer interface
  └── streamer.go     → RefCount registration, one fetch goroutine per symbol,
                        optional PriceStore + OnTick callback

markettracker/
  ├── domain.go       → MarketSnap, BreakerConfig, MarketTracker interface
  └── tracker.go      → Ring buffer (256 capacity), sliding window Δ%,
                        circuit breaker activation/deactivation, warm-start from PriceStore

debouncer/
  ├── domain.go       → Debouncer interface
  └── debouncer.go    → Cooldown + burst window, per-bot instance

idempotency/
  ├── domain.go       → Store interface
  └── sqlite.go       → Reserve/Confirm/Lookup — ClientOrderID write-ahead reservation
```

## Dependency Flow

```
cmd/greedy/main.go
  │
  └─► serve command (trading/delivery/serve.go)
        │
        ├─► infrastructure/paper (PaperExchange)
        ├─► infrastructure/db (SQLite + migrations)
        ├─► pricestore (SQLite price history)
        ├─► pricestreamer (shared fetch, atomic cache)
        │     └─► OnTick callback ──► markettracker (ring buffer, circuit breaker)
        ├─► kernel (exchange state snapshot/restore)
        ├─► idempotency (ClientOrderID dedup)
        ├─► trading/supervisor
        │     └─► StartBot ──► creates debouncer per bot, injects streamer + tracker + idempotency
        └─► mcp/server (goroutine, shares same exchange + supervisor)
```

### Bot Tick Flow

```
Bot.tick() [every 100ms]
  │
  ├─► streamer.GetCached(symbol)        → lock-free atomic read (no exchange call)
  ├─► tracker.IsBreakerActive(symbol)   → skip if circuit breaker on
  ├─► exchange.GetPosition/GetBalance   → remaining exchange calls
  ├─► strategy.Evaluate(state)          → ActionHold | ActionBuy | ActionSell
  ├─► debouncer.CanExecute()            → skip if cooldown or burst limit
  ├─► idempotency.Reserve(clientID)     → write-ahead before place
  ├─► exchange.PlaceOrder(req)          → actual order
  ├─► debouncer.RecordExecution()       → track for rate limiting
  └─► idempotency.Confirm(clientID)     → mark as confirmed after fill
```

## Composition Root: cmd/greedy/main.go

```go
func main() {
    // 1. Parse CLI flags → determine subcommand
    // 2. Each subcommand handler delegates to feature delivery:
    //    serve   → tradingdelivery.ServeCommand
    //    run     → tradingdelivery.RunCommand (legacy wrapper)
    //    backtest → backtestdelivery.BacktestCommand
    //    mcp-serve → mcpdelivery.MCPServeCommand (legacy wrapper)
    //    status  → tradingdelivery.StatusCommand
    // 3. NO business logic in main.go — just wiring and delegation
}
```

## Design Patterns Applied

| Pattern | Where | Why |
|---------|-------|-----|
| **Vertical Slicing** | `trading/`, `backtest/`, `mcp/`, stream packages | Complete features in one directory, no layer hopping |
| **Registry + Strategy** | `trading/strategy/registry.go` | Dynamic strategy lookup — no switch/magic strings |
| **Builder** | `trading/strategy/builder_dca.go` | Config → Strategy construction with validation |
| **Command** | `mcp/domain.go` + `mcp/commands.go` | Each MCP tool is a Command with Execute() |
| **Observer** | `trading/domain.go` (OrderConfirmer, OrderFilledListener) | Strategy callbacks without coupling |
| **Factory Method** | `trading/strategy/registry.go` (Build function) | Strategy instantiation through registry |
| **Composition Root** | `cmd/greedy/main.go` | Single point of object graph construction |
| **Template Method** | `mcp/commands.go` (struct embedding) | Shared command behavior (Name, Description, Schema) |
| **Dependency Injection** | Throughout | Constructor injection, no global state |
| **Atomic Cache** | `pricestreamer/streamer.go` | `atomic.Value` for lock-free reads on hot path |
| **Reference Counting** | `pricestreamer/streamer.go` | Goroutine lifecycle tied to active consumers |
| **Circuit Breaker** | `markettracker/tracker.go` | Blocks orders on excessive price movement |
| **Write-Ahead Reservation** | `idempotency/sqlite.go` | Reserve ClientOrderID before PlaceOrder |
| **Ring Buffer** | `markettracker/tracker.go` | Fixed-memory sliding window for price tracking |
