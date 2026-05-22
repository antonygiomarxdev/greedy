package pricestreamer

import (
	"context"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/pricestore"
)

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
	ActiveSymbols() []string

	SetPriceStore(store pricestore.PriceStore)
	OnTick(fn func(symbol string, price float64, ts time.Time))
}
