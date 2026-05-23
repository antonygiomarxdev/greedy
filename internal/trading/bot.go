package trading

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/debouncer"
	"github.com/antonygiomarxdev/greedy/internal/idempotency"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/db"
	"github.com/antonygiomarxdev/greedy/internal/markettracker"
	"github.com/antonygiomarxdev/greedy/internal/pricestreamer"
	"github.com/antonygiomarxdev/greedy/internal/shared"
)

type Status string

const (
	StatusStopped  Status = "stopped"
	StatusStarting Status = "starting"
	StatusRunning  Status = "running"
	StatusPaused   Status = "paused"
	StatusError    Status = "error"
	StatusStopping Status = "stopping"
)

type Bot struct {
	ID        string
	Name      string
	Config    config.BotConfig
	Exchange  shared.Exchange
	Strategy  Strategy
	DB        *sql.DB
	repo      *db.BotRepository
	orderRepo *db.OrderRepository

	streamer    pricestreamer.PriceStreamer
	tracker     markettracker.MarketTracker
	debouncer   debouncer.Debouncer
	idempotency idempotency.Store
	seq         uint64

	mu     sync.RWMutex
	status Status
	err    error
	logger *slog.Logger
}

func New(id, name string, cfg config.BotConfig, ex shared.Exchange, strat Strategy, database *sql.DB) *Bot {
	b := &Bot{
		ID:       id,
		Name:     name,
		Config:   cfg,
		Exchange: ex,
		Strategy: strat,
		DB:       database,
		status:   StatusStopped,
		logger:   slog.Default().With("bot", id),
	}
	if database != nil {
		b.repo = db.NewBotRepository(database)
		b.orderRepo = db.NewOrderRepository(database)
	}
	return b
}

func (b *Bot) Status() Status {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.status
}

func (b *Bot) Error() error {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.err
}

func (b *Bot) setStatus(s Status) {
	b.mu.Lock()
	b.status = s
	b.mu.Unlock()
}

func (b *Bot) Run(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			b.logger.Error("bot panicked", "panic", r)
			b.setStatus(StatusError)
		}
	}()

	// Transition: STOPPED → STARTING
	b.setStatus(StatusStarting)
	b.logger.Info("bot starting")

	// Persist bot record (optional if DB is nil)
	if b.repo != nil {
		err := b.repo.Insert(db.BotRecord{
			ID:         b.ID,
			Name:       b.Name,
			Strategy:   b.Config.Strategy.Type,
			Symbol:     b.Config.Strategy.Symbol,
			ConfigYAML: "loaded",
			Status:     string(StatusStarting),
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		})
		if err != nil {
			b.logger.Warn("failed to persist bot", "error", err)
		}
	}

	// Reconcile with exchange
	if err := b.reconcile(ctx); err != nil {
		b.logger.Error("reconcile failed", "error", err)
		b.setStatus(StatusError)
		b.mu.Lock()
		b.err = fmt.Errorf("reconcile: %w", err)
		b.mu.Unlock()
		return
	}

	// Transition: STARTING → RUNNING
	b.setStatus(StatusRunning)
	if b.repo != nil {
		if err := b.repo.UpdateStatus(b.ID, string(StatusRunning)); err != nil {
			b.logger.Warn("failed to update bot status", "error", err)
		}
	}
	b.logger.Info("bot running")

	// Main loop
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			b.logger.Info("bot stopping")
			b.setStatus(StatusStopping)
			b.cancelAllOrders()
			if b.repo != nil {
				if err := b.repo.UpdateStatus(b.ID, string(StatusStopped)); err != nil {
					b.logger.Warn("failed to update bot status on stop", "error", err)
				}
			}
			b.setStatus(StatusStopped)
			return

		case <-ticker.C:
			status := b.Status()
			if status != StatusRunning {
				continue
			}
			if err := b.tick(ctx); err != nil {
				b.logger.Error("tick error", "error", err)
				b.mu.Lock()
				b.err = err
				b.mu.Unlock()
			}
		}
	}
}

