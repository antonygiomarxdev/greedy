# Greedy — Clean Architecture Migration Plan

> 2026-05-22 | Status: DRAFT

## Target Architecture

```
cmd/greedy/                    # Composition root (DI wiring only, zero logic)
  main.go                      # wires all dependencies
  run.go                       # CLI handler → delegates to use cases
  backtest.go                  # CLI handler → delegates to use cases
  mcp.go                       # CLI handler → delegates to use cases

internal/
  domain/                      # Enterprise business rules (pure Go, zero imports)
    exchange/
      exchange.go              # Exchange interface (already exists ✅)
      types.go                 # Order, Balance, Position, etc. (already exists ✅)
      errors.go                # Domain errors (already exists ✅)
    bot/
      bot.go                   # Bot interface
      supervisor.go            # Supervisor interface
      status.go                # BotStatus type
    strategy/
      strategy.go              # Strategy interface (already ✅)
      types.go                 # StrategyConfig, params
    stream/
      price_streamer.go        # PriceStreamer interface
      market_tracker.go        # MarketTracker interface
      rate_limiter.go          # RateLimiter interface
      debouncer.go             # Debouncer interface
      idempotency.go           # IdempotencyStore interface
      portfolio.go             # PortfolioValuator interface
      order_reconciler.go      # OrderReconciler interface
      types.go                 # CachedTicker, PricePoint, BreakerEvent, etc.

  usecases/                    # Application business rules (depends on domain interfaces)
    start_bot.go               # StartBotUseCase
    stop_bot.go                # StopBotUseCase
    place_order.go             # PlaceOrderUseCase (idempotent)
    get_market_data.go         # GetTickerUseCase, GetOrderBookUseCase
    get_portfolio.go           # GetPortfolioUseCase
    monitor_market.go          # MarketMonitoringUseCase (observes streamer + tracker)

  infrastructure/              # Implementations of domain interfaces
    exchange/
      paper/                   # PaperExchange (already ✅)
      coinbase/                # CoinbaseExchange (planned)
      binance/                 # BinanceExchange (planned)
      profile.go               # RateLimitProfile + ProviderDefaults
    stream/
      price_streamer_impl.go   # implements domain/stream.PriceStreamer
      market_tracker_impl.go   # implements domain/stream.MarketTracker
      rate_limiter_impl.go     # implements domain/stream.RateLimiter
      debouncer_impl.go        # implements domain/stream.Debouncer
      idempotency_impl.go      # implements domain/stream.IdempotencyStore
      portfolio_impl.go        # implements domain/stream.PortfolioValuator
      order_reconciler_impl.go # implements domain/stream.OrderReconciler
      price_store.go           # SQLite persistence (infrastructure detail)
    db/
      sqlite.go                # SQLite connection + migrations
      bot_repo.go              # implements domain/bot.BotRepository
      config_repo.go           # implements domain/bot.ConfigRepository
    config/
      loader.go                # YAML strategy loader
      strategy_builder.go      # BuildStrategy factory function

  delivery/                    # Input adapters
    mcp/
      server.go                # MCP JSON-RPC server (depends on use cases)
      transport.go             # Stdio transport
      tools.go                 # Tool definitions
    cli/
      run.go                   # Run command handler
      backtest.go              # Backtest command handler
      serve.go                 # MCP serve command handler
```

## SOLID Principles Applied

| Principle | How |
|-----------|-----|
| **S** — Single Responsibility | Each use case does ONE thing. Each domain interface defines ONE capability. |
| **O** — Open/Closed | Add new exchanges via Exchange interface without touching use cases. Add new strategies via Strategy interface. |
| **L** — Liskov | PaperExchange and CoinbaseExchange are substitutable — use cases don't know which one they have. |
| **I** — Interface Segregation | RateLimiter ≠ PriceStreamer ≠ MarketTracker — small, focused interfaces. |
| **D** — Dependency Inversion | Use cases depend on domain interfaces. Infrastructure implements them. `cmd/greedy` wires them. |

## Migration Phases

### Phase 0: Restructure existing code (ZERO new features)

Create the directory structure and move existing code into it. All tests must still pass. No new behavior.

#### Issue #M0-1: Extract domain/interfaces.go
- Create `internal/domain/exchange/` — move `exchange.go`, `types.go`, `errors.go`
- Create `internal/domain/bot/` — extract `Bot`, `Supervisor` interfaces
- Create `internal/domain/strategy/` — extract `Strategy` interface + config types
- Update all imports across codebase
- All existing tests pass

#### Issue #M0-2: Extract use cases from cmd/ and internal/mcp/
- Create `internal/usecases/start_bot.go` — extract bot creation logic from `cmd/greedy/main.go` and `internal/mcp/server.go`
- Create `internal/usecases/place_order.go` — extract order placement logic
- Create `internal/usecases/get_market_data.go` — extract market data queries
- CLI and MCP handlers delegate to use cases
- All existing tests pass

#### Issue #M0-3: Reorganize infrastructure
- Move `internal/exchange/paper/` → `internal/infrastructure/exchange/paper/`
- Move `internal/db/` → `internal/infrastructure/db/`
- Move `internal/config/` → `internal/infrastructure/config/`
- Update all imports
- All existing tests pass

