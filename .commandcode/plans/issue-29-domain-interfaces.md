# Issue #29 — Extract domain interfaces

**Phase 0.1 of Clean Architecture Migration**
**Goal:** Create `internal/domain/` with pure interfaces + types. ZERO new features. All 71 tests pass.

---

## What moves where

### `internal/exchange/` → `internal/domain/exchange/` (entire package)
Pure domain — zero internal imports. No changes to file contents.
- exchange.go (Exchange interface, 14 methods)
- types.go (OrderSide, OrderStatus, Order, Balance, Position...)
- errors.go (sentinel errors)

### `internal/bot/strategy.go` → `internal/domain/bot/` (split into 3 files)
- `strategy.go`: Strategy interface, Action, Signal, BotState
- `types.go`: Status, RestartPolicy, BotStatus
- `supervisor.go`: Supervisor interface, BotConfig interface

### `internal/config/` partial → `internal/domain/config/`
- config.go (BotConfig, DCAConfig, GridConfig, SignalConfig, SafetyOrder)
- params.go (ParseFloatParam, ParseIntParam, ParseDurationParam)
- defaults.go (DefaultDCAConfig(), constants)

Loader.go stays in `internal/config/` — it's infrastructure (YAML+os.ReadFile).

---

## Import updates — 17 files
All consumer files get import path updates. Zero behavior changes.

## Files created: 10 | Files removed: 4 | Files modified: 17

## Verification
```bash
go build ./cmd/greedy && go test ./... -count=1 && go vet ./... && golangci-lint run
```

## Key risk
`domain_bot.BotConfig` interface needs 4 methods (ID, Name, StrategyType, Symbol). Concrete `config.BotConfig` must implement them — need to add 4 getter methods.
