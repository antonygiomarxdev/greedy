package paper

import "github.com/antonygiomarxdev/greedy/internal/domain/exchange"

type orderHandler interface {
	place(ob *OrderBook, order *exchange.Order) []exchange.Fill
}

type marketBuyHandler struct{}

func (h *marketBuyHandler) place(ob *OrderBook, order *exchange.Order) []exchange.Fill {
	return ob.matchSide(exchange.SideBuy, order, func(ask exchange.BookLevel, _ float64) bool { return true })
}

type marketSellHandler struct{}

func (h *marketSellHandler) place(ob *OrderBook, order *exchange.Order) []exchange.Fill {
	return ob.matchSide(exchange.SideSell, order, func(bid exchange.BookLevel, _ float64) bool { return true })
}

type limitBuyHandler struct{}

func (h *limitBuyHandler) place(ob *OrderBook, order *exchange.Order) []exchange.Fill {
	ob.addLimitOrder(order)
	return ob.matchSide(exchange.SideBuy, order, func(ask exchange.BookLevel, limit float64) bool { return ask.Price <= limit })
}

type limitSellHandler struct{}

func (h *limitSellHandler) place(ob *OrderBook, order *exchange.Order) []exchange.Fill {
	ob.addLimitOrder(order)
	return ob.matchSide(exchange.SideSell, order, func(bid exchange.BookLevel, limit float64) bool { return bid.Price >= limit })
}

var handlers = map[exchange.OrderType]map[exchange.OrderSide]orderHandler{
	exchange.TypeMarket: {
		exchange.SideBuy:  &marketBuyHandler{},
		exchange.SideSell: &marketSellHandler{},
	},
	exchange.TypeLimit: {
		exchange.SideBuy:  &limitBuyHandler{},
		exchange.SideSell: &limitSellHandler{},
	},
}