func (b *Bot) tick(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var price float64
	var ts time.Time

	if b.streamer != nil {
		cached, ok := b.streamer.GetCached(b.Config.Strategy.Symbol)
		if !ok {
			return fmt.Errorf("symbol %s not registered in streamer", b.Config.Strategy.Symbol)
		}
		if cached.Stale {
			return fmt.Errorf("stale price for %s", b.Config.Strategy.Symbol)
		}
		price = cached.Price
		ts = cached.Timestamp
	} else {
		ticker, err := b.Exchange.GetTicker(ctx, b.Config.Strategy.Symbol)
		if err != nil {
			return fmt.Errorf("get ticker: %w", err)
		}
		price = ticker.Price
		ts = ticker.Time
	}

	if b.tracker != nil && b.tracker.IsBreakerActive(b.Config.Strategy.Symbol) {
		return nil
	}

	position, err := b.Exchange.GetPosition(ctx, b.Config.Strategy.Symbol)
	if err != nil {
		return fmt.Errorf("get position: %w", err)
	}

	balance, err := b.Exchange.GetBalance(ctx, "USD")
	if err != nil {
		return fmt.Errorf("get balance: %w", err)
	}

	openOrders, err := b.Exchange.ListOpenOrders(ctx, b.Config.Strategy.Symbol)
	if err != nil {
		return fmt.Errorf("list open orders: %w", err)
	}

	ticker := &shared.Ticker{
		Symbol: b.Config.Strategy.Symbol,
		Price:  price,
		Time:   ts,
	}

	state := &BotState{
		Symbol:     b.Config.Strategy.Symbol,
		Position:   position,
		Balance:    balance,
		Ticker:     ticker,
		OpenOrders: openOrders,
	}

	signal, err := b.Strategy.Evaluate(ctx, state)
	if err != nil {
		return fmt.Errorf("evaluate strategy: %w", err)
	}

	if signal.Action == ActionHold {
		return nil
	}

	if b.debouncer != nil && !b.debouncer.CanExecute() {
		return nil
	}

	req := shared.OrderRequest{
		Symbol:   signal.Symbol,
		Side:     shared.SideBuy,
		Type:     signal.Type,
		Quantity: signal.Quantity,
		Price:    signal.Price,
	}
	if signal.Action == ActionSell {
		req.Side = shared.SideSell
	}

	b.seq++
	req.ClientOrderID = fmt.Sprintf("%s-%d-%04d", b.ID, time.Now().UnixMilli(), b.seq)

	if b.idempotency != nil {
		if err := b.idempotency.Reserve(ctx, req.ClientOrderID, b.ID, b.Config.Strategy.Symbol); err != nil {
			return fmt.Errorf("idempotency reserve: %w", err)
		}
	}

	order, err := b.Exchange.PlaceOrder(ctx, req)
	if err != nil {
		return fmt.Errorf("place order: %w", err)
	}

	if b.debouncer != nil {
		b.debouncer.RecordExecution()
	}

	b.logger.Info("order placed",
		"order_id", order.ID,
		"side", order.Side,
		"qty", order.Quantity,
		"price", order.Price,
		"status", order.Status,
	)

	if b.orderRepo != nil {
		rec := db.OrderRecord{
			ID:              order.ID,
			BotID:           b.ID,
			ExchangeOrderID: order.ClientOrderID,
			Symbol:          order.Symbol,
			Side:            string(order.Side),
			Type:            string(order.Type),
			Price:           order.Price,
			Quantity:        order.Quantity,
			FilledQuantity:  order.FilledQuantity,
			Status:          string(order.Status),
			CreatedAt:       order.CreatedAt,
			UpdatedAt:       order.UpdatedAt,
		}
		if err := b.orderRepo.Insert(rec); err != nil {
			b.logger.Warn("failed to persist order", "order_id", order.ID, "error", err)
		}
	}

	NotifyOrderConfirmer(b.Strategy, signal.Price, order.ID)
	if order.Status == shared.StatusFilled || order.Status == shared.StatusPartiallyFilled {
		NotifyOrderFilled(b.Strategy, signal.Price)
		if b.idempotency != nil {
			if err := b.idempotency.Confirm(ctx, req.ClientOrderID, order.ID); err != nil {
				b.logger.Warn("idempotency confirm failed", "clientOrderID", req.ClientOrderID, "error", err)
			}
		}
	}

	return nil
}

func (b *Bot) reconcile(ctx context.Context) error {
	// Ensure exchange is reachable
	if err := b.Exchange.Ping(ctx); err != nil {
		return fmt.Errorf("exchange ping: %w", err)
	}

	// Fetch open orders from exchange — this is the source of truth
	openOrders, err := b.Exchange.ListOpenOrders(ctx, b.Config.Strategy.Symbol)
	if err != nil {
		return fmt.Errorf("list open orders: %w", err)
	}
	b.logger.Info("reconciled open orders", "count", len(openOrders))

	return nil
}

func (b *Bot) cancelAllOrders() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	orders, err := b.Exchange.ListOpenOrders(ctx, b.Config.Strategy.Symbol)
	if err != nil {
		b.logger.Warn("failed to list orders for cancellation", "error", err)
		return
	}

	for _, o := range orders {
		if err := b.Exchange.CancelOrder(ctx, o.ID); err != nil {
			b.logger.Warn("failed to cancel order", "order_id", o.ID, "error", err)
		}
	}
}