#### Issue #M0-4: Move MCP to delivery
- Move `internal/mcp/` → `internal/delivery/mcp/`
- Move CLI handlers from `cmd/greedy/*.go` → `internal/delivery/cli/`
- `cmd/greedy/main.go` becomes pure wiring
- All existing tests pass

**Gate: 71 tests pass, build succeeds, no MCP E2E regression.**

### Phase 1: Stream layer — 1 slice at a time

Each slice delivers a complete vertical: domain interface → infrastructure implementation → use case → delivery integration.

#### Issue #M1-1: RateLimiter slice
```
domain/stream/rate_limiter.go          ← RateLimiter interface
domain/exchange/profile.go             ← RateLimitProfile domain type
infrastructure/stream/rate_limiter_impl.go ← token bucket + weight-based impl
infrastructure/exchange/profile.go     ← ProviderDefaults (paper, coinbase, binance)
usecases/place_order.go                ← PlaceOrderUseCase consumes RateLimiter
delivery/mcp/server.go                 ← No change (use case handles it)
```
**Verification:** Paper exchange calls pass through limiter (unlimited). Tests with `-race`.

#### Issue #M1-2: PriceStreamer slice
```
domain/stream/price_streamer.go        ← PriceStreamer interface
domain/stream/types.go                 ← CachedTicker, SymbolRegistration
infrastructure/stream/price_streamer_impl.go ← polling + WebSocket modes
infrastructure/stream/price_store.go   ← SQLite persistence
usecases/get_market_data.go            ← GetTickerUseCase consumes PriceStreamer
delivery/mcp/server.go                 ← get_ticker tool uses use case
```
**Verification:** Register → fetch → cache → Unregister. WebSocket fallback. E2E MCP.

#### Issue #M1-3: MarketTracker slice
```
domain/stream/market_tracker.go        ← MarketTracker interface
infrastructure/stream/market_tracker_impl.go ← ring buffer + circuit breaker
usecases/monitor_market.go             ← MarketMonitoringUseCase
delivery/mcp/server.go                 ← get_digest shows breaker status
```
**Verification:** Δ% computation, breaker activation/recovery/auto-reset.

#### Issue #M1-4: Idempotency slice
```
domain/stream/idempotency.go           ← IdempotencyStore interface
infrastructure/stream/idempotency_impl.go ← ClientOrderID reservation
infrastructure/db/migrations.go        ← idempotency_keys table
usecases/place_order.go                ← idempotent PlaceOrder flow
```
**Verification:** Reserve → Place → Confirm. Timeout retry with same ClientOrderID.

#### Issue #M1-5: Debouncer + ExchangeHealth slice
```
domain/stream/debouncer.go             ← Debouncer interface
domain/stream/health.go                ← ExchangeHealthMonitor interface
infrastructure/stream/debouncer_impl.go
infrastructure/stream/health_impl.go
usecases/place_order.go                ← checks debouncer + health before placing
```
**Verification:** Cooldown blocks orders. Burst limit enforced. Health pauses all.

#### Issue #M1-6: OrderReconciler slice
```
domain/stream/order_reconciler.go       ← OrderReconciler interface
infrastructure/stream/order_reconciler_impl.go
usecases/reconcile_orders.go           ← ReconcileOrdersUseCase
```
**Verification:** Drift detection. Orphan recovery on startup.

#### Issue #M1-7: Portfolio slice
```
domain/stream/portfolio.go             ← PortfolioValuator interface
infrastructure/stream/portfolio_impl.go
usecases/get_portfolio.go              ← GetPortfolioUseCase
delivery/mcp/server.go                 ← get_portfolio tool
```
**Verification:** Multi-currency conversion. Unpriced assets handled.

#### Issue #M1-8: Bot integration slice
```
usecases/start_bot.go                  ← wires streamer + tracker + debouncer into bot
domain/bot/bot.go                      ← Bot interface updated with stream deps
infrastructure/bot/bot_impl.go         ← tick loop reads cache, checks breaker/health/debouncer
```
**Verification:** E2E: start bot → tick → cache read → breaker check → debounce → idempotent order.

---

## How Each Vertical Slice Works

Every slice follows the same pattern:

1. **Define the interface** in `domain/`. Pure Go. Zero imports beyond stdlib.
2. **Implement it** in `infrastructure/`. This is where SQLite, HTTP, WebSocket live.
3. **Create/update the use case** in `usecases/`. Depends ONLY on domain interfaces.
4. **Wire it** in `delivery/` (MCP or CLI) and `cmd/greedy/main.go` (DI composition).
5. **Test at every layer**: domain interface tests (mocks), infrastructure tests (real), use case tests (mocked deps), delivery tests (integration).

## Dependency Rule (Uncle Bob)

```
delivery ──→ usecases ←── infrastructure
    │            │
    └────────────┼──→ domain ←── infrastructure
                 │        ↑
                 └────────┘
```

- `domain` imports NOTHING from other layers
- `usecases` imports ONLY from `domain`
- `infrastructure` imports from `domain` (implements interfaces)
- `delivery` imports from `usecases` and `domain`
- `cmd/greedy` imports from everything (composition root)

## What This Enables

- **Testability**: Every use case is testable with mocked domain interfaces
- **Replaceability**: Swap PaperExchange for CoinbaseExchange without touching use cases
- **Deployability**: Each slice is a complete feature — mergeable independently
- **AI readability**: An AI reading `domain/stream/rate_limiter.go` instantly understands the contract without reading implementation
