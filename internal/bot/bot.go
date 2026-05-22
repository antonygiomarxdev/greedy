package bot

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/config"
	"github.com/antonygiomarxdev/greedy/internal/db"
	"github.com/antonygiomarxdev/greedy/internal/exchange"
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
	ID       string
	Name     string
	Config   config.BotConfig
	Exchange exchange.Exchange
	Strategy Strategy
	DB       *sql.DB // for persistence
	repo     *db.BotRepository

	mu     sync.RWMutex
	status Status
	err    error
	logger *slog.Logger
}

func New(id, name string, cfg config.BotConfig, ex exchange.Exchange, strat Strategy, database *sql.DB) *Bot {
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

	ticker, err := b.Exchange.GetTicker(ctx, b.Config.Strategy.Symbol)
	if err != nil {
		return fmt.Errorf("get ticker: %w", err)
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

	req := exchange.OrderRequest{
		ClientOrderID: fmt.Sprintf("%s-%d", b.ID, time.Now().UnixNano()),
		Symbol:        signal.Symbol,
		Side:          exchange.SideBuy,
		Type:          signal.Type,
		Quantity:      signal.Quantity,
		Price:         signal.Price,
	}
	if signal.Action == ActionSell {
		req.Side = exchange.SideSell
	}

	order, err := b.Exchange.PlaceOrder(ctx, req)
	if err != nil {
		return fmt.Errorf("place order: %w", err)
	}

	b.logger.Info("order placed",
		"order_id", order.ID,
		"side", order.Side,
		"qty", order.Quantity,
		"price", order.Price,
		"status", order.Status,
	)

	// Notify strategy of order confirmation (GRID needs this)
	if confirmer, ok := b.Strategy.(interface{ ConfirmOrder(float64, string) }); ok {
		confirmer.ConfirmOrder(signal.Price, order.ID)
	}

	// If filled immediately, notify strategy
	if order.Status == exchange.StatusFilled || order.Status == exchange.StatusPartiallyFilled {
		if filler, ok := b.Strategy.(interface{ OrderFilled(float64) }); ok {
			filler.OrderFilled(signal.Price)
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
