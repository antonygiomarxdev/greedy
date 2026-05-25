# Strategy YAML Schema Reference

Canonical reference for all strategy types supported by Greedy.

## Root Format (greedy.yaml)

```yaml
data_dir: ~/.greedy      # optional, defaults to ~/.greedy
bots:
  - id: my-bot           # unique bot ID
    name: "My Bot"       # display name
    exchange: paper       # paper | binance | coinbase
    strategy:
      type: dca           # dca | grid | signal
      symbol: BTC-USD     # BASE-QUOTE format
      params:             # type-specific parameters below
    debouncer:            # optional rate limiting
      cooldown: 5s
      burst_limit: 10
      burst_window: 30s
```

## DCA (Dollar Cost Averaging)

```yaml
strategy:
  type: dca
  symbol: BTC-USD
  params:
    base_order_size: 100       # USD amount per regular buy
    frequency: "1h"            # Go duration: how often to buy
    safety_orders:             # optional: triggered on price drops
      - price_deviation_pct: -5    # negative = price drop from initial
        volume_scale: 1.5          # multiplier on base_order_size
      - price_deviation_pct: -10
        volume_scale: 2.0
    max_safety_orders: 5       # max total safety orders
```

**Behavior**: On first tick, records `initialPrice`. Every `frequency`, places
a market buy for `base_order_size / currentPrice` quantity. Safety orders
trigger exactly once per level when price drops below the deviation threshold.

## GRID

```yaml
strategy:
  type: grid
  symbol: ETH-USD
  params:
    lower_bound: 2000     # bottom of grid
    upper_bound: 4000     # top of grid
    grid_levels: 10       # number of price levels
    order_size: 0.5       # quantity per level
```

**Behavior**: Divides `upper_bound - lower_bound` into `grid_levels` equally
spaced price points. Places limit buy orders at each level for `order_size`
quantity. When filled, can re-buy at that level. No sell logic.

## Signal

```yaml
strategy:
  type: signal
  symbol: SOL-USD
  position_size: 1000    # USD amount per trade
```

**Behavior**: Manual trigger strategy. External code calls `Trigger("entry")`
or `Trigger("exit")` via MCP/channel. Entry: market buy for `positionSize / price`.
Exit: market sell entire position. Only one position at a time.

## Go Duration Format

All duration fields use Go's `time.ParseDuration` format:
- `"30s"` = 30 seconds
- `"5m"` = 5 minutes
- `"1h"` = 1 hour
- `"4h30m"` = 4 hours 30 minutes
- `"24h"` = 1 day

## Debouncer Config (optional)

```yaml
debouncer:
  cooldown: 5s         # minimum time between orders (default 5s)
  burst_limit: 10      # max orders in burst window (default 10)
  burst_window: 30s    # sliding window for burst limit (default 30s)
```

All fields optional. Omitted fields use defaults.

## Exchange Parameters

### Paper
- Initial balance: $100,000 USD
- Fee: 0.1% per trade
- Random walk price feed: drift=0.1, volatility=0.3, start=$50,000
- Instant order fills (no pending/expired states)

### Binance
- Symbol format: BTCUSDT (no separator)
- Spot trading only (ListPositions returns nil)
- SubscribeOrderBook: not yet implemented

### Coinbase
- Symbol format: BTC-USD
- SubscribeOrderBook: not yet implemented
