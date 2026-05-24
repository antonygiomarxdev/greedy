package paper

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/shared"
)

type PaperExchange struct {
	mu        sync.RWMutex
	books     map[string]*OrderBook
	feeds     map[string]*PriceFeed
	balances  map[string]float64
	orders    map[string]*shared.Order
	positions map[string]*shared.Position
	nextID    atomic.Uint64
	trades    []shared.Fill
	feeRate   float64
}

func New(feeRate float64) *PaperExchange {
	pe := &PaperExchange{
		books:     make(map[string]*OrderBook),
		feeds:     make(map[string]*PriceFeed),
		balances:  map[string]float64{"USD": 100000},
		orders:    make(map[string]*shared.Order),
		positions: make(map[string]*shared.Position),
		feeRate:   feeRate,
	}

	// Default BTC-USD market
	pe.books[shared.DefaultSymbol] = NewOrderBook()
	pe.feeds[shared.DefaultSymbol] = NewRandomWalkFeed(shared.DefaultSymbol, shared.DefaultBasePrice, shared.DefaultRandomWalkDrift, shared.DefaultRandomWalkVolatility, shared.DefaultTickInterval)

	return pe
}

func (pe *PaperExchange) Name() string { return string(shared.ProviderPaper) }

// AddMarket adds a new symbol with a price feed to the shared.
func (pe *PaperExchange) AddMarket(symbol string, feed *PriceFeed) {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	pe.books[symbol] = NewOrderBook()
	pe.feeds[symbol] = feed
}

func (pe *PaperExchange) SetPrice(symbol string, price float64) error {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	feed, ok := pe.feeds[symbol]
	if !ok {
		return shared.ErrSymbolNotFound
	}
	feed.SetPrice(price)
	return nil
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
		book.Bids = append(book.Bids, shared.BookLevel{Price: round2(mid - offset), Quantity: 1.0})
		book.Asks = append(book.Asks, shared.BookLevel{Price: round2(mid + offset), Quantity: 1.0})
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
		return nil, shared.ErrSymbolNotFound
	}
	return book, nil
}

