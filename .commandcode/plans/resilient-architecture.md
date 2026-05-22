# Resilient Trading Architecture — Design Plan

## Architecture Diagram

```
 ┌─────────────────────────────────────────────────────────────────┐
 │                      cmd/greedy (CLI)                           │
 └───────┬─────────────────────────────────────────────────┬───────┘
         │                                                 │
         ▼                                                 ▼
 ┌───────────────────┐                           ┌─────────────────┐
 │    Supervisor     │                           │   MCP Server    │
 │  (owns streamer)  │                           │  (AI interface) │
 │                   │                           │                 │
 │ ┌───────────────┐ │                           │  get_digest     │
 │ │ PriceStreamer │◀├── 1 fetch/symbol/interval  │  list_bots      │
 │ │ (shared fetch)│ │     all bots read cache    │  start_bot      │
 │ └───────┬───────┘ │                           │                 │
 │         │         │                           └────────┬────────┘
 │ ┌───────┴───────┐ │                                    │
 │ │MarketTracker  │ │                           reads compact digest
 │ │per-symbol     │ │                                    │
 │ │sliding window │ │                                    │
 │ │circuit breaker│ │
 │ └───────────────┘ │
 │                   │
 │ ┌───┐ ┌───┐ ┌───┐ │
 │ │ B1│ │ B2│ │ B3│ │◄── reads cached price from streamer
 │ └─┬─┘ └─┬─┘ └─┬─┘ │    checks breaker before BUY
 │   │      │      │   │    checks debouncer before any order
 │   └──────┼──────┘   │
 │          ▼           │
 │  ┌───────────────┐   │
 │  │  Debouncer    │   │ ◄── per-bot cooldown + burst limit
 │  └───────────────┘   │
 └──────────┬───────────┘
            │
            ▼
 ┌──────────────────────┐
 │   Exchange Interface  │
 │ (paper | coinbase |  │
 │  binance | ...)      │
 │                      │
 │ GetTicker()          │◄── ONLY PriceStreamer calls this
 │ GetPosition()        │◄── Bots call directly (freshness matters)
 │ GetBalance()         │◄── Bots call directly
 │ PlaceOrder()         │◄── Bots call after debounce+breaker check
 └──────────────────────┘
```

## Core Structs

### `PriceStreamer` — Shared Fetcher (internal/stream/price_streamer.go)

```go
type PriceStreamer struct {
    mu            sync.RWMutex
    exchange      exchange.Exchange          // underlying exchange
    
    registrations map[string]*SymbolRegistration  // symbol → refcount + config
    tickers       map[string]*CachedTicker        // symbol → latest cached price
    
    tracker       *MarketTracker             // sliding window + circuit breaker
    
    ctx           context.Context
    cancel        context.CancelFunc
    wg            sync.WaitGroup
    
    fetchers      map[string]context.CancelFunc // per-symbol goroutine cancel
    
    defaultInterval time.Duration  // 10s default
    maxStaleAge     time.Duration  // 60s — after this, return ErrStaleData
    healthy         atomic.Bool    // supervisor health check
    
    logger          *slog.Logger
}
```

Thread safety: `registrations` and `tickers` share one `sync.RWMutex`. `healthy` is atomic — no lock needed for health checks.

### `SymbolRegistration` — Reference Counter (internal/stream/types.go)

```go
type SymbolRegistration struct {
    Symbol              string
    RefCount            int32          // atomic — bots register/unregister
    Interval            time.Duration  // fetch interval
    LastFetch           time.Time
    LastSuccess         time.Time
    ConsecutiveFailures int            // for exponential backoff
}
```

### `CachedTicker` — The Value Bots Read (internal/stream/types.go)

```go
type CachedTicker struct {
    Symbol    string
    Price     float64
    Bid       float64
    Ask       float64
    FetchedAt time.Time
    Stale     bool            // true if last fetch failed
    Age       time.Duration   // time.Since(FetchedAt)
}
```

Always replaced atomically (never mutated in place), so reader snapshots are consistent.

### `MarketTracker` — Sliding Window (internal/stream/market_tracker.go)

