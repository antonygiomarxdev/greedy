package exchange

import (
	"context"

	"github.com/antonygiomarxdev/greedy/internal/shared"
)

type Exchange interface {
	Name() string
	Ping(ctx context.Context) error
}

type MarketDataProvider interface {
	GetTicker(ctx context.Context, symbol string) (*shared.Ticker, error)
	GetOrderBook(ctx context.Context, symbol string, depth int) (*shared.OrderBook, error)
	GetCandles(ctx context.Context, symbol string, interval shared.CandleInterval, limit int) ([]shared.Candle, error)
}

type OrderExecutor interface {
	PlaceOrder(ctx context.Context, req shared.OrderRequest) (*shared.Order, error)
	CancelOrder(ctx context.Context, orderID string) error
	GetOrder(ctx context.Context, orderID string) (*shared.Order, error)
	ListOpenOrders(ctx context.Context, symbol string) ([]shared.Order, error)
}

type AccountProvider interface {
	GetBalance(ctx context.Context, asset string) (*shared.Balance, error)
	ListBalances(ctx context.Context) ([]shared.Balance, error)
	GetPosition(ctx context.Context, symbol string) (*shared.Position, error)
	ListPositions(ctx context.Context) ([]shared.Position, error)
}

type StreamProvider interface {
	SubscribeOrderBook(ctx context.Context, symbol string) (<-chan *shared.OrderBookUpdate, error)
}
