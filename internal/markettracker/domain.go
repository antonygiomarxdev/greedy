package markettracker

import (
	"context"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/pricestore"
)

type BreakerConfig struct {
	MaxPriceDeltaPct float64
	WindowDuration   time.Duration
	CooldownDuration time.Duration
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
	Restore(ctx context.Context, symbols []string, store pricestore.PriceStore) error
}
