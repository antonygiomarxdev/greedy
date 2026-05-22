# Unified Daemon Architecture: `greedy serve`

## Problem

`greedy run` and `greedy mcp-serve` are independent processes with independent exchange state. The product needs a single daemon that runs bots AND accepts MCP commands — sharing one exchange, one supervisor, one source of truth.

## Approach

`ServeCommand` in `internal/trading/delivery/serve.go` (same pattern as existing `RunCommand` and `MCPServeCommand`). No new package — follows vertical slicing. It orchestrates shared DB, Exchange, Supervisor, and optionally wires MCP server as a goroutine.

## Decisions Made

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Transport | stdio only | AI tools (Claude, Cursor, Windsurf) only support stdio MCP today. Unix socket / TCP when ecosystem catches up |
| New package? | No — `trading/delivery/serve.go` | Follows existing pattern. Don't pre-optimize |
| Backward compat | `run` and `mcp-serve` become kernel wrappers | Install script stays working. migrate to `serve` later |
| Strategy state persistence | Not in this phase | Exchange state (positions, balances) survives restart. Strategies restart fresh but see existing positions |
| Multi-exchange | No change needed | `shared.Exchange` interface already abstracts this |
| YAML config | Simplified | No server.mcp boilerplate. MCP always on in serve mode |
| Multiple AI clients | Not supported yet | stdio is 1:1 by design. Single AI tool per daemon |

## Files to Create

| File | Purpose |
|------|---------|
| `internal/trading/delivery/serve.go` | `ServeCommand()` — daemon lifecycle |
| `internal/infrastructure/db/migrations/002_exchange_state.sql` | Schema for exchange balances + positions |
| `internal/kernel/persistence.go` | `SnapshotExchange()` / `RestoreExchange()` |
| `internal/kernel/kernel_test.go` | Integration tests |
| `examples/greedy.yaml` | Example daemon config |

## Files to Modify

| File | Change |
|------|--------|
| `cmd/greedy/main.go` | Add `"serve"` command |
| `internal/infrastructure/config/config.go` | `RootConfig` already has `DataDir` + `Bots` — no new fields needed |
| `internal/infrastructure/paper/paper.go` | Add `Snapshot()` / `Restore()` methods |
| `internal/trading/supervisor.go` | Add `sync.WaitGroup`, `Shutdown(ctx)` with timeout |

## Design Details

### 1. `ServeCommand` (in `trading/delivery/serve.go`)

```go
func ServeCommand(ctx context.Context, logger *slog.Logger, args []string)
```

Lifecycle:
```
1. Parse flags: --config greedy.yaml, --strategy single.yaml, --mcp (default true)
2. db.Open(dataDir) + RunMigrations
3. Create paper.New() exchange
4. RestoreExchange() from DB if snapshot exists
5. Seed default markets + StartFeeds(ctx)
6. Create trading.NewSupervisor(exchange, db)
7. Bootstrap bots from config.Bots (YAML) or --strategy flag
8. If MCP: create mcp.NewServer(), launch ServeStdio in goroutine
9. Block on <-ctx.Done()
10. Shutdown: cancel() → supervisor.Shutdown(ctx) → SnapshotExchange() → db.Close()
```

### 2. YAML Config (`greedy.yaml`)

```yaml
data_dir: ~/.greedy
bots:
  - id: dca-btc
    name: "BTC DCA"
    strategy:
      type: dca
      symbol: BTC-USD
      params:
        base_order_size: 100
        frequency: "1h"
  - id: grid-eth
    name: "ETH GRID"
    strategy:
      type: grid
      symbol: ETH-USD
      params:
        lower_bound: 2000
        upper_bound: 4000
        grid_levels: 10
        order_size: 0.5
```

No `server.mcp` section — MCP is always on in serve mode. Use `--mcp=false` flag to disable.

### 3. CLI Contract

