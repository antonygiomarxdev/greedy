# Phase 1: Stream Layer — Vertical Slice Plan v2

> Refined 2026-05-22 after codebase pattern analysis.  
> Each slice: `domain.go` interface → implementation → tests → `go test -race`.  
> Commit por slice. Paper-only.

## Patterns We Follow

These are extracted from the existing codebase (`trading/`, `mcp/`, `backtest/`):

| Pattern | Example | Rule |
|---------|---------|------|
| **Interface en domain.go** | `trading/domain.go` → `Strategy interface` | Todo slice define su abstracción en `domain.go` |
| **Constructor `New*()`** | `NewSupervisor(ex, db, policy)` | Inyección explícita, sin DI framework |
| **shared.Exchange como dependencia universal** | `Bot.Exchange shared.Exchange` | Campo público, tipado con interfaz de `shared/` |
| **Context propagation** | `signal.NotifyContext` → `WithCancel` por bot → `WithTimeout` por tick | Raíz → Supervisor → Bot → Operación |
| **CancelFunc en map** | `Supervisor.cancels map[string]context.CancelFunc` | Un cancel por recurso |
| **sync.WaitGroup para goroutines** | `Supervisor.wg` en StartBot/ShutdownCtx | Track + wait con deadline |
| **Opción "tipo inyectado puede ser nil"** | `PriceStreamer.SetRateLimiter(nil)` para paper | Interfaces opcionales con setters separados |

## Slices (6 slices, orden de dependencia)

```
Slice 1: PriceStore          (SQLite — no dependencias)
    ↓
Slice 2: PriceStreamer        (usa PriceStore, shared.Exchange)
    ↓
Slice 3: MarketTracker        (usa PriceStore para warm-start, no depende de PriceStreamer)
    ↓
Slice 4: Debouncer            (sin dependencias)
    ↓
Slice 5: IdempotencyStore     (SQLite — sin dependencias)
    ↓
Slice 6: Bot Integration      (wirea 1-5 en el tick loop del bot)
```

### Lo que NO está en esta fase (Phase 2: Real Exchanges)

| Componente | Por qué no ahora |
|------------|-----------------|
| RateLimiter | Paper es infinito. Solo aplica a APIs con rate limits |
| OrderReconciler | Paper resuelve órdenes al instante. No hay pending/expired |
| ExchangeHealthMonitor | Paper nunca falla |
| WebSocket | Paper no tiene WebSocket |
| Portfolio valuation | Necesita múltiples assets y cross-pairs, follow-up |
| TradingDigest | Depende de portfolio + market tracker maduro |

---

## Slice 1: PriceStore

### Propósito
Persistir ticks de precio en SQLite. Sirve para MarketTracker warm-start después de restart.

### Estructura
```
internal/pricestore/
├── domain.go     // PricePoint, PriceStore interface
├── sqlite.go     // SQLitePriceStore implementación
└── sqlite_test.go

internal/infrastructure/db/migrations/
└── 003_price_ticks.sql
```

### Interface (`domain.go`)
```go
package pricestore

type PricePoint struct {
    Symbol    string
    Price     float64
    Timestamp time.Time
}

type PriceStore interface {
    Insert(ctx context.Context, p PricePoint) error
    QueryWindow(ctx context.Context, symbol string, from, to time.Time) ([]PricePoint, error)
    Prune(ctx context.Context, olderThan time.Duration) (int64, error)
}
```

### Constructor
```go
func NewSQLitePriceStore(db *sql.DB) *SQLitePriceStore
```

Patrón: recibe `*sql.DB` (como `BotRepository` en `internal/infrastructure/db/bot_repo.go`).

### Schema
```sql
CREATE TABLE IF NOT EXISTS price_ticks (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol    TEXT    NOT NULL,
    price     REAL    NOT NULL,
    timestamp INTEGER NOT NULL,
    created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
CREATE INDEX idx_price_ticks_symbol_ts ON price_ticks(symbol, timestamp);
```

### Tests
- Insert 100 ticks, QueryWindow recupera todos en orden
- Prune borra ticks viejos, retiene recientes
- QueryWindow vacío (símbolo sin datos)
- Concurrencia: Insert + Query simultáneos sin race

---

## Slice 2: PriceStreamer

