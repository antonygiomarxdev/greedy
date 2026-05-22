package strategy

import (
	"context"
	"sync"

	"github.com/antonygiomarxdev/greedy/internal/domain/bot"
	"github.com/antonygiomarxdev/greedy/internal/domain/exchange"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
)

type triggerHandler interface {
	handle(s *Signal, state *bot.BotState) (*bot.Signal, error)
}

type entryTrigger struct{}

func (t *entryTrigger) handle(s *Signal, state *bot.BotState) (*bot.Signal, error) {
	if s.inPosition {
		return &bot.Signal{Action: bot.ActionHold}, nil
	}
	s.inPosition = true
	price := state.Ticker.Price
	if price <= 0 {
		return &bot.Signal{Action: bot.ActionHold}, nil
	}
	qty := s.cfg.PositionSize / price
	if qty <= 0 {
		return &bot.Signal{Action: bot.ActionHold}, nil
	}
	return &bot.Signal{
		Action:   bot.ActionBuy,
		Symbol:   state.Symbol,
		Quantity: qty,
		Type:     exchange.TypeMarket,
	}, nil
}

type exitTrigger struct{}

func (t *exitTrigger) handle(s *Signal, state *bot.BotState) (*bot.Signal, error) {
	if !s.inPosition {
		return &bot.Signal{Action: bot.ActionHold}, nil
	}
	s.inPosition = false
	if state.Position == nil || state.Position.Quantity <= 0 {
		return &bot.Signal{Action: bot.ActionHold}, nil
	}
	return &bot.Signal{
		Action:   bot.ActionSell,
		Symbol:   state.Symbol,
		Quantity: state.Position.Quantity,
		Type:     exchange.TypeMarket,
	}, nil
}

const (
	TriggerEntry = "entry"
	TriggerExit  = "exit"
)

var triggerHandlers = map[string]triggerHandler{
	TriggerEntry: &entryTrigger{},
	TriggerExit:  &exitTrigger{},
}

type Signal struct {
	cfg config.SignalConfig

	mu sync.Mutex

	entryActive bool
	inPosition  bool
	signalCh    chan string
}

func NewSignal(cfg config.SignalConfig) *Signal {
	return &Signal{
		cfg:      cfg,
		signalCh: make(chan string, 16),
	}
}

func (s *Signal) Name() string { return "signal" }

func (s *Signal) Trigger(signal string) {
	select {
	case s.signalCh <- signal:
	default:
	}
}

func (s *Signal) Evaluate(ctx context.Context, state *bot.BotState) (*bot.Signal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	select {
	case trigger := <-s.signalCh:
		h, ok := triggerHandlers[trigger]
		if !ok {
			return &bot.Signal{Action: bot.ActionHold}, nil
		}
		return h.handle(s, state)
	default:
	}

	return &bot.Signal{Action: bot.ActionHold}, nil
}

func (s *Signal) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inPosition = false
	s.entryActive = false
	for len(s.signalCh) > 0 {
		<-s.signalCh
	}
}
