# Greedy Architecture

Sovereign, local-first algorithmic trading engine. Clean Architecture + Vertical Slicing.

## Principles

| Principle | Rule |
|-----------|------|
| **Dependency Rule** | Dependencies point inward. Outer layers depend on inner layers. Inner layers know nothing about outer layers. |
| **Domain Isolation** | `domain/` imports **nothing** from other layers. Pure Go + stdlib only. |
| **Interface Segregation** | Small, focused interfaces. PriceStreamer ≠ MarketTracker ≠ RateLimiter. |
| **Dependency Inversion** | Use cases depend on domain interfaces. Infrastructure implements them. Composition root wires them. |
| **Vertical Slicing** | Each feature is a complete vertical through all layers. Independently testable, mergeable, deployable. |

---

## Layer Map

```
┌──────────────────────────────────────────────────────────────┐
│                      cmd/greedy/main.go                      │
│                   COMPOSITION ROOT (DI wiring)                │
│              Creates concrete implementations, wires them    │
│              into use cases, passes to delivery adapters     │
└──────┬──────────────────────────────────────────────┬────────┘
       │                                              │
       ▼                                              ▼
┌──────────────┐                            ┌──────────────────┐
│   DELIVERY   │  ← input adapters          │ INFRASTRUCTURE   │
│              │                            │                  │
│ delivery/mcp │  JSON-RPC over stdio       │ infra/exchange/  │
│ delivery/cli │  CLI subcommands           │   paper/         │
│              │                            │   coinbase/      │
│  Delegates   │                            │   binance/       │
│  to use      │                            │ infra/stream/    │
│  cases       │                            │   *_impl.go      │
│              │                            │ infra/db/        │
└──────┬───────┘                            │ infra/config/    │
       │                                    └────────┬─────────┘
       │                                             │
       ▼                                             │
┌──────────────┐                                     │
│   USECASES   │  ← application rules                │
│              │                                     │
│ start_bot.go │  StartBotUseCase                    │
│ stop_bot.go  │  StopBotUseCase                     │
│ place_order  │  PlaceOrderUseCase                  │
│ get_market   │  GetMarketDataUseCase               │
│ get_portfolio│  GetPortfolioUseCase                │
│ monitor      │  MarketMonitoringUseCase            │
│ reconcile    │  ReconcileOrdersUseCase              │
│              │                                     │
│  Depends ONLY on domain interfaces                 │
└──────┬───────┘                                     │
       │                                             │
       ▼                                             ▼
┌──────────────────────────────────────────────────────────────┐
│                          DOMAIN                               │
│                  pure Go interfaces + types                   │
│                                                              │
│  domain/exchange/   Exchange interface, Order, Balance...    │
│  domain/bot/        Bot, Supervisor interfaces               │
│  domain/strategy/   Strategy interface, config types         │
│  domain/stream/     PriceStreamer, MarketTracker,            │
│                     RateLimiter, Debouncer, Portfolio...      │
│                                                              │
│  Imports NOTHING from other layers                           │
└──────────────────────────────────────────────────────────────┘
```

---

## Target Directory Structure