```go
type MarketTracker struct {
    mu       sync.RWMutex
    windows  map[string]*PriceWindow      // per-symbol ring buffer
    breakers map[string]*CircuitBreaker   // per-symbol breaker state
    
    config   MarketTrackerConfig
}

type MarketTrackerConfig struct {
    WindowSizes      []time.Duration  // [10s, 30s, 60s]
    Resolution       time.Duration    // 1s — tick storage resolution
    MaxWindow        time.Duration    // 120s — max data stored
    BreakerThreshold float64          // -0.02 = -2% triggers breaker
    BreakerHysteresis float64         // -0.01 = -1% recovery threshold
    BreakerCooldown  time.Duration    // 30s min breaker active
    BreakerMaxAge    time.Duration    // 5min auto-reset
}

type PriceWindow struct {
    Symbol   string
    ring     []PricePoint      // cap = MaxWindow / Resolution
    head     int
    count    int
    full     bool
}

type PricePoint struct {
    Price     float64
    Timestamp time.Time
}

type CircuitBreaker struct {
    Symbol       string
    Active       bool
    ActivatedAt  time.Time
    TriggerDelta float64    // what triggered it (for diagnostics)
}
```

Single writer per symbol (fetch goroutine), many readers (bots). Uses RWMutex.

### `Debouncer` — Execution Flow Control (internal/stream/debouncer.go)

```go
type Debouncer struct {
    mu           sync.Mutex                 // per-bot instance
    botID        string
    lastOrderAt  time.Time
    cooldown     time.Duration   // 5s default between orders
    orderCount   int             // orders in current burst window
    burstWindow  time.Duration   // 30s tracking window
    burstLimit   int             // 10 max orders per burst window
    windowStart  time.Time       // reset when window expires
}
```

One `Debouncer` per bot. Lives in the Bot struct or managed by Supervisor.

### `TradingDigest` — AI-Optimized Context (internal/stream/digest.go)

```go
type TradingDigest struct {
    Ts   int64              `json:"ts"`    // unix ms
    
    // Account
    Eq   float64            `json:"eq"`    // total equity USD
    Bal  map[string]float64 `json:"bal"`   // asset → free balance
    
    // Markets (per active symbol)
    Mkt  map[string]MktSnap `json:"mkt"`
    
    // Bots
    Bot  []BotSnap          `json:"bot"`
    
    // Recent activity (last 5 orders)
    Ord  []OrdSnap          `json:"ord"`
    
    // Alerts
    Alrt []string           `json:"alrt"`  // "CB:BTC-USD", "BAL:LOW:USD"
}

type MktSnap struct {
    P    float64 `json:"p"`      // price
    Ch1h float64 `json:"ch1h"`   // 1h % change
    S    float64 `json:"s"`      // spread %
    CB   bool    `json:"cb"`     // circuit breaker active
}

type BotSnap struct {
    ID    string  `json:"id"`
    St    string  `json:"st"`    // strategy: dca/grid/signal
    Sym   string  `json:"sym"`
    Sta   string  `json:"sta"`   // running/paused/error
    PnL   float64 `json:"pnl"`
    Pos   float64 `json:"pos"`   // position size
    Entry float64 `json:"entry"` // avg entry
}

type OrdSnap struct {
    S  string  `json:"s"`       // buy/sell
    P  float64 `json:"p"`
    Q  float64 `json:"q"`
    St string  `json:"st"`      // filled/open/cancelled
    T  int64   `json:"t"`       // unix ms
}
```

Single-character JSON keys minimize LLM token costs on repeated reads. Semantic mapping documented in code comments and tool description.

## Data Flow — Step by Step Through a Tick

