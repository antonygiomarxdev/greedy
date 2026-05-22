package exchange

import "time"

const (
	DefaultFeeRate         = 0.001
	DefaultBasePrice       = 50_000.0
	DefaultTickInterval    = 100 * time.Millisecond
	DefaultLiquidityLevels = 10
	DefaultLiquidityDepth  = 100

	DefaultRandomWalkDrift      = 0.1
	DefaultRandomWalkVolatility = 0.3

	DefaultSymbol = "BTC-USD"
	DefaultQuote  = "USD"
)