### Propósito
Un solo fetch por símbolo compartido entre N bots. Referencia contada: `Register(symbol)` incrementa refCount, `Unregister(symbol)` decrementa. Si refCount llega a 0, la goroutine de fetch muere.

Los bots leen del cache con `GetCached(symbol)` — solo RLock, sin contención.

### Estructura
```
internal/pricestreamer/
├── domain.go        // CachedTicker, PriceStreamer interface
├── streamer.go      // Streamer implementación
└── streamer_test.go
```

### Interface (`domain.go`)
```go
package pricestreamer

type CachedTicker struct {
    Symbol    string
    Price     float64
    Bid       float64
    Ask       float64
    Timestamp time.Time
    Stale     bool
}

type PriceStreamer interface {
    Register(ctx context.Context, symbol string, interval time.Duration) error
    Unregister(symbol string)
    GetCached(symbol string) (CachedTicker, bool)

    // Opcionales — paper no los necesita pero permiten composición
    SetPriceStore(store PriceStore)
    OnTick(fn func(symbol string, price float64, ts time.Time))
}
```

### Constructor
```go
func New(exchange shared.Exchange) *Streamer
```

### Implementación
```go
type symbolState struct {
    refCount int
    cancel   context.CancelFunc
    cache    CachedTicker
    mu       sync.RWMutex
}

type Streamer struct {
    exchange   shared.Exchange
    priceStore PriceStore
    onTick     func(string, float64, time.Time)
    symbols    map[string]*symbolState
    mu         sync.RWMutex
    logger     *slog.Logger
}
```

**Register:** si `refCount == 0`, crea `ctx, cancel` y lanza goroutine que fetchea periódicamente vía `exchange.GetTicker()`. Actualiza cache, persiste en PriceStore, dispara callback.

**Unregister:** decrementa refCount. Si llega a 0, llama `cancel()` y borra del map.

**GetCached:** `RLock`, devuelve cache + bool (found).

### Tests
- Register → GetCached devuelve precio después del primer tick
- Register × 2 (mismo símbolo) → refCount=2, una sola goroutine
- Unregister × 2 → goroutine muere en el segundo
- GetCached con símbolo no registrado → false
- Error de exchange → CachedTicker.Stale = true
- Concurrencia: Register + GetCached simultáneos
- Callback OnTick se dispara en cada update

---

## Slice 3: MarketTracker

### Propósito
Ring buffer por símbolo con sliding window de precios. Calcula Δ% sobre horizontes configurables. Circuit breaker: si Δ% > umbral, bloquea órdenes de compra por N segundos.

Warm-start desde PriceStore: en startup, carga los últimos 120s para pre-llenar buffers.

### Estructura
```
internal/markettracker/
├── domain.go        // MarketSnap, BreakerConfig, MarketTracker interface
├── tracker.go       // ring buffer + circuit breaker
├── restore.go       // warm-start desde PriceStore
└── tracker_test.go
```

### Interface (`domain.go`)
```go
package markettracker

type BreakerConfig struct {
    MaxPriceDeltaPct  float64
    WindowDuration    time.Duration
    CooldownDuration  time.Duration
}

type MarketSnap struct {
    Symbol        string
    CurrentPrice  float64
    DeltaPct      float64
    BreakerActive bool
    BreakerUntil  time.Time
}

type MarketTracker interface {
    Record(symbol string, price float64, timestamp time.Time)
    GetSnapshot(symbol string) MarketSnap
    IsBreakerActive(symbol string) bool
    Restore(ctx context.Context, symbols []string, store PriceStore) error
}
```

### Constructor
```go
func New(cfg BreakerConfig) *Tracker
```

### Tests
- Ring buffer overflow: insertar 200 puntos, verificar solo últimos N
- Δ% positivo → breaker activado
- Δ% dentro del umbral → breaker no activado
- Breaker cooldown expira → IsBreakerActive = false
- Restore desde PriceStore con 50 ticks → ring buffer pre-llenado
- Restore sin datos → buffer vacío, no panic
- Concurrencia: Record + GetSnapshot simultáneos

---

## Slice 4: Debouncer

### Propósito
Previene que un bot coloque demasiadas órdenes en rápida sucesión.

### Estructura
```
internal/debouncer/
├── domain.go      // Debouncer interface
├── debouncer.go   // implementación
└── debouncer_test.go
```

