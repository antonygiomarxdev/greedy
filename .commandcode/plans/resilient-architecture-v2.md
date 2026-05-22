# Resilient Trading Architecture — Design Plan v3

> Updated 2026-05-22: added WebSocket-first, order lifecycle, idempotency,
> multi-currency support, exchange health breaker, shutdown safety.

---

## Architecture Diagram

```
 ┌───────────────────────────────────────────────────────────────────────┐
 │                           cmd/greedy (CLI)                            │
 └───────┬───────────────────────────────────────────────────────┬───────┘
         │                                                       │
         ▼                                                       ▼
 ┌───────────────────┐                                 ┌─────────────────┐
 │    Supervisor     │                                 │   MCP Server    │
 │  (owns all infra) │                                 │  (AI interface) │
 │                   │                                 │                 │
 │ ┌─────────────────┐                                │  get_digest     │
 │ │  RateLimiter    │ ◄── token bucket per provider  │  list_bots      │
 │ │  (per exchange) │     shared across all callers   │  start_bot      │
 │ └────────┬────────┘                                │                 │
 │          │                                         └────────┬────────┘
 │ ┌────────┴────────┐                                         │
 │ │ PriceStreamer   │◄── 1 fetch/symbol/interval               │
 │ │ (shared fetch)  │    checks RateLimiter before call         │
 │ │                 │    persists ticks to SQLite               │
 │ └───┬─────────┬───┘                                         │
 │     │         │                                              │
 │ ┌───┴──┐ ┌────┴────────┐                                    │
 │ │Market│ │ PriceStore   │ ◄── SQLite-backed history          │
 │ │Track │ │ (DB persist) │     survives restarts              │
 │ │(mem) │ └─────────────┘                                    │
 │ └──────┘                                                     │
 │                                                              │
 │ ┌───┐ ┌───┐ ┌───┐                                          │
 │ │ B1│ │ B2│ │ B3│◄── reads cache, checks breaker            │
 │ └─┬─┘ └─┬─┘ └─┬─┘    checks debouncer                       │
 │   │      │      │                                            │
 │   └──────┼──────┘                                            │
 │          ▼                                                   │
 │  ┌───────────────┐                                           │
 │  │  Debouncer    │ ◄── per-bot cooldown + burst limit        │
 │  └───────────────┘                                           │
 └──────────┬───────────────────────────────────────────────────┘
            │
            ▼
 ┌──────────────────────┐
 │  Exchange Interface   │
 │ ┌──────────────────┐ │
 │ │ RateLimitProfile │ │ ◄── each exchange exposes its limits
 │ │ (from provider)  │ │
 │ └──────────────────┘ │
 │                      │
 │ GetTicker()          │◄── PriceStreamer (goes through RateLimiter)
 │ GetOrderBook()       │◄── Bots (goes through RateLimiter)
 │ PlaceOrder()         │◄── Bots (goes through RateLimiter)
 │ GetPosition()        │◄── Bots (goes through RateLimiter)
 │ GetBalance()         │◄── Bots (goes through RateLimiter)
 └──────────────────────┘
```

Key change: **RateLimiter wraps ALL exchange calls**, not just GetTicker. Every
outbound HTTP request consumes a token. The RateLimiter is provider-aware and
uses the exchange's `RateLimitProfile` to enforce limits proactively.

---

## Component 1: RateLimitProfile (Provider Config)

### `internal/exchange/profile.go` (NEW)

```go
// RateLimitProfile defines the rate limits and capabilities of an exchange.
// Every Exchange implementation must return its profile via RateLimitProfile().
type RateLimitProfile struct {
    // Provider identifier
    Provider string // "paper", "coinbase", "binance"

    // Token bucket configuration
    RequestsPerSecond float64 // 10 for Coinbase public, 30 for Coinbase private
    BurstSize         int     // max burst before throttling (default = RequestsPerSecond)

    // Endpoint weights (only for weight-based exchanges like Binance)
    // If nil, all endpoints count as 1 request.
    EndpointWeights map[string]int // "GetTicker" → 1, "GetOrderBook" → 10, "PlaceOrder" → 5

    // Minimum intervals enforced by the exchange
    MinFetchInterval time.Duration // fastest allowed ticker fetch (1s for Binance, 100ms for paper)

    // Capabilities
    SupportsWebSocket    bool // if true, SubscribeOrderBook works natively
    SupportsBulkOrders   bool // if true, batch cancel/place works
    SupportsCandles      bool // if true, OHLCV via REST is available

    // Auth
    RequiresAuth        bool // paper=false, coinbase=true
    AuthType            string // "hmac_header" (Coinbase), "hmac_query" (Binance), "none"
}

// ProviderDefaults returns profiles for known exchanges.
func ProviderDefaults() map[string]RateLimitProfile {
    return map[string]RateLimitProfile{
        "paper": {
            Provider:         "paper",
            RequestsPerSecond: 100000, // effectively unlimited
            BurstSize:         100000,
            MinFetchInterval:  100 * time.Millisecond,
            SupportsWebSocket: false,
            SupportsBulkOrders: false,
            SupportsCandles:    false,
            RequiresAuth:       false,
        },
        "coinbase": {
            Provider:          "coinbase",
            RequestsPerSecond: 10, // public endpoints
            BurstSize:         10,
            MinFetchInterval:  1 * time.Second,
            SupportsWebSocket: true,
            SupportsBulkOrders: true,
            SupportsCandles:   true,
            RequiresAuth:       true,
            AuthType:           "hmac_header",
        },
        "coinbase_private": {
            Provider:          "coinbase",
            RequestsPerSecond: 30, // private endpoints
            BurstSize:         30,
            MinFetchInterval:  1 * time.Second,
            SupportsWebSocket: true,
            SupportsBulkOrders: true,
            SupportsCandles:   true,
            RequiresAuth:       true,
            AuthType:           "hmac_header",
        },
        "binance": {
            Provider:          "binance",
            RequestsPerSecond: 20, // 1200 weight/min = 20 weight/s
            BurstSize:         20,
            MinFetchInterval:  1 * time.Second,
            EndpointWeights: map[string]int{
                "GetTicker":    1,
                "GetOrderBook": 10,
                "GetCandles":   2,
                "PlaceOrder":   5,
                "CancelOrder":  2,
                "GetOrder":     1,
                "GetPosition":  5,
                "GetBalance":   5,
            },
            SupportsWebSocket: true,
            SupportsBulkOrders: true,
            SupportsCandles:   true,
            RequiresAuth:       true,
            AuthType:           "hmac_query",
        },
    }
}
```