```
cmd/greedy/                         # Composition root only
  main.go                           # DI wiring — creates implementations, wires use cases

internal/
  domain/                           # Enterprise business rules
    exchange/
      exchange.go                   # Exchange interface (13 methods)
      types.go                      # Order, Balance, Position, Ticker, Candle...
      errors.go                     # ErrRateLimited, ErrAuthFailed...
      profile.go                    # RateLimitProfile (domain type, not impl)
    bot/
      bot.go                        # Bot interface
      supervisor.go                 # Supervisor interface
      status.go                     # BotStatus type
    strategy/
      strategy.go                   # Strategy interface (Evaluate, OnFill, OnCancel)
      types.go                      # StrategyConfig, DCAConfig, GridConfig...
    stream/
      price_streamer.go             # PriceStreamer interface
      market_tracker.go             # MarketTracker interface
      rate_limiter.go               # RateLimiter interface
      debouncer.go                  # Debouncer interface
      idempotency.go                # IdempotencyStore interface
      portfolio.go                  # PortfolioValuator interface
      order_reconciler.go           # OrderReconciler interface
      types.go                      # CachedTicker, PricePoint, BreakerEvent...

  usecases/                         # Application business rules
    start_bot.go                    # StartBotUseCase
    stop_bot.go                     # StopBotUseCase
    place_order.go                  # PlaceOrderUseCase (idempotent)
    get_market_data.go              # GetTickerUseCase, GetOrderBookUseCase...
    get_portfolio.go                # GetPortfolioUseCase
    monitor_market.go               # MarketMonitoringUseCase
    reconcile_orders.go             # ReconcileOrdersUseCase

  delivery/                         # Input adapters
    mcp/
      server.go                     # MCP JSON-RPC server
      transport.go                  # Stdio transport
      tools.go                      # Tool definitions + schemas
    cli/
      run.go                        # Run command handler
      backtest.go                   # Backtest command handler
      serve.go                      # MCP serve command handler
      version.go                    # Version command handler

  infrastructure/                   # Implementations of domain interfaces
    exchange/
      profile.go                    # ProviderDefaults (rate limit profiles)
      paper/
        paper.go                    # PaperExchange
        feed.go                     # Price feeds
        orderbook.go                # Order book matching engine
      coinbase/
        coinbase.go                 # CoinbaseExchange
        auth.go                     # HMAC signing
        rest.go                     # HTTP + rate limiting
      binance/
        binance.go                  # BinanceExchange
    stream/
      price_streamer_impl.go        # WebSocket-first + polling fallback
      market_tracker_impl.go        # Ring buffer + circuit breaker
      rate_limiter_impl.go          # Token bucket + weight-based
      debouncer_impl.go             # Per-bot cooldown
      idempotency_impl.go           # ClientOrderID reservation
      portfolio_impl.go             # Multi-currency valuation
      order_reconciler_impl.go      # Periodic reconciliation
      price_store.go                # SQLite price persistence
    db/
      sqlite.go                     # Connection + WAL mode
      migrations.go                 # Schema migrations
      bot_repo.go                   # BotRepository
      config_repo.go                # ConfigRepository
    config/
      loader.go                     # YAML strategy loader
      strategy_builder.go           # Shared BuildStrategy() factory
```

---

## Vertical Slicing

Every feature is implemented as a complete vertical through all layers. A slice is never just "add an interface" or "add a DB table" — it's the full stack.

### Slice Template

```
Slice: {Feature Name}

1. domain/        → Define the interface + domain types
2. infrastructure/→ Implement the interface
3. usecases/      → Create/update use cases that consume the interface
4. delivery/      → Wire into MCP tools or CLI commands
5. cmd/greedy/    → Wire into composition root (DI)
```

### Example: RateLimiter Slice

```
domain/stream/rate_limiter.go          ← RateLimiter interface
domain/exchange/profile.go             ← RateLimitProfile domain type

infrastructure/stream/rate_limiter_impl.go  ← Token bucket + Binance weight impl
infrastructure/exchange/profile.go     ← ProviderDefaults

usecases/place_order.go                ← PlaceOrderUseCase depends on RateLimiter
usecases/get_market_data.go            ← GetTickerUseCase depends on RateLimiter

delivery/mcp/server.go                 ← MCP tools use use cases (no direct limiter access)

cmd/greedy/main.go                     ← Wire: exchange → limiter → use cases
```

**This entire slice is independently testable and mergeable.** It doesn't break anything. It doesn't depend on any other stream component being ready.

### Slice Dependency Order

