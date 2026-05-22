package paper

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/exchange"
)

type PaperExchange struct {
	mu        sync.RWMutex
	books     map[string]*OrderBook
	feeds     map[string]*PriceFeed
	balances  map[string]float64
	orders    map[string]*exchange.Order
	positions map[string]*exchange.Position
	nextID    atomic.Uint64
	trades    []exchange.Fill
	feeRate   float64
}

func New(feeRate float64) *PaperExchange {
	pe := &PaperExchange{
		books:     make(map[string]*OrderBook),
		feeds:     make(map[string]*PriceFeed),
		balances:  map[string]float64{"USD": 100000},
		orders:    make(map[string]*exchange.Order),
		positions: make(map[string]*exchange.Position),
		feeRate:   feeRate,
	}

	// Default BTC-USD market
	pe.books["BTC-USD"] = NewOrderBook()
	pe.feeds["BTC-USD"] = NewRandomWalkFeed("BTC-USD", 50000, 0.1, 0.3, 100*time.Millisecond)

	return pe
}

func (pe *PaperExchange) Name() string { return "paper" }

// AddMarket adds a new symbol with a price feed to the exchange.
func (pe *PaperExchange) AddMarket(symbol string, feed *PriceFeed) {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	pe.books[symbol] = NewOrderBook()
	pe.feeds[symbol] = feed
}

// SeedLiquidity populates the order book with synthetic liquidity around the current price.
func (pe *PaperExchange) SeedLiquidity(symbol string, levels int, spread float64) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	feed, ok := pe.feeds[symbol]
	if !ok {
		return
	}
	book, ok := pe.books[symbol]
	if !ok {
		return
	}

	mid := feed.Price()

	book.mu.Lock()
	defer book.mu.Unlock()

	for i := 0; i < levels; i++ {
		offset := spread * float64(i+1)
		book.Bids = append(book.Bids, exchange.BookLevel{Price: round2(mid - offset), Quantity: 1.0})
		book.Asks = append(book.Asks, exchange.BookLevel{Price: round2(mid + offset), Quantity: 1.0})
	}
}

func (pe *PaperExchange) nextOrderID() string {
	id := pe.nextID.Add(1)
	return fmt.Sprintf("paper-%d", id)
}

func (pe *PaperExchange) getBook(symbol string) (*OrderBook, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	book, ok := pe.books[symbol]
	if !ok {
		return nil, exchange.ErrSymbolNotFound
	}
	return book, nil
}

func (pe *PaperExchange) getFeed(symbol string) (*PriceFeed, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	feed, ok := pe.feeds[symbol]
	if !ok {
		return nil, exchange.ErrSymbolNotFound
	}
	return feed, nil
}

func (pe *PaperExchange) StartFeeds(ctx context.Context) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	for _, feed := range pe.feeds {
		go feed.Run(ctx)
	}
}

func (pe *PaperExchange) Ping(ctx context.Context) error { return nil }

func (pe *PaperExchange) GetOrderBook(ctx context.Context, symbol string, depth int) (*exchange.OrderBook, error) {
	book, err := pe.getBook(symbol)
	if err != nil {
		return nil, err
	}
	snap := book.Snapshot()
	if depth > 0 {
		if len(snap.Bids) > depth {
			snap.Bids = snap.Bids[:depth]
		}
		if len(snap.Asks) > depth {
			snap.Asks = snap.Asks[:depth]
		}
	}
	return snap, nil
}

func (pe *PaperExchange) GetTicker(ctx context.Context, symbol string) (*exchange.Ticker, error) {
	feed, err := pe.getFeed(symbol)
	if err != nil {
		return nil, err
	}
	return &exchange.Ticker{
		Symbol: symbol,
		Price:  feed.Price(),
		Time:   time.Now(),
	}, nil
}

func (pe *PaperExchange) GetCandles(ctx context.Context, symbol string, interval exchange.CandleInterval, limit int) ([]exchange.Candle, error) {
	feed, err := pe.getFeed(symbol)
	if err != nil {
		return nil, err
	}
	price := feed.Price()
	candle := exchange.Candle{
		Symbol:   symbol,
		Interval: string(interval),
		OpenTime: time.Now(),
		Open:     price,
		High:     price,
		Low:      price,
		Close:    price,
		Volume:   0,
	}
	return []exchange.Candle{candle}, nil
}

