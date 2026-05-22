package paper

import "github.com/antonygiomarxdev/greedy/internal/shared"

type orderHandler interface {
	place(ob *OrderBook, order *shared.Order) []shared.Fill
}

type marketBuyHandler struct{}

func (h *marketBuyHandler) place(ob *OrderBook, order *shared.Order) []shared.Fill {
	return ob.matchSide(shared.SideBuy, order, func(ask shared.BookLevel, _ float64) bool { return true })
}

type marketSellHandler struct{}

func (h *marketSellHandler) place(ob *OrderBook, order *shared.Order) []shared.Fill {
	return ob.matchSide(shared.SideSell, order, func(bid shared.BookLevel, _ float64) bool { return true })
}

type limitBuyHandler struct{}

func (h *limitBuyHandler) place(ob *OrderBook, order *shared.Order) []shared.Fill {
	ob.addLimitOrder(order)
	return ob.matchSide(shared.SideBuy, order, func(ask shared.BookLevel, limit float64) bool { return ask.Price <= limit })
}

type limitSellHandler struct{}

func (h *limitSellHandler) place(ob *OrderBook, order *shared.Order) []shared.Fill {
	ob.addLimitOrder(order)
	return ob.matchSide(shared.SideSell, order, func(bid shared.BookLevel, limit float64) bool { return bid.Price >= limit })
}

var handlers = map[shared.OrderType]map[shared.OrderSide]orderHandler{
	shared.TypeMarket: {
		shared.SideBuy:  &marketBuyHandler{},
		shared.SideSell: &marketSellHandler{},
	},
	shared.TypeLimit: {
		shared.SideBuy:  &limitBuyHandler{},
		shared.SideSell: &limitSellHandler{},
	},
}
