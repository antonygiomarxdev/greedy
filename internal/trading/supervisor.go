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
	"github.com/antonygiomarxdev/greedy/internal/markettracker"
	"github.com/antonygiomarxdev/greedy/internal/pricestreamer"
	"github.com/antonygiomarxdev/greedy/internal/shared"
)

type RestartPolicy int

const (
	RestartNever RestartPolicy = iota
	RestartAlways
	RestartOnError
)

type Supervisor struct {
	mu       sync.RWMutex
	bots     map[string]*Bot
	cancels  map[string]context.CancelFunc
	exchange shared.Exchange
	db       *sql.DB
	policy   RestartPolicy
	logger   *slog.Logger
	wg       sync.WaitGroup

	streamer    pricestreamer.PriceStreamer
	tracker     markettracker.MarketTracker
	idempotency idempotency.Store
}

func (s *Supervisor) SetStreamer(streamer pricestreamer.PriceStreamer) {
	s.streamer = streamer
}

func (s *Supervisor) SetTracker(tracker markettracker.MarketTracker) {
	s.tracker = tracker
}

func (s *Supervisor) SetIdempotency(store idempotency.Store) {
	s.idempotency = store
}

func NewSupervisor(ex shared.Exchange, database *sql.DB, policy RestartPolicy) *Supervisor {
	return &Supervisor{
		bots:     make(map[string]*Bot),
		cancels:  make(map[string]context.CancelFunc),
		exchange: ex,
		db:       database,
		policy:   policy,
		logger:   slog.Default().With("component", "supervisor"),
	}
}

type BotStatus struct {
	ID       string
	Name     string
	Strategy string
	Symbol   string
	Status   Status
	Error    error
}

func (s *Supervisor) StartBot(ctx context.Context, id string, cfg config.BotConfig, strat Strategy) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.bots[id]; exists {
		return fmt.Errorf("bot %s already running", id)
	}

	botCtx, cancel := context.WithCancel(ctx) // #nosec G118 — cancel is stored and called in Shutdown

	bot := New(id, cfg.Name, cfg, s.exchange, strat, s.db)
	bot.streamer = s.streamer
	bot.tracker = s.tracker
	bot.debouncer = debouncer.New(5*time.Second, 10, 30*time.Second)
	bot.idempotency = s.idempotency
	s.bots[id] = bot
	s.cancels[id] = cancel

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		bot.Run(botCtx)
	}()
	s.logger.Info("bot started", "id", id, "strategy", cfg.Strategy.Type)
	return nil
}

func (s *Supervisor) StopBot(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cancel, ok := s.cancels[id]
	if !ok {
		return fmt.Errorf("bot %s not found", id)
	}

	cancel()
	delete(s.cancels, id)
	delete(s.bots, id)
	s.logger.Info("bot stopped", "id", id)
	return nil
}

func (s *Supervisor) PauseBot(id string) error {
	s.mu.RLock()
	bot, ok := s.bots[id]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("bot %s not found", id)
	}
	bot.setStatus(StatusPaused)
	return nil
}

func (s *Supervisor) ResumeBot(id string) error {
	s.mu.RLock()
	bot, ok := s.bots[id]
	s.mu.RUnlock()
	if !ok {
		return fmt.Errorf("bot %s not found", id)
	}
	if bot.Status() == StatusPaused {
		bot.setStatus(StatusRunning)
	}
	return nil
}

func (s *Supervisor) ListBots() map[string]BotStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]BotStatus, len(s.bots))
	for id, bot := range s.bots {
		result[id] = BotStatus{
			ID:       bot.ID,
			Name:     bot.Name,
			Strategy: bot.Config.Strategy.Type,
			Symbol:   bot.Config.Strategy.Symbol,
			Status:   bot.Status(),
			Error:    bot.Error(),
		}
	}
	return result
}

func (s *Supervisor) Shutdown() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, cancel := range s.cancels {
		cancel()
		delete(s.cancels, id)
		delete(s.bots, id)
	}
	s.logger.Info("supervisor shutdown complete")
}

func (s *Supervisor) ShutdownCtx(ctx context.Context) error {
	s.mu.Lock()
	for id, cancel := range s.cancels {
		cancel()
		delete(s.cancels, id)
		delete(s.bots, id)
	}
	s.mu.Unlock()

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	timeout := 30 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	select {
	case <-done:
		s.logger.Info("supervisor shutdown complete")
		return nil
	case <-time.After(timeout):
		s.logger.Warn("supervisor shutdown timed out waiting for bots")
		return fmt.Errorf("shutdown timed out after %v", timeout)
	case <-ctx.Done():
		s.logger.Warn("supervisor shutdown cancelled")
		return ctx.Err()
	}
}