```bash
# Full daemon with multi-bot config
greedy serve --config greedy.yaml

# Single bot quick start (replaces greedy run)
greedy serve --strategy dca.yaml

# Daemon without MCP (trading only)
greedy serve --config greedy.yaml --mcp=false

# Backward compat (unchanged externally, delegate to kernel internally)
greedy run --strategy dca.yaml
greedy mcp-serve
```

### 4. State Persistence

Schema:
```sql
CREATE TABLE IF NOT EXISTS exchange_balances (
    asset TEXT PRIMARY KEY,
    free REAL NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS exchange_positions (
    symbol TEXT PRIMARY KEY,
    quantity REAL NOT NULL,
    avg_entry_price REAL NOT NULL,
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

Flow:
- **Shutdown**: DELETE all rows, INSERT current balances + positions
- **Startup**: SELECT all rows, call `exchange.Restore(balances, positions)`
- **Strategies**: restart fresh. They see restored positions but recalculate entry/exit logic

### 5. PaperExchange Changes

```go
func (pe *PaperExchange) Snapshot() ([]Balance, []Position)
func (pe *PaperExchange) Restore(balances []Balance, positions []Position)
```

### 6. Supervisor Changes

```go
// Add WaitGroup to track bot goroutines
wg sync.WaitGroup

// StartBot: wg.Add(1) before go bot.Run()
// bot.Run: defer wg.Done() at top

// New Shutdown with timeout
func (s *Supervisor) Shutdown(ctx context.Context) error {
    // 1. Cancel all bot contexts
    s.mu.Lock()
    for _, cancel := range s.cancels { cancel() }
    s.mu.Unlock()
    
    // 2. Wait for bots to finish (with timeout)
    done := make(chan struct{})
    go func() { s.wg.Wait(); close(done) }()
    select {
    case <-done:
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

### 7. Goroutine Layout

```
main → ServeCommand(ctx)
  ├─ exchange.StartFeeds(ctx)         → N feed goroutines
  ├─ supervisor.StartBot(ctx, ...) × N → N bot goroutines (+ WaitGroup)
  ├─ go mcpServer.ServeStdio(ctx)     → 1 MCP goroutine (if MCP enabled)
  └─ <-ctx.Done()                     → shutdown sequence
```

### 8. Backward Compatibility

`run` and `mcp-serve` handlers in `main.go` become thin wrappers:

```go
"run": func(ctx context.Context, logger *slog.Logger, args []string) {
    // Parse --strategy flag, load YAML
    botCfg, _ := config.LoadStrategyFile(stratFile, strategy.Validator())
    cfg := ServeConfig{
        DataDir:   botCfg.DataDir(),
        Bootstrap: []config.BotConfig{*botCfg},
        MCP:       false,
    }
    serveDaemon(ctx, logger, cfg)
}

"mcp-serve": func(ctx context.Context, logger *slog.Logger, args []string) {
    cfg := ServeConfig{
        DataDir: resolveDataDir(),
        MCP:     true,
    }
    serveDaemon(ctx, logger, cfg)
}
```

External behavior identical. Existing scripts and Claude Desktop configs keep working.

## Verification

### Integration Tests
- Start daemon with one bot → bot appears in positions after first tick
- MCP tools (`list_bots`, `get_positions`) return bootstrapped bot data
- `start_bot` via MCP adds a second bot → both visible
- SIGINT → graceful shutdown → exchange state persisted to DB
- Restart → positions restored, bots rebootstrapped
- Multiple bots from YAML config → all running

### Manual
```bash
greedy serve --config examples/greedy.yaml
# In another terminal (or via Claude):
#   list_bots → shows configured bots
#   get_positions → shows current positions
# Ctrl+C
sqlite3 ~/.greedy/greedy.db "SELECT * FROM exchange_balances;"
greedy serve --config examples/greedy.yaml
# → balances restored
```

## Out of Scope (Follow-ups)

- Per-bot strategy state persistence (DCA initialPrice, GRID grid levels)
- Unix socket / TCP MCP transport
- gRPC health endpoint
- Real exchange adapters (Coinbase, Binance)
- Multi-client MCP
- SIGHUP config reload
