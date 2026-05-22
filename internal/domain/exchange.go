package domain

import "context"

type Exchange interface {
	Name() string
	Ping(ctx context.Context) error
	GetOrderBook(ctx context.Context, symbol string, depth int) (*OrderBook, error)
	GetTicker(ctx context.Context, symbol string) (*Ticker, error)
	GetCandles(ctx context.Context, symbol string, interval CandleInterval, limit int) ([]Candle, error)
	SubscribeOrderBook(ctx context.Context, symbol string) (<-chan *OrderBookUpdate, error)
	PlaceOrder(ctx context.Context, req OrderRequest) (*Order, error)
	CancelOrder(ctx context.Context, orderID string) error
	GetOrder(ctx context.Context, orderID string) (*Order, error)
	ListOpenOrders(ctx context.Context, symbol string) ([]Order, error)
	GetBalance(ctx context.Context, asset string) (*Balance, error)
	ListBalances(ctx context.Context) ([]Balance, error)
	GetPosition(ctx context.Context, symbol string) (*Position, error)
	ListPositions(ctx context.Context) ([]Position, error)
}
