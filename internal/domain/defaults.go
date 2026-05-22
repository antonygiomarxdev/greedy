package domain

import (
	"context"
	"time"
)

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

type MarketLifecycleManager interface {
	AddMarket(symbol string, feed interface{})
	SeedLiquidity(symbol string, levels int, depth float64)
	StartFeeds(ctx context.Context)
}
