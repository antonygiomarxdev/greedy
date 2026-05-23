# Phase 6: Real Exchange Connectors — Plan

## Current State

Paper trading is complete. All delivery layers hardcode `paper.New()`.
`BotConfig.Exchange` field exists but is never used to select the exchange.
The crypto package (NaCl secretbox + Argon2id) is built but not integrated.

## Goals

1. `greedy serve --config greedy.yaml` can select "coinbase" or "paper" per bot
2. API keys encrypted at rest, never in plaintext
3. Coinbase REST connector implementing the full `shared.Exchange` interface
4. Exchange factory that constructs the right connector from config
5. Backward compatible — paper is still the default, all existing tests pass

## Architecture Overview

```
YAML config                Crypto + DB                Exchange Factory
─────────────              ─────────────              ────────────────
bots:                      CredentialStore            factory.New("coinbase")
  - exchange: coinbase     ├─ Encrypt(key, secret)    ├─ paper → PaperExchange
    coinbase:               ├─ Decrypt(blob, key)     ├─ coinbase → CoinbaseExchange
      api_key: env:GREEDY_CB_KEY                      └─ (binance → future)
      api_secret: env:GREEDY_CB_SECRET
```

The exchange field in YAML controls which connector is created.
Credentials are stored encrypted via `greedy config set-exchange`.
A `--master-password` flag or `GREEDY_MASTER_KEY` env var derives the encryption key.

## Slices (3 slices, ordered by dependency)

```
Slice 1: CredentialStore + ExchangeConfig
    ↓
Slice 2: Coinbase REST Connector
    ↓
Slice 3: Exchange Factory + CLI/MCP Integration
```

---

## Slice 1: CredentialStore + ExchangeConfig

### Propósito
Almacenar API keys encriptadas en SQLite. El usuario las configura una vez.
La master password deriva la clave de cifrado (Argon2id).

### Archivos
```
internal/credentials/
├── domain.go         // Credential, CredentialStore interface
├── store.go          // SQLiteCredentialStore (encrypt/decrypt)
├── store_test.go

internal/infrastructure/db/migrations/
└── 005_credentials.sql

internal/infrastructure/config/
└── config.go (modify) // Add ExchangeConfig + CoinbaseConfig
```

### Interface
```go
type Credential struct {
    Exchange   string // "coinbase"
    Label      string // user-assigned name
    APIKey     string // encrypted at rest
    APISecret  string // encrypted at rest
    Passphrase string // encrypted at rest (Coinbase-specific)
}

type CredentialStore interface {
    Set(ctx context.Context, cred Credential, masterKey *[32]byte) error
    Get(ctx context.Context, exchange, label string, masterKey *[32]byte) (*Credential, error)
    List(ctx context.Context) ([]CredentialMeta, error)
    Delete(ctx context.Context, exchange, label string) error
}
```

### Schema
```sql
CREATE TABLE IF NOT EXISTS credentials (
    exchange    TEXT NOT NULL,
    label       TEXT NOT NULL DEFAULT 'default',
    api_key     BLOB NOT NULL,     -- NaCl encrypted
    api_secret  BLOB NOT NULL,     -- NaCl encrypted
    passphrase  BLOB,              -- NaCl encrypted (nullable)
    salt        BLOB NOT NULL,     -- Argon2 salt for key derivation
    created_at  INTEGER NOT NULL DEFAULT (unixepoch()),
    PRIMARY KEY (exchange, label)
);
```

### Config Changes
```go
type RootConfig struct {
    DataDir   string           `yaml:"data_dir"`
    Bots      []BotConfig      `yaml:"bots,omitempty"`
    Exchanges []ExchangeConfig `yaml:"exchanges,omitempty"`
    MasterKey string           `yaml:"master_key,omitempty"` // or env:GREEDY_MASTER_KEY
}

type BotConfig struct {
    // ... existing fields ...
    CredentialLabel string `yaml:"credential,omitempty"` // "default" or custom label
}

type ExchangeConfig struct {
    Name     string          `yaml:"name"` // "coinbase"
    Label    string          `yaml:"label,omitempty"` // defaults to "default"
    Provider string          `yaml:"provider"` // "coinbase"
    Sandbox  bool            `yaml:"sandbox,omitempty"`
    Coinbase *CoinbaseConfig `yaml:"coinbase,omitempty"`
}

type CoinbaseConfig struct {
    RESTBaseURL    string `yaml:"rest_url,omitempty"` // default: api.coinbase.com
    WebsocketURL   string `yaml:"ws_url,omitempty"`   // default: advanced-trade-ws.coinbase.com
    RequestsPerSec int    `yaml:"requests_per_sec,omitempty"` // default: 10
}
```

### CLI
```bash
# Set credentials (encrypts + stores)
greedy config set-exchange --exchange coinbase --key $KEY --secret $SECRET

# List stored credentials (metadata only, no secrets)
greedy config list-exchanges

# Delete
greedy config delete-exchange --exchange coinbase
```

---

## Slice 2: Coinbase REST Connector

### Propósito
Implementación completa del `shared.Exchange` interface usando la REST API de Coinbase Advanced Trade. HMAC-SHA256 authentication. No WebSocket en esta fase (solo REST polling).

