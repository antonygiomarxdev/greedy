package paper

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/domain/exchange"
)

type OrderBook struct {
	mu   sync.RWMutex
	Bids []exchange.BookLevel
	Asks []exchange.BookLevel
	Time time.Time
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		Bids: make([]exchange.BookLevel, 0),
		Asks: make([]exchange.BookLevel, 0),
		Time: time.Now(),
	}
}

func (ob *OrderBook) Snapshot() *exchange.OrderBook {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	bids := make([]exchange.BookLevel, len(ob.Bids))
	copy(bids, ob.Bids)
	asks := make([]exchange.BookLevel, len(ob.Asks))
	copy(asks, ob.Asks)

	return &exchange.OrderBook{
		Bids: bids,
		Asks: asks,
		Time: ob.Time,
	}
}

func (ob *OrderBook) BestBid() float64 {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	if len(ob.Bids) == 0 {
		return 0
	}
	return ob.Bids[0].Price
}

func (ob *OrderBook) BestAsk() float64 {
	ob.mu.RLock()
	defer ob.mu.RUnlock()
	if len(ob.Asks) == 0 {
		return 0
	}
	return ob.Asks[0].Price
}

func (ob *OrderBook) PlaceOrder(order *exchange.Order) ([]exchange.Fill, error) {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	sideHandlers, ok := handlers[order.Type]
	if !ok {
		return nil, fmt.Errorf("unknown order type: %s", order.Type)
	}
	h, ok := sideHandlers[order.Side]
	if !ok {
		return nil, fmt.Errorf("unsupported side %s for type %s", order.Side, order.Type)
	}
	return h.place(ob, order), nil
}

func (ob *OrderBook) CancelOrder(orderID string) bool {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	for i, bid := range ob.Bids {
		_ = bid
		_ = i
	}
	return false
}

type matchPredicate func(level exchange.BookLevel, limitPrice float64) bool

func (ob *OrderBook) matchSide(side exchange.OrderSide, order *exchange.Order, predicate matchPredicate) []exchange.Fill {
	var fills []exchange.Fill
	remaining := order.Quantity

	levels := ob.oppositeLevels(side)
	for len(*levels) > 0 && remaining > 0 {
		level := (*levels)[0]
		if !predicate(level, order.Price) {
			break
		}
		fillQty := min(remaining, level.Quantity)
		fills = append(fills, exchange.Fill{
			OrderID:  order.ID,
			Quantity: fillQty,
			Price:    level.Price,
		})
		remaining -= fillQty
		level.Quantity -= fillQty
		if level.Quantity <= 0 {
			*levels = (*levels)[1:]
		} else {
			(*levels)[0] = level
		}
	}

	order.FilledQuantity = order.Quantity - remaining
	if order.FilledQuantity >= order.Quantity {
		order.Status = exchange.StatusFilled
	} else if order.FilledQuantity > 0 {
		order.Status = exchange.StatusPartiallyFilled
	} else if order.Type == exchange.TypeMarket {
		order.Status = exchange.StatusRejected
	}

	return fills
}

func (ob *OrderBook) oppositeLevels(side exchange.OrderSide) *[]exchange.BookLevel {
	if side == exchange.SideBuy {
		return &ob.Asks
	}
	return &ob.Bids
}

func (ob *OrderBook) addLimitOrder(order *exchange.Order) {
	level := exchange.BookLevel{Price: order.Price, Quantity: order.Quantity}
	if order.Side == exchange.SideBuy {
		ob.Bids = append(ob.Bids, level)
		sort.Slice(ob.Bids, func(i, j int) bool {
			return ob.Bids[i].Price > ob.Bids[j].Price
		})
	} else {
		ob.Asks = append(ob.Asks, level)
		sort.Slice(ob.Asks, func(i, j int) bool {
			return ob.Asks[i].Price < ob.Asks[j].Price
		})
	}
	order.Status = exchange.StatusOpen
}
