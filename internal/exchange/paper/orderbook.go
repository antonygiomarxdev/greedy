package paper

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/exchange"
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

	switch order.Type {
	case exchange.TypeMarket:
		return ob.matchMarketOrder(order), nil
	case exchange.TypeLimit:
		ob.addLimitOrder(order)
		return ob.matchLimitOrder(order), nil
	default:
		return nil, fmt.Errorf("unknown order type: %s", order.Type)
	}
}

func (ob *OrderBook) CancelOrder(orderID string) bool {
	ob.mu.Lock()
	defer ob.mu.Unlock()

	for i, bid := range ob.Bids {
		// Bids store client IDs in a synthetic field; we track them by price/quantity
		// In a real implementation, each level has order references
		_ = bid
		_ = i
	}
	return false
}

func (ob *OrderBook) matchMarketOrder(order *exchange.Order) []exchange.Fill {
	remaining := order.Quantity
	var fills []exchange.Fill

	if order.Side == exchange.SideBuy {
		for len(ob.Asks) > 0 && remaining > 0 {
			ask := ob.Asks[0]
			fillQty := min(remaining, ask.Quantity)
			fills = append(fills, exchange.Fill{
				OrderID:  order.ID,
				Quantity: fillQty,
				Price:    ask.Price,
			})
			remaining -= fillQty
			ask.Quantity -= fillQty
			if ask.Quantity <= 0 {
				ob.Asks = ob.Asks[1:]
			} else {
				ob.Asks[0] = ask
			}
		}
	} else {
		for len(ob.Bids) > 0 && remaining > 0 {
			bid := ob.Bids[0]
			fillQty := min(remaining, bid.Quantity)
			fills = append(fills, exchange.Fill{
				OrderID:  order.ID,
				Quantity: fillQty,
				Price:    bid.Price,
			})
			remaining -= fillQty
			bid.Quantity -= fillQty
			if bid.Quantity <= 0 {
				ob.Bids = ob.Bids[1:]
			} else {
				ob.Bids[0] = bid
			}
		}
	}

	order.FilledQuantity = order.Quantity - remaining
	if order.FilledQuantity > 0 {
		if order.FilledQuantity >= order.Quantity {
			order.Status = exchange.StatusFilled
		} else {
			order.Status = exchange.StatusPartiallyFilled
		}
	} else {
		order.Status = exchange.StatusRejected
	}

	return fills
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

func (ob *OrderBook) matchLimitOrder(order *exchange.Order) []exchange.Fill {
	var fills []exchange.Fill
	remaining := order.Quantity

	if order.Side == exchange.SideBuy {
		for len(ob.Asks) > 0 && ob.Asks[0].Price <= order.Price && remaining > 0 {
			ask := ob.Asks[0]
			fillQty := min(remaining, ask.Quantity)
			fills = append(fills, exchange.Fill{
				OrderID:  order.ID,
				Quantity: fillQty,
				Price:    ask.Price,
			})
			remaining -= fillQty
			ask.Quantity -= fillQty
			if ask.Quantity <= 0 {
				ob.Asks = ob.Asks[1:]
			} else {
				ob.Asks[0] = ask
			}
		}
	} else {
		for len(ob.Bids) > 0 && ob.Bids[0].Price >= order.Price && remaining > 0 {
			bid := ob.Bids[0]
			fillQty := min(remaining, bid.Quantity)
			fills = append(fills, exchange.Fill{
				OrderID:  order.ID,
				Quantity: fillQty,
				Price:    bid.Price,
			})
			remaining -= fillQty
			bid.Quantity -= fillQty
			if bid.Quantity <= 0 {
				ob.Bids = ob.Bids[1:]
			} else {
				ob.Bids[0] = bid
			}
		}
	}

	// Update remaining quantity on the books
	for i, bid := range ob.Bids {
		// Simple approach: just update order's filled quantity and status
		_ = bid
		_ = i
	}

	order.FilledQuantity = order.Quantity - remaining
	if order.FilledQuantity >= order.Quantity {
		order.Status = exchange.StatusFilled
	} else if order.FilledQuantity > 0 {
		order.Status = exchange.StatusPartiallyFilled
	}

	return fills
}
