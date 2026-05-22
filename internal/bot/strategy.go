package bot

import (
	"context"

	"github.com/antonygiomarxdev/greedy/internal/exchange"
)

type Action string

const (
	ActionBuy  Action = "buy"
	ActionSell Action = "sell"
	ActionHold Action = "hold"
)

type Signal struct {
	Action   Action
	Symbol   string
	Quantity float64
	Price    float64  // 0 = market order
	Type     exchange.OrderType
}

type BotState struct {
	Symbol    string
	Position  *exchange.Position
	Balance   *exchange.Balance
	Ticker    *exchange.Ticker
	OpenOrders []exchange.Order
}

type Strategy interface {
	Name() string
	Evaluate(ctx context.Context, state *BotState) (*Signal, error)
	Reset()
}
