package idempotency

import "context"

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