Each Exchange implementation adds a method: `RateLimitProfile() RateLimitProfile`.
The factory (`internal/exchange/factory.go`) reads the profile and creates a
`RateLimiter` configured for that provider.

---

## Component 2: RateLimiter (Token Bucket, Proactive)

### `internal/stream/rate_limiter.go` (NEW)

```go
// RateLimiter enforces exchange rate limits using a token bucket algorithm.
// It wraps any Exchange and intercepts ALL method calls to count tokens.
//
// Unlike reactive backoff (which only triggers after a 429 error), this
// proactively delays or rejects calls that would exceed the rate limit.
type RateLimiter struct {
    profile  RateLimitProfile      // from the exchange

    // Single token bucket for simple rate-limited exchanges
    limiter  *rateLimiterBucket    // golang.org/x/time/rate or custom

    // For weight-based exchanges (Binance): tracks weight consumption
    // across the sliding 1-minute window.
    weightMu    sync.Mutex
    weightUsed  int                 // weight consumed in current window
    windowStart time.Time           // start of 1-minute sliding window

    logger  *slog.Logger
}

// Wait blocks until a token is available, or returns an error if the context
// is cancelled or the wait would exceed MaxWait.
func (rl *RateLimiter) Wait(ctx context.Context, endpoint string) error

// ConsumeWeight deducts weight from the binance-style sliding window.
// For non-weight-based exchanges, it calls Wait().
func (rl *RateLimiter) ConsumeWeight(ctx context.Context, endpoint string) error
```

If the profile has `EndpointWeights`, `ConsumeWeight` tracks the 1-minute sliding
window. If the weight budget is exhausted, it blocks until the window rolls over.

If the profile is simple (Coinbase), it uses a standard token bucket
(`golang.org/x/time/rate.Limiter` or custom stdlib implementation using
`time.Ticker`).

**Critical design choice**: All exchange calls — GetTicker, GetOrderBook,
PlaceOrder, GetBalance, etc. — go through `RateLimiter.Wait()` or
`RateLimiter.ConsumeWeight()` before the actual HTTP call. This prevents the
PriceStreamer + bots combined from exceeding the rate limit.

The RateLimiter lives in the Supervisor (or is attached to the Exchange
instance) and is shared across ALL callers.

---

## Component 3: PriceStreamer (Updated — knows RateLimiter)

### `internal/stream/price_streamer.go`

Same as v1, but:
- Each fetch goroutine calls `rateLimiter.Wait(ctx, "GetTicker")` before the exchange call
- The `Register` method validates that the requested interval is ≥ `profile.MinFetchInterval`
- The `defaultInterval` is auto-set to `profile.MinFetchInterval` if not explicitly configured

---

## Component 4: PriceStore (SQLite Persistence)

### `internal/stream/price_store.go` (NEW)

```go
// PriceStore persists ticker price history to SQLite for:
// 1. Surviving process restarts (warm start for MarketTracker)
// 2. Backtesting data source (enrich with live market data)
// 3. Audit trail (when did we see what price?)
type PriceStore struct {
    db     *sql.DB
    logger *slog.Logger
}

// InsertTicker writes a single ticker snapshot to SQLite.
func (ps *PriceStore) InsertTicker(ctx context.Context, symbol string, price, bid, ask float64, ts time.Time) error

// QueryWindow returns prices within a time range, used by MarketTracker
// to restore its sliding window on startup.
func (ps *PriceStore) QueryWindow(ctx context.Context, symbol string, from, to time.Time) ([]PricePoint, error)

// Prune removes ticks older than the retention period (default 7 days).
func (ps *PriceStore) Prune(ctx context.Context, olderThan time.Duration) (int64, error)

// InsertCircuitBreakerEvent logs breaker activation/deactivation for audit.
func (ps *PriceStore) InsertCircuitBreakerEvent(ctx context.Context, symbol string, active bool, delta float64, ts time.Time) error

// QueryCircuitBreakerEvents returns recent breaker events for diagnostics.
func (ps *PriceStore) QueryCircuitBreakerEvents(ctx context.Context, symbol string, limit int) ([]BreakerEvent, error)
```

### SQLite Schema (new migration)