```
SETUP (Supervisor.StartBot)
1. Bot requests start
2. Supervisor calls PriceStreamer.Register("BTC-USD", 10s)
3. RefCount: 0 → 1 → PriceStreamer starts fetch goroutine for BTC-USD
4. If RefCount was already >0, just increments counter (no new goroutine)
5. Supervisor creates Debouncer for this bot
6. Bot starts tick loop with: streamer, debouncer, exchange reference

TICK LOOP (Bot, every 100ms)
1. price := streamer.GetCached("BTC-USD")
2. If price.Stale && price.Age > maxStaleAge → log warning, skip this tick
3. breaker := streamer.GetBreaker("BTC-USD")
4. If strategy is BUY and breaker.Active:
   - If DCA safety order: still place (averaging down IS the point)
   - If speculative buy: SKIP — don't catch a falling knife
5. If CanExecute(botID):  ← debounce check
   - Can place order → execute, RecordExecution(botID)
   - Cannot → skip this tick, log at debug level

BACKGROUND FETCH (PriceStreamer, every 10s per symbol)
1. ticker, err := exchange.GetTicker(ctx, symbol)
2. If err:
   - ConsecutiveFailures++, apply backoff
   - Set CachedTicker.Stale = true, keep old price
3. If success:
   - Replace CachedTicker atomically
   - Feed price to MarketTracker.Record(symbol, price)
4. MarketTracker checks Δ vs sliding window:
   - Δ = (price_now - price_[10s_ago]) / price_[10s_ago] * 100
   - If Δ < BreakerThreshold (-2%):
     - Activate circuit breaker
     - Log WARN with full context
   - If breaker was active AND Δ > BreakerHysteresis (-1%):
     - Deactivate circuit breaker
     - Log INFO recovery

SHUTDOWN (Supervisor.StopBot)
1. Bot stops
2. Supervisor calls PriceStreamer.Unregister("BTC-USD")
3. RefCount: 1 → 0
4. PriceStreamer cancels fetch goroutine for BTC-USD
5. Debouncer released (GC)
```

## Circuit Breaker Algorithm (pseudocode)

```
FUNC RecordTick(symbol, price, timestamp):
    window := windows[symbol]
    window.Append(PricePoint{price, timestamp})
    
    FOR EACH horizon IN config.WindowSizes:
        oldPrice := window.GetPriceAt(timestamp - horizon)
        IF oldPrice exists:
            delta := (price - oldPrice) / oldPrice * 100
            
            IF delta < config.BreakerThreshold:
                breaker := breakers[symbol]
                IF !breaker.Active:
                    breaker.Active = true
                    breaker.ActivatedAt = now
                    breaker.TriggerDelta = delta
                    LOG WARNING "circuit breaker activated: symbol=%s delta=%.2f%% horizon=%s"
            
            ELSE IF delta > config.BreakerHysteresis:
                breaker := breakers[symbol]
                IF breaker.Active AND (now - breaker.ActivatedAt > config.BreakerCooldown):
                    breaker.Active = false
                    LOG INFO "circuit breaker reset: symbol=%s delta=%.2f%%"
    
    // Auto-reset if breaker active too long without re-trigger
    breaker := breakers[symbol]
    IF breaker.Active AND (now - breaker.ActivatedAt > config.BreakerMaxAge):
        breaker.Active = false
        LOG INFO "circuit breaker auto-reset: symbol=%s (max age)"

// Called by bots before placing orders
FUNC IsBreakerActive(symbol) → bool:
    breaker := breakers[symbol]
    IF breaker == nil: return false
    RETURN breaker.Active
```

## Integration Strategy

### New Package: `internal/stream/`

| File | Purpose |
|------|---------|
| `types.go` | CachedTicker, SymbolRegistration, all config types |
| `price_streamer.go` | PriceStreamer struct, Register/Unregister, GetCached, fetch loop |
| `market_tracker.go` | MarketTracker struct, Record/GetBreaker/GetDelta/GetMetrics |
| `debouncer.go` | Debouncer struct, CanExecute/RecordExecution |
| `digest.go` | DigestBuilder struct, Build method |
| Plus `_test.go` files | |

### Files Modified