### Archivos
```
internal/exchange/coinbase/
├── coinbase.go       // CoinbaseExchange struct + New()
├── auth.go           // HMAC-SHA256 signing (CB-ACCESS-* headers)
├── rest.go           // HTTP client, request builder, response parser
├── ticker.go         // GetTicker implementation
├── orders.go         // PlaceOrder, CancelOrder, GetOrder, ListOpenOrders
├── account.go        // GetBalance, ListBalances
├── products.go       // GetOrderBook, GetCandles
├── coinbase_test.go  // Integration tests with sandbox
```

### Struct
```go
type CoinbaseExchange struct {
    client      *http.Client
    apiKey      string
    apiSecret   string
    restBaseURL string
    rateLimiter *RateLimiter  // token bucket, 10 req/s default
}

func New(cfg CoinbaseConfig, cred *Credential) *CoinbaseExchange
```

### HMAC Auth (auth.go)
```
Headers:
  CB-ACCESS-KEY:        apiKey
  CB-ACCESS-TIMESTAMP:  unix seconds
  CB-ACCESS-SIGN:       HMAC-SHA256(timestamp + method + path + body, secret)
  CB-ACCESS-PASSPHRASE: passphrase (optional for portfolios)
```

### Product IDs
Coinbase uses `BTC-USD` (same as our symbol convention) — no mapping needed.
Binance would need `BTCUSDT` → `BTC-USD` mapping, but that's for later.

### Rate Limiting
Token bucket: 10 requests/sec (public endpoints), 30 requests/sec (private).
Proactive: `Wait()` before each HTTP call. If 429 received, halves the bucket.

### What's NOT in this slice
- WebSocket (Phase 6.1 follow-up)
- Binance connector (separate slice after Coinbase)
- Position tracking via Coinbase portfolios (Phase 6.2)
- Order reconciliation against Coinbase (needs order state machine first)

---

## Slice 3: Exchange Factory + Integration

### Propósito
Wire the exchange factory into all delivery layers. `BotConfig.Exchange` now
actually drives construction. Paper is still the default.

### Archivos
```
internal/exchange/
├── factory.go        // func New(name, cfg, cred) (shared.Exchange, error)
└── factory_test.go

Modified:
  internal/trading/delivery/serve.go  // use factory instead of paper.New()
  internal/trading/delivery/cli.go    // use factory
  internal/mcp/delivery/cli.go        // use factory
  internal/mcp/commands.go            // handle non-MarketLifecycleManager exchanges
  internal/backtest/engine.go         // keep paper only (backtest is paper by design)
  cmd/greedy/main.go                  // add "config" subcommand
```

### Factory
```go
func New(name string, cfg ExchangeConfig, cred *Credential) (shared.Exchange, error) {
    switch name {
    case "paper", "":
        return paper.New(shared.DefaultFeeRate), nil
    case "coinbase":
        return coinbase.New(cfg.Coinbase, cred)
    default:
        return nil, fmt.Errorf("unknown exchange: %s", name)
    }
}
```

### MCP add_market handling
`addMarketCommand` currently type-asserts to `MarketLifecycleManager`.
Real exchanges don't implement this. The command should:
- For paper: call `AddMarket` + `SeedLiquidity` (current behavior)
- For real exchanges: return error "add_market only supported in paper mode"

### CLI
```bash
greedy config set-exchange --exchange coinbase --label default --key xxx --secret yyy
greedy config list-exchanges
```

### Example YAML (greedy.yaml)
```yaml
exchanges:
  - name: coinbase
    provider: coinbase
    sandbox: true
    coinbase:
      requests_per_sec: 8

bots:
  - id: dca-btc
    exchange: coinbase
    credential: default
    strategy:
      type: dca
      symbol: BTC-USD
      params:
        base_order_size: 100
        frequency: "1h"
```

---

## Dependency Flow

```
CredentialStore ← crypto package (existing)
       ↓
CoinbaseExchange implements shared.Exchange
       ↓
Exchange Factory (New("coinbase", cfg, cred))
       ↓
serve.go / cli.go / mcp delivery → factory.New(cfg.Exchange, ...)
```

## Verification

```bash
# Set sandbox credentials
greedy config set-exchange --exchange coinbase --key $SANDBOX_KEY --secret $SANDBOX_SECRET

# Run with real (sandbox) exchange
greedy serve --config greedy.yaml
# → "connected to coinbase sandbox"
# → bot ticks use real REST API with HMAC auth
# → orders appear in Coinbase sandbox dashboard

# Verify encryption
sqlite3 ~/.greedy/greedy.db "SELECT exchange, label, length(api_key) FROM credentials;"
# → api_key column is binary (encrypted), not plaintext

# Backward compat
greedy serve --strategy dca.yaml
# → defaults to paper exchange, works exactly as before
```

## What This Does NOT Include (Follow-ups)

- Binance connector
- WebSocket support for real exchanges
- Order reconciliation (pending/expired states from real exchange)
- Exchange health monitoring (5xx detection, circuit breaking)
- Rate limit adaptive backoff
- Portfolio-level position tracking (Coinbase portfolios API)