```sql
CREATE TABLE IF NOT EXISTS price_ticks (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol    TEXT    NOT NULL,
    price     REAL    NOT NULL,
    bid       REAL,
    ask       REAL,
    timestamp INTEGER NOT NULL,  -- unix milliseconds
    created_at INTEGER NOT NULL  -- insertion time
);

CREATE INDEX idx_price_ticks_symbol_ts ON price_ticks(symbol, timestamp);

CREATE TABLE IF NOT EXISTS breaker_events (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol    TEXT    NOT NULL,
    active    INTEGER NOT NULL,  -- 0=deactivated, 1=activated
    delta_pct REAL    NOT NULL,
    timestamp INTEGER NOT NULL
);

CREATE INDEX idx_breaker_events_symbol_ts ON breaker_events(symbol, timestamp);
```

The PriceStreamer inserts every fetched ticker into `PriceStore`. The
MarketTracker on startup queries `PriceStore.QueryWindow` to restore its ring
buffers from the last `MaxWindow` duration. Circuit breaker activation/deactivation
is also logged for audit.

---

## Component 5: MarketTracker (Updated — SQLite-backed warm start)

### `internal/stream/market_tracker.go`

Same as v1, plus:

- `NewMarketTracker(db *sql.DB, config MarketTrackerConfig)` — accepts a DB handle
- `Restore(ctx context.Context, symbols []string)` — queries PriceStore for each
  symbol's recent ticks and pre-fills the ring buffers. Called once on startup.