| File | Changes |
|------|---------|
| `internal/bot/supervisor.go` | Create PriceStreamer at init. Register on StartBot, Unregister on StopBot. Pass streamer + debouncer to Bot. Health-check streamer.healthy. |
| `internal/bot/bot.go` | Tick loop reads cached price instead of Exchange.GetTicker. Checks breaker before speculative buys. Checks debouncer before orders. |
| `internal/mcp/server.go` | New tool `get_digest` returns TradingDigest. Augment `list_bots` with circuit breaker column. Remove explicit GetTicker/GetCandles from tools? (still useful for one-off queries). |
| `cmd/greedy/main.go` | Wire stream components. Pass master config for breaker/debounce thresholds. |

### What Stays the Same

- Exchange interface: unchanged
- Strategy implementations (DCA, GRID, Signal): unchanged — they just read from cache instead of calling exchange
- Config system: unchanged — stream config is runtime, not YAML
- MCP transport: unchanged
- Backtest engine: unchanged — uses its own CSV data, not PriceStreamer

## Key Decisions

1. **Safety orders bypass breaker.** DCA averaging down IS the strategy — you WANT to buy dips. Circuit breaker blocks speculative buys, not planned safety orders.
2. **Debouncer is per-bot, not per-symbol.** Two bots on different strategies can both trade; internal logic decides validity.
3. **PriceStreamer only wraps GetTicker.** Positions and balances need real-time freshness. Rate limits are generous for those endpoints.
4. **Single-character JSON keys.** Real cost savings for LLMs reading MCP output repeatedly. Documented in struct comments.
5. **Reference counting, not heartbeats.** Cleaner lifecycle — bots register on start, unregister on stop. No timer-based cleanup needed.

## Edge Cases & Safety

| Scenario | Handling |
|----------|----------|
| Fetch goroutine panics | recover() in loop, set healthy=false, supervisor restarts |
| All bots stop | RefCount → 0, goroutine cancelled, no leak |
| Exchange returns garbage | Plausibility check: >50% price change → flag stale, don't update cache |
| Race: bot reads while streamer writes | CachedTicker swapped atomically via pointer/map write under lock. Reader gets consistent snapshot |
| 100 symbols active | Each = 1 goroutine + ring buffer. Bounded by active symbols. 10k points/symbol at 1s resolution. ~1MB RAM. Fine |
| Breaker stays active forever | BreakerMaxAge (5min) auto-reset |
| Bot crashes mid-tick | Defer in tick loop handles it. RefCount decremented by StopBot |
| Bitcoin flash crashes 30% in 10s | Circuit breaker activates, bots stop buying, AI gets alert via digest. Human/LLM decides |

## Vertical Slice Issue (GitHub)

**Title:** Resilient Trading Layer — Sliding Window State, Shared Fetcher, and AI Digest

**Labels:** `area/stream`, `enhancement`, `phase`

**Description:**

Implement the `internal/stream` package with three components and integrate them into the bot supervisor and MCP server.

**Acceptance Criteria:**

1. **PriceStreamer**: One fetch goroutine per symbol, reference-counted registration, stale-data policy
2. **MarketTracker**: Per-symbol ring buffer, Δ% calculation over configurable horizons, circuit breaker activation/deactivation
3. **Debouncer**: Per-bot cooldown + burst limit
4. **Bot integration**: Tick loop reads from cache, checks breaker before speculative buys, checks debouncer before orders
5. **Safety orders bypass breaker** — DCA averaging down always executes
6. **MCP `get_digest` tool**: Returns condensed TradingDigest JSON (≤15 lines)
7. **MCP `list_bots` augmented** with circuit breaker status
8. **Tests with `-race` flag**: Concurrency tests for streamer, tracker, debouncer
9. **Edge case tests**: Fetch failure → stale data, breaker activation → recovery → auto-reset, refCount zero → goroutine cleanup
10. **No breaking changes** to existing exchange interface, strategy implementations, or backtest engine

**Files:**
- Create: `internal/stream/types.go`, `price_streamer.go`, `market_tracker.go`, `debouncer.go`, `digest.go` + `_test.go`
- Modify: `internal/bot/supervisor.go`, `internal/bot/bot.go`, `internal/mcp/server.go`, `cmd/greedy/main.go`

**Estimated complexity:** Medium. ~500 lines of new code, ~300 lines of tests.
