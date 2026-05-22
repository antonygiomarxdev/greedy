package shared

import "errors"

var (
	ErrRateLimited       = errors.New("exchange rate limited")
	ErrAuthFailed        = errors.New("authentication failed")
	ErrInsufficientFunds = errors.New("insufficient funds")
	ErrOrderNotFound     = errors.New("order not found")
	ErrInvalidOrder      = errors.New("invalid order parameters")
	ErrSymbolNotFound    = errors.New("symbol not found")
	ErrExchangeDown      = errors.New("exchange unavailable")
)