```
Phase 0: Restructure existing code (ZERO new features)
  #29 → domain interfaces
  #30 → use cases
  #31 → infrastructure reorganization
  #32 → delivery layer

Phase 1: Stream layer
  RateLimiter        → no deps (standalone)
  PriceStreamer      → depends on RateLimiter
  MarketTracker      → depends on PriceStreamer
  Idempotency        → no deps (standalone)
  Debouncer+Health   → no deps (standalone)
  OrderReconciler    → depends on Idempotency
  Portfolio          → depends on PriceStreamer + MarketTracker
  Bot Integration    → depends on ALL previous slices
```

Each slice builds on the previous but is independently deployable. You could ship the RateLimiter slice on a Friday and the MarketTracker slice on Monday — no coupling.

---

## SOLID in Practice

### Single Responsibility

```go
// domain/stream/rate_limiter.go — does ONE thing
type RateLimiter interface {
    Wait(ctx context.Context, endpoint string) error
    Shutdown()
}

// domain/stream/market_tracker.go — does ONE thing
type MarketTracker interface {
    Record(symbol string, price float64, ts time.Time)
    IsBreakerActive(symbol string) bool
    GetDelta(symbol string, horizon time.Duration) (float64, error)
}
```

Not one `StreamManager` interface that does everything. Small, focused interfaces.

### Open/Closed

```go
// domain/exchange/exchange.go
type Exchange interface { /* 13 methods */ }

// infrastructure/exchange/paper/paper.go
type PaperExchange struct { /* implements Exchange */ }

// infrastructure/exchange/coinbase/coinbase.go
type CoinbaseExchange struct { /* implements Exchange */ }
```

Add a new exchange without touching use cases. Use cases depend on `Exchange`, not `*PaperExchange` or `*CoinbaseExchange`.

### Liskov Substitution

```go
// usecases/start_bot.go
type StartBotUseCase struct {
    exchange domain_exchange.Exchange  // ANY Exchange works
}

// Tests:
func TestStartBotUseCase(t *testing.T) {
    mockExchange := &MockExchange{}           // satisfies Exchange
    uc := NewStartBotUseCase(mockExchange, ...)
    bot, err := uc.Execute(ctx, cfg)
    // Works with ANY implementation of Exchange
}
```

### Interface Segregation

```go
// BAD: one fat interface
type StreamManager interface {
    Wait(...)    // rate limiting
    Register(...) // streamer
    Record(...)   // tracker
    // ... 10 more methods
}

// GOOD: small interfaces
type RateLimiter interface { Wait(...) }
type PriceStreamer interface { Register(...); GetCached(...) }
type MarketTracker interface { Record(...); IsBreakerActive(...) }
```

### Dependency Inversion

```go
// usecases/place_order.go — depends on ABSTRACTION
type PlaceOrderUseCase struct {
    exchange    domain_exchange.Exchange    // interface
    rateLimiter domain_stream.RateLimiter   // interface
    debouncer   domain_stream.Debouncer     // interface
    idempotency domain_stream.IdempotencyStore // interface
}

// cmd/greedy/main.go — depends on CONCRETION (wiring only)
func main() {
    exchange := paper.New(...)
    limiter := streamimpl.NewRateLimiter(exchange.RateLimitProfile())
    debounce := streamimpl.NewDebouncer(...)
    idemp    := streamimpl.NewIdempotencyStore(db)

    uc := usecases.NewPlaceOrderUseCase(exchange, limiter, debounce, idemp)
}
```

The use case NEVER imports `infrastructure/`. It only imports `domain/`.

---

## Testing Strategy by Layer

| Layer | Test Type | Tool | Mock Strategy |
|-------|-----------|------|---------------|
| **domain** | Interface contract tests | Go test | Table-driven, no mocks needed |
| **usecases** | Unit tests with mocked deps | Go test + manual mocks | Each use case receives mock interfaces |
| **infrastructure** | Integration tests | Go test + real deps | SQLite in-memory, paper exchange, real HTTP for coinbase |
| **delivery** | E2E tests | Go test + stdio | Real MCP server + paper exchange |
| **cmd** | Smoke tests | Shell script | `greedy version`, `greedy mcp-serve < test.json` |

