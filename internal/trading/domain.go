package trading

import (
	"context"

	"github.com/antonygiomarxdev/greedy/internal/shared"
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
	Price    float64
	Type     shared.OrderType
}

type BotState struct {
	Symbol     string
	Position   *shared.Position
	Balance    *shared.Balance
	Ticker     *shared.Ticker
	OpenOrders []shared.Order
}

type Strategy interface {
	Name() string
	Evaluate(ctx context.Context, state *BotState) (*Signal, error)
	Reset()
}

type OrderConfirmer interface {
	ConfirmOrder(price float64, orderID string)
}

type OrderFilledListener interface {
	OrderFilled(price float64)
}

func NotifyOrderConfirmer(strat Strategy, price float64, orderID string) {
	if c, ok := strat.(OrderConfirmer); ok {
		c.ConfirmOrder(price, orderID)
	}
}

func NotifyOrderFilled(strat Strategy, price float64) {
	if f, ok := strat.(OrderFilledListener); ok {
		f.OrderFilled(price)
	}
}