- On every `Record(symbol, price)`, the PriceStreamer also calls
  `PriceStore.InsertTicker()` (persisted outside the MarketTracker's lock).

---

## Component 6: Supervisor (Updated — wires everything)

### `internal/bot/supervisor.go`

On startup:
1. Resolve exchange (paper or real)
2. Read `RateLimitProfile` from exchange
3. Create `RateLimiter(profile)`
4. Create `PriceStore(db)`
5. Create `MarketTracker(db, config)` → call `Restore()`
6. Create `PriceStreamer(exchange, rateLimiter, priceStore, tracker)`

On `StartBot`:
1. `PriceStreamer.Register(symbol)`
2. Create `Debouncer` per bot
3. Bot receives: streamer, rateLimiter, debouncer

On `StopBot`:
1. `PriceStreamer.Unregister(symbol)`

All exchange calls in the supervisor and bots go through `rateLimiter`.

---

## Full Data Flow (Per Tick, Real Exchange)

```
BACKGROUND FETCH (PriceStreamer goroutine, per symbol, every 10s):
1. rateLimiter.Wait(ctx, "GetTicker")         ← blocks if bucket exhausted
2. ticker, err := exchange.GetTicker(ctx, symbol)
3. IF err: set CachedTicker.Stale, keep old price
4. IF ok:  replace CachedTicker
5. priceStore.InsertTicker(symbol, price, bid, ask, ts)  ← ASYNC or sync?
6. marketTracker.Record(symbol, price, ts)
7. IF breaker activates/deactivates:
   priceStore.InsertCircuitBreakerEvent(symbol, active, delta, ts)

BOT TICK (100ms):
1. price := streamer.GetCached(symbol)         ← no lock contention, RLock
2. breaker := streamer.IsBreakerActive(symbol)  ← RLock
3. IF BUY and breaker: check if safety order → allow, else SKIP
4. IF NOT debouncer.CanExecute(): SKIP
5. rateLimiter.Wait(ctx, "PlaceOrder")         ← blocks if no tokens
6. order := exchange.PlaceOrder(...)
7. debouncer.RecordExecution()
8. IF rateLimiter gets 429 from exchange:
   - Even though we're proactive, exchanges can still 429
   - RateLimiter observes the 429 via response interceptor
   - Cuts token bucket in half (adaptive backoff)
   - Logs WARNING

STARTUP (after restart):
1. supervisor.NewSupervisor()
2. marketTracker.Restore(ctx, activeSymbols)
   - Queries priceStore for last 120s of ticks
   - Pre-fills ring buffers
   - Restores breaker state from breaker_events table
3. Bots resume with warm state (no cold-start blindness)
```

---

## Provider Config Strategy

Three layers of configuration, from most specific to most general:

### Layer 1: Exchange-provided profile (hardcoded, authoritative)
Each package `internal/exchange/coinbase/` has a `CoinbaseProfile()` that returns
the known rate limits from Coinbase's docs. This is the source of truth.

### Layer 2: User overrides in `greedy.yaml` (optional)
```yaml
rate_limits:
  coinbase:
    requests_per_second: 8      # conservative (official is 10)
    min_fetch_interval: "2s"    # user preference
  binance:
    requests_per_second: 15     # 900 weight/min instead of 1200
```

### Layer 3: Adaptive (runtime)
The RateLimiter observes actual 429 responses and 403 (banned) responses from the
exchange. If it gets rate-limited despite being under the profile limit, it
halves the token bucket and logs a WARNING suggesting the user lower their config.

---

## Data Persistence Summary

| What | Where | Retention | Restore on Startup |
|------|-------|-----------|-------------------|
| Price ticks | SQLite `price_ticks` | 7 days, pruned hourly | MarketTracker ring buffer |
| Breaker events | SQLite `breaker_events` | 30 days | Breaker state |
| Bot state (positions, orders) | Already in `internal/db/` | Permanent | Already implemented |
| Debouncer state | In-memory only | N/A | Reset on restart (safe default) |
| Rate limiter state | In-memory only | N/A | Reset on restart (safe default) |

Bot state (positions, orders, balances from paper exchange) already goes through
`internal/db/` with the existing SQLite schema. PriceStore adds the missing
piece: price history.

---

## New Files

| File | Purpose |
|------|---------|
| `internal/exchange/profile.go` | RateLimitProfile + ProviderDefaults |
| `internal/stream/rate_limiter.go` | Token bucket + weight-based rate limiter |
| `internal/stream/rate_limiter_test.go` | Concurrency + exhaustion tests |
| `internal/stream/price_store.go` | SQLite price persistence |
| `internal/stream/price_store_test.go` | Insert/Query/Prune tests |
| `internal/stream/types.go` | CachedTicker, PricePoint, SymbolRegistration, BreakerEvent, PortfolioValuation, all configs |
| `internal/stream/price_streamer.go` | WebSocket-first shared fetcher (Mode A polling + Mode B WebSocket) |
| `internal/stream/price_streamer_test.go` | Registration, cache, WS reconnect, stale data, race tests |
| `internal/stream/market_tracker.go` | Sliding window + circuit breaker + DB restore + zero-variance detection |
| `internal/stream/market_tracker_test.go` | Window math, breaker logic, restore, zero-variance tests |
| `internal/stream/debouncer.go` | Per-bot cooldown + burst limit |
| `internal/stream/debouncer_test.go` | Cooldown, burst limit tests |
| `internal/stream/digest.go` | AI-optimized context builder |
| `internal/stream/digest_test.go` | Digest size, key accuracy tests |
| `internal/db/idempotency.go` | IdempotencyStore — ClientOrderID reservation + confirmation |
| `internal/db/idempotency_test.go` | Reserve, confirm, duplicate key tests |
| `internal/stream/order_reconciler.go` | Periodic order state reconciliation loop |
| `internal/stream/order_reconciler_test.go` | Drift detection, orphan recovery tests |
| `internal/stream/exchange_health.go` | Exchange health circuit breaker |
| `internal/stream/exchange_health_test.go` | Fail/recovery thresholds, pause/resume tests |
| `internal/stream/portfolio.go` | Multi-currency portfolio valuation |
| `internal/stream/portfolio_test.go` | Base currency conversion, cross-pair, unpriced assets tests |

## Modified Files

| File | Changes |
|------|---------|
| `internal/exchange/exchange.go` | Add `RateLimitProfile() RateLimitProfile` + `SubscribeTickers()` to interface |
| `internal/exchange/types.go` | Add `StatusPending`, `StatusExpired` |
| `internal/exchange/paper/paper.go` | Implement `RateLimitProfile()` + `SubscribeTickers()`; Configurable initial balances |
| `internal/db/migrations.go` | Add `price_ticks`, `breaker_events`, `idempotency_keys` tables |
| `internal/bot/supervisor.go` | Create all stream components on init; Register/Unregister symbols on StartBot/StopBot; Pass streamer + limiter + debouncer to bots; Shutdown cancel-all flow |
| `internal/bot/bot.go` | Tick loop reads cache; Checks market breaker + exchange health + debouncer; Uses idempotent ClientOrderID for all orders |
| `internal/mcp/server.go` | `get_digest`, `get_portfolio` tools; Augment `list_bots` with breaker + health status |
| `cmd/greedy/main.go` | Wire stream components; Signal handler → Supervisor.Shutdown(); Parse rate limit overrides from greedy.yaml |
| `cmd/greedy/run.go` | Pass streamer + limiter to supervisor |

## Edge Cases

| Scenario | Handling |
|----------|----------|
| Rate limiter blocks all callers | Context deadline → error returned to bot → bot retries next tick |
| Rate limiter + 5 bots + streamer all wait | FIFO token bucket — fair across all callers |
| Binance weight budget exhausted at 59s | `ConsumeWeight` blocks until windowStart + 60s rolls over |
| Exchange returns 429 despite proactive limiting | RateLimiter intercepts, halves bucket, logs WARNING |
| Exchange returns 403 (banned) | RateLimiter sets banned=true, all calls return ErrBanned immediately |
| Process restarts with empty DB | MarketTracker.Restore returns empty — cold start, fine |
| Process restarts with 1M price ticks in DB | Restore only queries last MaxWindow (120s) — bounded query |
| PriceStore DB locked during Prune | Prune runs in a separate goroutine, uses WAL mode |
| 50 symbols active on Binance | 50 × 2 weight/min (GetTicker=1 + occasional GetOrderBook=10) = ~100 weight/min. Binance limit = 1200. Well within budget. |
| Breaker activates, DB insert fails | Log WARNING, breaker still active in memory — degraded but safe |
| RateLimiter goroutine leaks | Supervisor owns it, cancels on shutdown via context |

---

## Component 7: WebSocket-First Price Feed

### Design

The `PriceStreamer` supports two modes, selected by the exchange's
`RateLimitProfile.SupportsWebSocket` flag:

**Mode A — Polling** (paper exchange, exchanges without WebSocket):
- Same as v2: one fetch goroutine per symbol, calls `GetTicker` at interval
- Goes through `RateLimiter.Wait()` before each call

**Mode B — WebSocket** (Coinbase, Binance):
- A **single WebSocket connection per exchange**, not per symbol
- On `Register(symbol)`: sends a subscription message for that symbol's ticker channel
- On `Unregister(symbol)`: sends unsubscribe when RefCount → 0
- Incoming ticks update `CachedTicker` directly — no polling, zero rate limit cost
- The fetch goroutine per symbol is replaced by the shared WebSocket read loop
- `RateLimiter` is bypassed for price updates (they're pushed, not requested)

WebSocket connection management:
- Auto-reconnect with exponential backoff (1s → 2s → 4s → ... max 30s)
- On reconnect, re-subscribe all active symbols
- During disconnect, `CachedTicker.Stale = true` with age tracking
- Health: if disconnected > 60s, set `streamer.healthy = false`

### WebSocket Message Flow (Coinbase example)

```
CONNECT wss://advanced-trade-ws.coinbase.com
  ↓
SUBSCRIBE {"type":"subscribe","channel":"ticker","product_ids":["BTC-USD","ETH-USD"]}
  ↓
RECEIVE {"type":"ticker","product_id":"BTC-USD","price":"50100",...}
  → update CachedTicker["BTC-USD"]
  → MarketTracker.Record("BTC-USD", 50100)
  → PriceStore.InsertTicker(...)
```

The `Exchange` interface gets a new optional method:

```go
// SubscribeTickers returns a channel of ticker updates for the given symbols.
// Only implemented by exchanges that support WebSocket (SupportsWebSocket=true).
// Returns ErrNotSupported for polling-only exchanges.
SubscribeTickers(ctx context.Context, symbols []string) (<-chan *Ticker, error)
```

`PriceStreamer` calls this once with all registered symbols and fans out updates
to the per-symbol caches.

---

## Component 8: Order Lifecycle & Reconciliation

### Problem

The paper exchange resolves orders instantly to `filled`. Real exchanges have:

```
pending → open → partially_filled → filled
       → cancelled → cancelled
       → expired   → expired
       → rejected  → rejected
```

Bots need to handle `partially_filled` (order placed for 1 BTC, only 0.3 filled).
And the local state can drift from the exchange state (network blip, crash).

### OrderReconciler (internal/stream/order_reconciler.go)

```go
type OrderReconciler struct {
    exchange exchange.Exchange
    store    *IdempotencyStore   // local DB
    interval time.Duration       // 30s default
    logger   *slog.Logger
}

// ReconcileLoop runs in a goroutine. Every interval:
// 1. Queries all orders in "open" or "partially_filled" status from local DB
// 2. Calls exchange.GetOrder() for each
// 3. If exchange status differs from local, updates local DB
// 4. If order vanished from exchange (filled/cancelled while we were down), marks accordingly
// 5. Emits events for state transitions (so bots can react)
func (or *OrderReconciler) ReconcileLoop(ctx context.Context)
```

The reconciler also handles the **startup case**: on restart, queries all orders
that were `open` or `partially_filled` at shutdown, reconciles them against the
exchange, and emits events so bots can resume correctly.

### Order State Machine (already in types.go)

Types already define `StatusOpen`, `StatusPartiallyFilled`, `StatusFilled`,
`StatusCancelled`, `StatusRejected`. Need to add `StatusExpired` and
`StatusPending`:

```go
StatusPending  OrderStatus = "pending"   // submitted to exchange, not yet ack'd
StatusExpired  OrderStatus = "expired"   // time-in-force exceeded
```

---

## Component 9: Idempotency Layer

### Problem

If `PlaceOrder()` times out (network blip), the bot doesn't know if the order
was created on the exchange. Retrying with a new order ≠ idempotent → duplicate.

### IdempotencyStore (internal/db/idempotency.go)

```go
type IdempotencyStore struct {
    db *sql.DB
}

// ReserveKey generates and stores a ClientOrderID BEFORE placing the order.
// Returns ErrDuplicateKey if this ID was already used (defense in depth).
func (is *IdempotencyStore) ReserveKey(ctx context.Context, clientOrderID string) error

// ConfirmOrder updates the stored key with the exchange-assigned OrderID.
func (is *IdempotencyStore) ConfirmOrder(ctx context.Context, clientOrderID, exchangeOrderID string, status exchange.OrderStatus) error

// LookupOrder returns the stored exchange OrderID for a ClientOrderID.
// Returns ErrNotFound if the key was never used.
func (is *IdempotencyStore) LookupOrder(ctx context.Context, clientOrderID string) (*exchange.Order, error)
```

### ClientOrderID Generation

Format: `{botID}-{unix_ms}-{seq}`

Example: `dca-btc-01-1716400000000-0005`

Generated per bot, monotonically increasing seq within each millisecond. The
`botID` prefix ensures no collisions across bots. The `unix_ms` + `seq` suffix
ensures uniqueness within a bot.

### PlaceOrder Flow (with idempotency)

```
1. clientOrderID := fmt.Sprintf("%s-%d-%04d", botID, now.UnixMilli(), seq)
2. seq++
3. store.ReserveKey(clientOrderID)           ← write-ahead: we INTEND to place
4. order, err := exchange.PlaceOrder(ctx, OrderRequest{
       ClientOrderID: clientOrderID,          ← exchange also deduplicates
       ...})
5. IF err == nil:
       store.ConfirmOrder(clientOrderID, order.ID, "open")
       return order
6. IF err is timeout/network:
       // Don't know if it succeeded. Retry with SAME ClientOrderID.
       // Exchange MUST deduplicate based on ClientOrderID.
       order, err := exchange.PlaceOrder(ctx, sameRequest)
       IF err == nil: return order
       // Still failing? Reconciliation will sort it out on next cycle.
7. IF err is explicit rejection (400):
       store.MarkFailed(clientOrderID)
       return err
```

### SQLite Schema

```sql
CREATE TABLE IF NOT EXISTS idempotency_keys (
    client_order_id   TEXT PRIMARY KEY,
    exchange_order_id TEXT,
    bot_id            TEXT NOT NULL,
    symbol            TEXT NOT NULL,
    status            TEXT NOT NULL,  -- reserved, confirmed, failed
    created_at        INTEGER NOT NULL,
    confirmed_at      INTEGER
);

CREATE INDEX idx_idempotency_bot ON idempotency_keys(bot_id, created_at);
```

---

## Component 10: Multi-Currency Portfolio Valuation

### Problem

The paper exchange seeds with `{"USD": 100000}`. Strategies and reporting assume
USD is the quote currency for all pairs. But:

- `BTC-ETH` has ETH as quote → P&L is in ETH, not USD
- A portfolio with BTC, ETH, SOL needs a base currency for total equity
- The `TradingDigest.Eq` field is meaningless without converting all assets

### Current State (what already works)

The `Balance` type has `Asset string` — it can represent any currency. The paper
exchange already tracks balances as `map[string]float64`. The strategies use
`symbol` from config (e.g., `BTC-ETH`), so the quote currency is whatever the
pair specifies. Nothing hardcodes `USD` except the paper exchange seed.

### What Needs to Change

**1. Paper exchange seed becomes configurable:**

```go
func New(feeRate float64, initialBalance map[string]float64) *PaperExchange
```

Default: `{"USD": 100000, "BTC": 1.0}` so both USD and crypto pairs work.

**2. Portfolio valuation via PriceStreamer:**

```go
// PortfolioValuation values all held assets in a base currency using
// the latest cached prices from the PriceStreamer.
type PortfolioValuation struct {
    BaseCurrency string             // "USD"
    TotalEquity  float64            // sum of all assets converted to base
    Assets       []AssetValue       // one per held asset
}

type AssetValue struct {
    Asset     string   // "BTC", "ETH", "USD"
    Balance   float64  // how much we hold
    Price     float64  // in base currency (1.0 for base itself)
    Value     float64  // balance * price
}

func (ps *PriceStreamer) ValuatePortfolio(
    ctx context.Context,
    baseCurrency string,
    balances map[string]float64,
) (*PortfolioValuation, error)
```

How it prices each asset:
- `USD` → price = 1.0
- `BTC` → fetch `BTC-USD` from cache (or `USDT-BTC` inverse)
- `ETH` → fetch `ETH-USD` from cache
- `X` → if `X-USD` exists in cache, use that. If not, try `USD-X` and invert.
  If neither, mark as `unpriced` in the output.

**3. TradingDigest updated:**

```go
type TradingDigest struct {
    Ts   int64              `json:"ts"`
    Base string             `json:"base"`  // "USD"
    Eq   float64            `json:"eq"`    // total equity in base currency
    Bal  map[string]float64 `json:"bal"`
    Mkt  map[string]MktSnap `json:"mkt"`
    Bot  []BotSnap          `json:"bot"`
    Ord  []OrdSnap          `json:"ord"`
    Alrt []string           `json:"alrt"`
}
```

The `Eq` field is now computed by `PortfolioValuation`, not a simple sum of USD
balance.

**4. Multi-currency caveat — cross pairs:**

For `BTC-ETH`, the strategy works correctly (quote = ETH). But to display BTC-ETH
in the digest, we need an ETH-USD price. The PriceStreamer needs `ETH-USD`
registered (or `USDT-ETH`) even if no bot is trading it — it's needed for
portfolio conversion. The Supervisor should auto-register the quote-side pair for
any non-USD quote currency.

### New MCP Tool: `get_portfolio`

```json
{
  "name": "get_portfolio",
  "description": "Total portfolio valuation in base currency (default USD) with per-asset breakdown"
}
```

Returns the `PortfolioValuation` as JSON. The AI can ask "what's my total portfolio worth?"
and get the answer in one call.

---

## Component 11: Exchange Health Circuit Breaker

### Problem

If Coinbase returns 5xx for 30 seconds, every bot call fails. The bots keep
retrying, burning through their tick budgets and logging errors. The system 
should detect "exchange is down" and pause all bots using that exchange.

### ExchangeHealthMonitor (internal/stream/exchange_health.go)

```go
type ExchangeHealthMonitor struct {
    mu              sync.RWMutex
    healthy         bool                         // current state
    consecutiveFail int                          // errors in a row
    threshold       int                          // 5 consecutive failures → unhealthy
    recoveryCount   int                          // successes needed to recover (3)
    lastError       time.Time
    lastErrorType   string
    logger          *slog.Logger
}

// RecordResult tracks success/failure of any exchange call.
func (ehm *ExchangeHealthMonitor) RecordResult(err error)

// IsHealthy returns false if the exchange is considered down.
func (ehm *ExchangeHealthMonitor) IsHealthy() bool

// WaitForRecovery blocks until the exchange is healthy or ctx is cancelled.
func (ehm *ExchangeHealthMonitor) WaitForRecovery(ctx context.Context) error
```

Integrated into `RateLimiter` — every exchange call result passes through
`RecordResult`. If 5 consecutive calls fail, `IsHealthy()` returns false.
Bots check this and pause. The `PriceStreamer` checks and stops fetching.

Recovery: after 3 consecutive successful calls (or a 30-second cooldown),
the monitor flips back to healthy. Bots resume.

The MCP digest and `list_bots` show `"exchange_health": "degraded"` when
unhealthy.

---

## Component 12: Shutdown Safety — Cancel All Open Orders

### Problem

When Greedy shuts down (Ctrl+C, OOM, crash), open orders on a real exchange
remain open. On restart, the user has stale orders they didn't intend to keep.

### Implementation

In the Supervisor (or main.go's signal handler):

```go
func (s *Supervisor) Shutdown(ctx context.Context) error {
    // 1. Stop all bots (they stop placing new orders)
    s.StopAllBots()
    
    // 2. Cancel all open orders on the exchange
    openOrders, err := s.exchange.ListOpenOrders(ctx, "")  // all symbols
    if err != nil {
        s.logger.Warn("could not list open orders during shutdown", "error", err)
    }
    for _, order := range openOrders {
        if err := s.exchange.CancelOrder(ctx, order.ID); err != nil {
            s.logger.Warn("failed to cancel order during shutdown", "order_id", order.ID, "error", err)
        }
    }
    
    // 3. Stop PriceStreamer, MarketTracker, RateLimiter
    s.streamer.Shutdown()
    s.rateLimiter.Shutdown()
    
    return nil
}
```

The shutdown flow respects a deadline (default 30s). If canceling orders takes
too long, it logs which orders remain and exits — the reconciler will handle
them on next startup.

On **startup**, the `OrderReconciler` checks for any orders that were open at
last shutdown. The user gets a WARNING in logs and the digest shows an alert:
`"ORPHAN:3"` (3 orders still open from previous session).

---

## Updated Architecture Diagram

```
 ┌───────────────────────────────────────────────────────────────────────┐
 │                           cmd/greedy (CLI)                            │
 │                    signal handler → Supervisor.Shutdown()             │
 └───────┬───────────────────────────────────────────────────────┬───────┘
         │                                                       │
         ▼                                                       ▼
 ┌───────────────────┐                                 ┌─────────────────┐
 │    Supervisor     │                                 │   MCP Server    │
 │                   │                                 │                 │
 │ ┌─────────────────┐                                │  get_digest     │
 │ │ RateLimiter     │ ◄── token bucket per provider  │  get_portfolio  │
 │ │ ExchangeHealth  │     + health monitor           │  list_bots      │
 │ └────────┬────────┘                                │  start_bot      │
 │          │                                         └────────┬────────┘
 │ ┌────────┴────────┐                                         │
 │ │ PriceStreamer   │◄── WebSocket-first                     │
 │ │ (polling/wss)   │    or polling fallback                  │
 │ │                 │    → PriceStore (SQLite)                │
 │ │                 │    → MarketTracker (mem)                │
 │ └───┬─────────┬───┘                                         │
 │     │         │                                              │
 │ ┌───┴──┐ ┌────┴──────────┐                                  │
 │ │Market│ │ PriceStore    │                                  │
 │ │Track │ │ IdempotencySt │                                  │
 │ └──────┘ │ OrderReconcil │                                  │
 │          └──────────────┘                                   │
 │                                                              │
 │ ┌───┐ ┌───┐ ┌───┐                                          │
 │ │ B1│ │ B2│ │ B3│◄── reads cache, checks breaker            │
 │ └─┬─┘ └─┬─┘ └─┬─┘    checks debouncer                       │
 │   │      │      │     checks exchange health                │
 │   │      │      │     uses ClientOrderID (idempotent)       │
 │   └──────┼──────┘                                            │
 │          ▼                                                   │
 │  ┌───────────────┐                                           │
 │  │  Debouncer    │ ◄── per-bot cooldown + burst              │
 │  └───────────────┘                                           │
 └──────────┬───────────────────────────────────────────────────┘
            │
            ▼
 ┌──────────────────────────┐
 │   Exchange Interface      │
 │ ┌──────────────────────┐ │
 │ │ RateLimitProfile     │ │
 │ │ SubscribeToTickers() │ │ ← optional WebSocket
 │ │ PlaceOrder()         │ │ ← uses ClientOrderID
 │ │ CancelOrder()        │ │
 │ │ ListOpenOrders()     │ │ ← used during shutdown
 │ └──────────────────────┘ │
 └──────────────────────────┘
```

---

## Edge Cases (Expanded)

| Scenario | Handling |
|----------|----------|
| **WebSocket disconnects** | Exponential backoff reconnect (1s→2s→4s...30s). CachedTicker.Stale=true. Re-subscribe all symbols on reconnect. >60s disconnected → streamer.healthy=false |
| **WebSocket sends duplicate tick** | Compare timestamp, ignore if older than last seen |
| **PlaceOrder times out** | Retry with same ClientOrderID. Exchange deduplicates. Reconciler verifies later |
| **Idempotency key collision** | Impossible with `{botID}-{unix_ms}-{seq}` format, but ReserveKey returns error if DB constraint violated |
| **BTC-ETH position, no ETH-USD price** | Supervisor auto-registers ETH-USD in PriceStreamer. If still unavailable, mark as "unpriced" in digest |
| **Exchange health flips unhealthy** | All bots using that exchange pause. PriceStreamer stops. MCP digest shows alert. AI can decide to wait or intervene |
| **Shutdown with 100 open orders** | CancelAll in parallel (goroutine per order). Deadline: 30s. Log remaining orders. Reconciler catches them on restart |
| **Crash kills process mid-cancel** | On restart: Reconciler finds orphan orders → WARNING + digest alert → user/AI decides: cancel or keep? |
| **DEX / Solana** | NOT covered by this architecture. Separate DEX interface + wallet manager needed. Milestone 4 placeholder.

## Vertical Slice Issue

**Title:** Resilient Trading Layer — WebSocket-First Shared Fetcher, Sliding Window State, Idempotency, Multi-Currency, and AI Digest

**Description:**

Implement the `internal/stream` package with 8 components and integrate
them into the bot supervisor and MCP server. This makes Greedy:

- **Rate-limit safe**: Proactive token bucket, WebSocket-first to avoid polling
- **Crash resilient**: Circuit breaker on price velocity + exchange health
- **Restart-safe**: SQLite persistence for price history, breaker events, idempotency
- **Idempotent**: ClientOrderID-based deduplication, retry-safe orders
- **Multi-currency**: Portfolio valuation in base currency for any asset
- **AI-optimized**: Compact digest with single-char JSON keys, portfolio view
- **Shutdown-safe**: Cancels all open orders on exit, reconciles orphans on restart

**Acceptance Criteria:**

1. **RateLimitProfile**: Provider defaults for paper, coinbase, binance. User overrides in `greedy.yaml`. Adaptive halving on 429.
2. **RateLimiter**: Token bucket + weight-based sliding window. Wraps ALL exchange calls.
3. **PriceStreamer**: WebSocket-first (Mode B) with polling fallback (Mode A). Reference-counted registration. Auto-reconnect. Stale detection.
4. **MarketTracker**: Ring buffer per symbol. Δ% over configurable horizons. Circuit breaker. Warm-start from PriceStore on restart. Zero-variance detection.
5. **PriceStore**: SQLite `price_ticks` + `breaker_events` tables. Insert, QueryWindow, Prune.
6. **IdempotencyStore**: SQLite `idempotency_keys` table. Write-ahead reservation. Retry-safe PlaceOrder flow with `{botID}-{unix_ms}-{seq}` ClientOrderID format.
7. **OrderReconciler**: Periodic reconciliation loop (30s). Detects order state drift. Handles orphan orders on startup.
8. **Order state machine**: Add `StatusPending` and `StatusExpired` to types.
9. **Debouncer**: Per-bot cooldown (5s) + burst limit (10 orders/30s).
10. **ExchangeHealthMonitor**: Pauses all bots when exchange is unhealthy (5 consecutive failures). Auto-recovers after 3 successes or 30s.
11. **PortfolioValuation**: Prices all assets in base currency via PriceStreamer cache. Auto-registers quote-side pairs for cross-rate conversion.
12. **Bot integration**: Tick loop reads cache. Checks market breaker, exchange health, debouncer. Uses idempotent ClientOrderID for all orders.
13. **Shutdown safety**: Cancel all open orders on exit (30s deadline). Log remaining. Reconciler catches orphans on restart.
14. **MCP tools**: `get_digest` (compact JSON, single-char keys, ≤15 lines). `get_portfolio` (base-currency valuation with per-asset breakdown). `list_bots` augmented with breaker + exchange health status.
15. **Tests with `-race`**: All stream components tested concurrently.
16. **Tests for edge cases**: WebSocket disconnect/reconnect. PlaceOrder timeout + retry. Breaker activation → recovery → auto-reset. RefCount zero → cleanup. DB restore on startup. Cross-pair pricing.
17. **No breaking changes** to existing strategy implementations or backtest engine.

**New Files:**

| File | Purpose |
|------|---------|
| `internal/exchange/profile.go` | RateLimitProfile + ProviderDefaults |
| `internal/stream/rate_limiter.go` | Token bucket + weight-based rate limiter |
| `internal/stream/price_store.go` | SQLite price persistence |
| `internal/stream/idempotency.go` | ClientOrderID store + PlaceOrder flow |
| `internal/stream/order_reconciler.go` | Periodic order state reconciliation |
| `internal/stream/exchange_health.go` | Exchange health circuit breaker |
| `internal/stream/portfolio.go` | Multi-currency portfolio valuation |
| `internal/stream/price_streamer.go` | WebSocket-first shared fetcher |
| `internal/stream/market_tracker.go` | Sliding window + circuit breaker + DB restore |
| `internal/stream/debouncer.go` | Per-bot cooldown |
| `internal/stream/digest.go` | AI-optimized context builder |
| `internal/stream/types.go` | All shared types |
| Plus `_test.go` for each | |

**Modified Files:**

| File | Changes |
|------|---------|
| `internal/exchange/exchange.go` | Add `RateLimitProfile()` + `SubscribeTickers()` to interface |
| `internal/exchange/types.go` | Add `StatusPending`, `StatusExpired` |
| `internal/exchange/paper/paper.go` | Implement `RateLimitProfile()`, configurable initial balances |
| `internal/db/migrations.go` | Add `price_ticks`, `breaker_events`, `idempotency_keys` tables |
| `internal/bot/supervisor.go` | Create all stream components on init; Register/Unregister symbols; Shutdown cancel-all flow |
| `internal/bot/bot.go` | Tick loop reads cache; Checks market breaker + exchange health + debouncer; Uses idempotent ClientOrderID |
| `internal/mcp/server.go` | `get_digest`, `get_portfolio` tools; Augment `list_bots` |
| `cmd/greedy/main.go` | Wire stream components; Signal handler → Supervisor.Shutdown(); Parse rate limit overrides |

**Estimated complexity:** High. ~2000 lines of new code, ~1200 lines of tests.

**DEX / Solana**: Explicitly out of scope for this issue. Requires separate
`DEXExchange` interface with wallet management, transaction signing, AMM routing,
and MEV protection. Planned as Milestone 4.
