package pricestore

import (
	"context"
	"time"
)

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