func (pe *PaperExchange) SubscribeOrderBook(ctx context.Context, symbol string) (<-chan *exchange.OrderBookUpdate, error) {
	feed, err := pe.getFeed(symbol)
	if err != nil {
		return nil, err
	}
	_, priceCh := feed.Subscribe()

	updates := make(chan *exchange.OrderBookUpdate, 16)
	go func() {
		defer close(updates)
		for {
			select {
			case <-ctx.Done():
				return
			case price, ok := <-priceCh:
				if !ok {
					return
				}
				// Rebuild book around new price
				book := &exchange.OrderBookUpdate{
					Symbol: symbol,
					Bids: []exchange.BookLevel{
						{Price: round2(price * 0.999), Quantity: 1.0},
						{Price: round2(price * 0.998), Quantity: 1.0},
					},
					Asks: []exchange.BookLevel{
						{Price: round2(price * 1.001), Quantity: 1.0},
						{Price: round2(price * 1.002), Quantity: 1.0},
					},
				}
				select {
				case updates <- book:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	return updates, nil
}

func (pe *PaperExchange) PlaceOrder(ctx context.Context, req exchange.OrderRequest) (*exchange.Order, error) {
	book, err := pe.getBook(req.Symbol)
	if err != nil {
		return nil, err
	}
	feed, err := pe.getFeed(req.Symbol)
	if err != nil {
		return nil, err
	}

	orderID := pe.nextOrderID()
	order := &exchange.Order{
		ID:            orderID,
		ClientOrderID: req.ClientOrderID,
		Symbol:        req.Symbol,
		Side:          req.Side,
		Type:          req.Type,
		Price:         req.Price,
		Quantity:      req.Quantity,
		Status:        exchange.StatusOpen,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if order.Type == exchange.TypeMarket && order.Price == 0 {
		if order.Side == exchange.SideBuy {
			order.Price = feed.Price() * 1.001
		} else {
			order.Price = feed.Price() * 0.999
		}
	}

	pe.mu.Lock()
	pe.orders[orderID] = order
	pe.mu.Unlock()

	fills, err := book.PlaceOrder(order)
	if err != nil {
		return nil, err
	}

	if len(fills) > 0 {
		for _, f := range fills {
			pe.mu.Lock()
			pe.trades = append(pe.trades, f)
			pe.mu.Unlock()
			pe.updatePosition(req.Symbol, req.Side, f.Price, f.Quantity)
		}
	}

	return order, nil
}

func (pe *PaperExchange) CancelOrder(ctx context.Context, orderID string) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	order, ok := pe.orders[orderID]
	if !ok {
		return exchange.ErrOrderNotFound
	}
	order.Status = exchange.StatusCancelled
	order.UpdatedAt = time.Now()
	return nil
}

func (pe *PaperExchange) GetOrder(ctx context.Context, orderID string) (*exchange.Order, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	order, ok := pe.orders[orderID]
	if !ok {
		return nil, exchange.ErrOrderNotFound
	}
	cp := *order
	return &cp, nil
}

func (pe *PaperExchange) ListOpenOrders(ctx context.Context, symbol string) ([]exchange.Order, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	var orders []exchange.Order
	for _, o := range pe.orders {
		if o.Status == exchange.StatusOpen || o.Status == exchange.StatusPartiallyFilled {
			if symbol == "" || o.Symbol == symbol {
				orders = append(orders, *o)
			}
		}
	}
	return orders, nil
}

func (pe *PaperExchange) GetBalance(ctx context.Context, asset string) (*exchange.Balance, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	free, ok := pe.balances[asset]
	if !ok {
		free = 0
	}
	return &exchange.Balance{
		Asset: asset,
		Free:  free,
		Total: free,
	}, nil
}

func (pe *PaperExchange) ListBalances(ctx context.Context) ([]exchange.Balance, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	var bals []exchange.Balance
	for asset, free := range pe.balances {
		bals = append(bals, exchange.Balance{Asset: asset, Free: free, Total: free})
	}
	return bals, nil
}

func (pe *PaperExchange) GetPosition(ctx context.Context, symbol string) (*exchange.Position, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	pos, ok := pe.positions[symbol]
	if !ok {
		return &exchange.Position{Symbol: symbol}, nil
	}
	cp := *pos
	return &cp, nil
}

func (pe *PaperExchange) ListPositions(ctx context.Context) ([]exchange.Position, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	var pos []exchange.Position
	for _, p := range pe.positions {
		pos = append(pos, *p)
	}
	return pos, nil
}

func (pe *PaperExchange) updatePosition(symbol string, side exchange.OrderSide, price, qty float64) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	pos, ok := pe.positions[symbol]
	if !ok {
		pos = &exchange.Position{Symbol: symbol}
		pe.positions[symbol] = pos
	}

	if side == exchange.SideBuy {
		totalCost := pos.AvgEntryPrice*math.Abs(pos.Quantity) + price*qty
		newQty := pos.Quantity + qty
		if newQty != 0 {
			pos.AvgEntryPrice = totalCost / math.Abs(newQty)
		}
		pos.Quantity = newQty
		// Deduct from quote balance
		if bal, ok := pe.balances["USD"]; ok {
			pe.balances["USD"] = bal - price*qty*(1+pe.feeRate)
		}
	} else {
		if pos.Quantity >= qty {
			pos.RealizedPnL += (price - pos.AvgEntryPrice) * qty
			pos.Quantity -= qty
		} else {
			pos.RealizedPnL += (price - pos.AvgEntryPrice) * pos.Quantity
			pos.Quantity = 0
		}
		// Add to quote balance
		if bal, ok := pe.balances["USD"]; ok {
			pe.balances["USD"] = bal + price*qty*(1-pe.feeRate)
		}
	}
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