### Why No Mock Framework

Manual mocks in Go are simple, explicit, and compile-time safe:

```go
type mockExchange struct {
    getTickerFn func(ctx context.Context, symbol string) (*exchange.Ticker, error)
}

func (m *mockExchange) GetTicker(ctx context.Context, symbol string) (*exchange.Ticker, error) {
    return m.getTickerFn(ctx, symbol)
}
```

No `gomock`, no `testify/mock`. Pure Go, zero-magic, easy to debug.

---

## Composition Root Pattern

`cmd/greedy/main.go` is the ONLY place where concrete implementations are created and wired together. It's a single file with one purpose: build the object graph.

```go
func main() {
    // 1. Parse CLI flags
    // 2. Create infrastructure
    db := db.Open(...)
    exchange := exchangeFactory(cfg)
    streamer := streamimpl.NewPriceStreamer(exchange, db)
    tracker := streamimpl.NewMarketTracker(db, streamer)
    limiter := streamimpl.NewRateLimiter(exchange.RateLimitProfile())

    // 3. Create use cases (inject interfaces)
    startBotUC := usecases.NewStartBotUseCase(exchange, streamer, tracker, db)
    placeOrderUC := usecases.NewPlaceOrderUseCase(exchange, limiter, debouncer, idemp)
    portfolioUC := usecases.NewGetPortfolioUseCase(exchange, streamer, db)

    // 4. Wire into delivery adapters
    handler := cli.NewHandler(startBotUC, placeOrderUC, portfolioUC)
    mcpServer := mcp.NewServer(startBotUC, placeOrderUC, portfolioUC)

    // 5. Dispatch
    switch cmd {
    case "run": handler.Run()
    case "mcp-serve": mcpServer.Serve()
    }
}
```

No business logic in `main.go`. Just wiring. This file is the ONLY place that imports `infrastructure/` directly.

---

## Current State → Target State

| Current | Target | Phase |
|---------|--------|-------|
| `internal/exchange/exchange.go` | `internal/domain/exchange/exchange.go` | #29 |
| `internal/exchange/paper/` | `internal/infrastructure/exchange/paper/` | #31 |
| `internal/bot/bot.go` (concrete) | `internal/domain/bot/bot.go` (interface) | #29 |
| `internal/mcp/server.go` (has business logic) | `internal/delivery/mcp/server.go` (delegates to use cases) | #30 + #32 |
| `cmd/greedy/main.go` (builds strategies) | `cmd/greedy/main.go` (wires use cases) | #30 |
| No `usecases/` | `internal/usecases/start_bot.go`, `place_order.go`... | #30 |
| No `domain/` | `internal/domain/` with interfaces only | #29 |
| No stream layer | `internal/domain/stream/` + `internal/infrastructure/stream/` | Phase 1 |

---

## Rules for Contributors

1. **New features start in `domain/`**. Define the interface first. Then implement.
2. **Use cases NEVER import `infrastructure/`**. They depend on `domain/` interfaces only.
3. **Infrastructure ALWAYS implements a `domain/` interface**. No orphan implementations.
4. **One file = one responsibility**. If `start_bot.go` has 3 interfaces, split into 3 files.
5. **Tests live alongside code**. `domain/stream/rate_limiter.go` → `domain/stream/rate_limiter_test.go`.
6. **No global state**. Everything passed via constructor injection.
7. **No `init()` functions**. All initialization is explicit in `cmd/greedy/main.go`.
8. **Exported names are meaningful**. `NewRateLimiter` is fine. `New` is not — there are many `New`s in different packages. Use package name disambiguation: `ratelimiter.New()`, `streamer.New()`.

---

## References

- [Clean Architecture — Robert C. Martin](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Vertical Slice Architecture — Jimmy Bogard](https://www.jimmybogard.com/vertical-slice-architecture/)
- [SOLID Principles in Go](https://dave.cheney.net/2016/08/20/solid-go-design)
