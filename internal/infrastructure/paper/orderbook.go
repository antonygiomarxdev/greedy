package paper

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/shared"
)

type OrderBook struct {
	mu   sync.RWMutex
	Bids []shared.BookLevel
	Asks []shared.BookLevel
	Time time.Time
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		Bids: make([]shared.BookLevel, 0),
		Asks: make([]shared.BookLevel, 0),
		Time: time.Now(),
	}
}

func (ob *OrderBook) Snapshot() *shared.OrderBook {
	ob.mu.RLock()
	defer ob.mu.RUnlock()

	bids := make([]shared.BookLevel, len(ob.Bids))
	copy(bids, ob.Bids)
	asks := make([]shared.BookLevel, len(ob.Asks))
	copy(asks, ob.Asks)

	return &shared.OrderBook{
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

func (ob *OrderBook) PlaceOrder(order *shared.Order) ([]shared.Fill, error) {
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

type matchPredicate func(level shared.BookLevel, limitPrice float64) bool

func (ob *OrderBook) matchSide(side shared.OrderSide, order *shared.Order, predicate matchPredicate) []shared.Fill {
	var fills []shared.Fill
	remaining := order.Quantity

	levels := ob.oppositeLevels(side)
	for len(*levels) > 0 && remaining > 0 {
		level := (*levels)[0]
		if !predicate(level, order.Price) {
			break
		}
		fillQty := min(remaining, level.Quantity)
		fills = append(fills, shared.Fill{
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
		order.Status = shared.StatusFilled
	} else if order.FilledQuantity > 0 {
		order.Status = shared.StatusPartiallyFilled
	} else if order.Type == shared.TypeMarket {
		order.Status = shared.StatusRejected
	}

	return fills
}

func (ob *OrderBook) oppositeLevels(side shared.OrderSide) *[]shared.BookLevel {
	if side == shared.SideBuy {
		return &ob.Asks
	}
	return &ob.Bids
}

func (ob *OrderBook) addLimitOrder(order *shared.Order) {
	level := shared.BookLevel{Price: order.Price, Quantity: order.Quantity}
	if order.Side == shared.SideBuy {
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
	order.Status = shared.StatusOpen
}
