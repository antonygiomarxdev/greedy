package bot

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sync"

	"github.com/antonygiomarxdev/greedy/internal/config"
	"github.com/antonygiomarxdev/greedy/internal/domain/exchange"
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
	exchange exchange.Exchange
	db       *sql.DB
	policy   RestartPolicy
	logger   *slog.Logger
}

func NewSupervisor(ex exchange.Exchange, database *sql.DB, policy RestartPolicy) *Supervisor {
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
	s.bots[id] = bot
	s.cancels[id] = cancel

	go bot.Run(botCtx)
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