func (pe *PaperExchange) getFeed(symbol string) (*PriceFeed, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()
	feed, ok := pe.feeds[symbol]
	if !ok {
		return nil, shared.ErrSymbolNotFound
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

func (pe *PaperExchange) GetOrderBook(ctx context.Context, symbol string, depth int) (*shared.OrderBook, error) {
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

func (pe *PaperExchange) GetTicker(ctx context.Context, symbol string) (*shared.Ticker, error) {
	feed, err := pe.getFeed(symbol)
	if err != nil {
		return nil, err
	}
	return &shared.Ticker{
		Symbol: symbol,
		Price:  feed.Price(),
		Time:   time.Now(),
	}, nil
}

func (pe *PaperExchange) GetCandles(ctx context.Context, symbol string, interval shared.CandleInterval, limit int) ([]shared.Candle, error) {
	feed, err := pe.getFeed(symbol)
	if err != nil {
		return nil, err
	}
	price := feed.Price()
	candle := shared.Candle{
		Symbol:   symbol,
		Interval: string(interval),
		OpenTime: time.Now(),
		Open:     price,
		High:     price,
		Low:      price,
		Close:    price,
		Volume:   0,
	}
	return []shared.Candle{candle}, nil
}

func (pe *PaperExchange) SubscribeOrderBook(ctx context.Context, symbol string) (<-chan *shared.OrderBookUpdate, error) {
	feed, err := pe.getFeed(symbol)
	if err != nil {
		return nil, err
	}
	_, priceCh := feed.Subscribe()

	updates := make(chan *shared.OrderBookUpdate, 16)
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
				book := &shared.OrderBookUpdate{
					Symbol: symbol,
					Bids: []shared.BookLevel{
						{Price: round2(price * 0.999), Quantity: 1.0},
						{Price: round2(price * 0.998), Quantity: 1.0},
					},
					Asks: []shared.BookLevel{
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

func (pe *PaperExchange) PlaceOrder(ctx context.Context, req shared.OrderRequest) (*shared.Order, error) {
	book, err := pe.getBook(req.Symbol)
	if err != nil {
		return nil, err
	}
	feed, err := pe.getFeed(req.Symbol)
	if err != nil {
		return nil, err
	}

	orderID := pe.nextOrderID()
	order := &shared.Order{
		ID:            orderID,
		ClientOrderID: req.ClientOrderID,
		Symbol:        req.Symbol,
		Side:          req.Side,
		Type:          req.Type,
		Price:         req.Price,
		Quantity:      req.Quantity,
		Status:        shared.StatusOpen,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if order.Type == shared.TypeMarket && order.Price == 0 {
		if order.Side == shared.SideBuy {
			order.Price = feed.Price() * 1.001
		} else {
			order.Price = feed.Price() * 0.999
		}
	}

	fills, err := book.PlaceOrder(order)
	if err != nil {
		return nil, err
	}

	pe.mu.Lock()
	pe.orders[orderID] = order
	pe.mu.Unlock()

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
		return shared.ErrOrderNotFound
	}
	order.Status = shared.StatusCancelled
	order.UpdatedAt = time.Now()
	return nil
}

func (pe *PaperExchange) GetOrder(ctx context.Context, orderID string) (*shared.Order, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	order, ok := pe.orders[orderID]
	if !ok {
		return nil, shared.ErrOrderNotFound
	}
	cp := *order
	return &cp, nil
}

func (pe *PaperExchange) ListOpenOrders(ctx context.Context, symbol string) ([]shared.Order, error) {
	pe.mu.RLock()
	ordersCopy := make([]shared.Order, 0, len(pe.orders))
	for _, o := range pe.orders {
		ordersCopy = append(ordersCopy, *o)
	}
	pe.mu.RUnlock()

	var orders []shared.Order
	for _, o := range ordersCopy {
		if o.Status == shared.StatusOpen || o.Status == shared.StatusPartiallyFilled {
			if symbol == "" || o.Symbol == symbol {
				orders = append(orders, o)
			}
		}
	}
	return orders, nil
}

func (pe *PaperExchange) GetBalance(ctx context.Context, asset string) (*shared.Balance, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	free, ok := pe.balances[asset]
	if !ok {
		free = 0
	}
	return &shared.Balance{
		Asset: asset,
		Free:  free,
		Total: free,
	}, nil
}

func (pe *PaperExchange) ListBalances(ctx context.Context) ([]shared.Balance, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	bals := make([]shared.Balance, 0, len(pe.balances))
	for asset, free := range pe.balances {
		bals = append(bals, shared.Balance{Asset: asset, Free: free, Total: free})
	}
	return bals, nil
}

func (pe *PaperExchange) GetPosition(ctx context.Context, symbol string) (*shared.Position, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	pos, ok := pe.positions[symbol]
	if !ok {
		return &shared.Position{Symbol: symbol}, nil
	}
	cp := *pos
	return &cp, nil
}

func (pe *PaperExchange) ListPositions(ctx context.Context) ([]shared.Position, error) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	pos := make([]shared.Position, 0, len(pe.positions))
	for _, p := range pe.positions {
		pos = append(pos, *p)
	}
	return pos, nil
}

func (pe *PaperExchange) updatePosition(symbol string, side shared.OrderSide, price, qty float64) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	pos, ok := pe.positions[symbol]
	if !ok {
		pos = &shared.Position{Symbol: symbol}
		pe.positions[symbol] = pos
	}

	if side == shared.SideBuy {
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

func (pe *PaperExchange) Snapshot() ([]shared.Balance, []shared.Position) {
	pe.mu.RLock()
	defer pe.mu.RUnlock()

	balances := make([]shared.Balance, 0, len(pe.balances))
	for asset, free := range pe.balances {
		balances = append(balances, shared.Balance{Asset: asset, Free: free, Total: free})
	}

	positions := make([]shared.Position, 0, len(pe.positions))
	for _, p := range pe.positions {
		positions = append(positions, *p)
	}

	return balances, positions
}

func (pe *PaperExchange) Restore(balances []shared.Balance, positions []shared.Position) {
	pe.mu.Lock()
	defer pe.mu.Unlock()

	for _, b := range balances {
		pe.balances[b.Asset] = b.Free
	}

	for _, p := range positions {
		cp := p
		pe.positions[p.Symbol] = &cp
	}
}