### Interface (`domain.go`)
```go
package debouncer

type Debouncer interface {
    CanExecute() bool
    RecordExecution()
    Reset()
}
```

### Constructor
```go
func New(cooldown time.Duration, burstLimit int, burstWindow time.Duration) *BotDebouncer
```

### Tests
- Cooldown bloquea inmediatamente después de ejecutar
- Cooldown expira → permite
- Burst limit se agota
- Ventana de burst se limpia de entradas viejas
- Reset limpia todo

---

## Slice 5: IdempotencyStore

### Propósito
Evitar órdenes duplicadas. Guarda `ClientOrderID` en SQLite antes de colocar la orden.

### Estructura
```
internal/idempotency/
├── domain.go      // IdempotencyStore interface
├── sqlite.go      // SQLiteIdempotencyStore implementación
└── sqlite_test.go

internal/infrastructure/db/migrations/
└── 004_idempotency_keys.sql
```

### Interface (`domain.go`)
```go
package idempotency

type Record struct {
    ClientOrderID   string
    ExchangeOrderID string
    BotID           string
    Symbol          string
    Status          string
}

type Store interface {
    Reserve(ctx context.Context, clientOrderID, botID, symbol string) error
    Confirm(ctx context.Context, clientOrderID, exchangeOrderID string) error
    Lookup(ctx context.Context, clientOrderID string) (Record, error)
}
```

### ClientOrderID format
`{botID}-{unixMilli}-{seq}` — ej: `dca-btc-1716400000000-0005`

### Tests
- Reserve OK, Reserve misma key → error
- Confirm actualiza, Lookup devuelve confirmado
- Lookup key inexistente → error
- Concurrencia: dos goroutines Reserve misma key → una gana

---

## Slice 6: Bot Integration

### Propósito
Wirear los 5 slices en el tick loop del bot.

### Cambios en `trading/bot.go`

**Struct:**
```go
type Bot struct {
    // ... campos existentes ...
    streamer    pricestreamer.PriceStreamer  // NEW (nil-safe)
    tracker     markettracker.MarketTracker  // NEW (nil-safe)
    debouncer   debouncer.Debouncer          // NEW (nil-safe)
    idempotency idempotency.Store            // NEW (nil-safe)
    seq         uint64                       // NEW
}
```

**Tick loop actualizado:**
1. `streamer.GetCached(symbol)` en vez de `exchange.GetTicker()` → 0 llamadas redundantes
2. `tracker.IsBreakerActive(symbol)` → skip si breaker activo
3. `debouncer.CanExecute()` → skip si en cooldown o burst agotado
4. `idempotency.Reserve(clientOrderID)` → previene duplicados
5. `debouncer.RecordExecution()` después de orden exitosa

### Cambios en `trading/supervisor.go`

Campos `streamer`, `tracker`, `idempotency`.  
`StartBot()` crea `debouncer.New()` por bot y lo inyecta.

### Cambios en `trading/delivery/serve.go`

Wirear: PriceStore → PriceStreamer → MarketTracker (callback). Registrar símbolos. Warm-start tracker. Inyectar en supervisor.

---

## Plan de commits

| Commit | Slice | Archivos |
|--------|-------|----------|
| 1 | PriceStore | `pricestore/domain.go`, `sqlite.go`, `sqlite_test.go`, `003_price_ticks.sql` |
| 2 | PriceStreamer | `pricestreamer/domain.go`, `streamer.go`, `streamer_test.go` |
| 3 | MarketTracker | `markettracker/domain.go`, `tracker.go`, `restore.go`, `tracker_test.go` |
| 4 | Debouncer | `debouncer/domain.go`, `debouncer.go`, `debouncer_test.go` |
| 5 | IdempotencyStore | `idempotency/domain.go`, `sqlite.go`, `sqlite_test.go`, `004_idempotency_keys.sql` |
| 6 | Bot Integration | Modifica: `trading/bot.go`, `trading/supervisor.go`, `trading/delivery/serve.go` |

## Verification final

```bash
go test -race -count=1 ./...
greedy serve --config examples/greedy.yaml
# → streamer + tracker + bots corriendo
# → Ctrl+C shutdown limpio
greedy serve --config examples/greedy.yaml
# → "market tracker restored N ticks" (warm start!)
```
