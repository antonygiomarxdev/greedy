package strategy

import (
	"context"
	"testing"
	"time"

	bot "github.com/antonygiomarxdev/greedy/internal/domain/bot"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
	"github.com/antonygiomarxdev/greedy/internal/shared"
)

func TestSignal_EntryTrigger(t *testing.T) {
	cfg := config.SignalConfig{
		Symbol:       "BTC-USD",
		PositionSize: 1000,
	}
	s := NewSignal(cfg)

	state := &bot.BotState{
		Symbol: "BTC-USD",
		Ticker: &shared.Ticker{Price: 50000},
	}

	// No trigger yet — should hold
	signal, _ := s.Evaluate(context.Background(), state)
	if signal.Action != bot.ActionHold {
		t.Fatalf("expected hold without trigger, got %s", signal.Action)
	}

	// Trigger entry
	s.Trigger("entry")
	signal, _ = s.Evaluate(context.Background(), state)
	if signal.Action != bot.ActionBuy {
		t.Fatalf("expected buy after entry trigger, got %s", signal.Action)
	}
	if signal.Type != shared.TypeMarket {
		t.Fatalf("expected market order, got %s", signal.Type)
	}
	expectedQty := 1000.0 / 50000.0
	if signal.Quantity != expectedQty {
		t.Fatalf("expected qty %f, got %f", expectedQty, signal.Quantity)
	}
}

func TestSignal_ExitTrigger(t *testing.T) {
	cfg := config.SignalConfig{
		Symbol:       "BTC-USD",
		PositionSize: 1000,
	}
	s := NewSignal(cfg)

	state := &bot.BotState{
		Symbol: "BTC-USD",
		Ticker: &shared.Ticker{Price: 50000},
		Position: &shared.Position{
			Symbol:   "BTC-USD",
			Quantity: 0.02,
		},
	}

	// Trigger entry then exit
	s.Trigger("entry")
	s.Evaluate(context.Background(), state)

	s.Trigger("exit")
	signal, _ := s.Evaluate(context.Background(), state)
	if signal.Action != bot.ActionSell {
		t.Fatalf("expected sell after exit trigger, got %s", signal.Action)
	}
	if signal.Quantity != 0.02 {
		t.Fatalf("expected qty 0.02, got %f", signal.Quantity)
	}
}

func TestSignal_ExitWithoutPosition(t *testing.T) {
	s := NewSignal(config.SignalConfig{
		Symbol:       "BTC-USD",
		PositionSize: 1000,
	})

	state := &bot.BotState{
		Symbol: "BTC-USD",
		Ticker: &shared.Ticker{Price: 50000},
	}

	s.Trigger("exit")
	signal, _ := s.Evaluate(context.Background(), state)
	if signal.Action != bot.ActionHold {
		t.Fatalf("expected hold on exit without position, got %s", signal.Action)
	}
}

func TestSignal_DoubleEntryIgnored(t *testing.T) {
	s := NewSignal(config.SignalConfig{
		Symbol:       "BTC-USD",
		PositionSize: 1000,
	})
	state := &bot.BotState{
		Symbol: "BTC-USD",
		Ticker: &shared.Ticker{Price: 50000},
	}

	s.Trigger("entry")
	s.Evaluate(context.Background(), state)

	s.Trigger("entry")
	signal, _ := s.Evaluate(context.Background(), state)
	if signal.Action != bot.ActionHold {
		t.Fatalf("expected hold on double entry, got %s", signal.Action)
	}
}

func TestSignal_EntryWithZeroPrice(t *testing.T) {
	s := NewSignal(config.SignalConfig{
		Symbol:       "BTC-USD",
		PositionSize: 1000,
	})
	state := &bot.BotState{
		Symbol: "BTC-USD",
		Ticker: &shared.Ticker{Price: 0},
	}

	s.Trigger("entry")
	signal, err := s.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	if signal.Action != bot.ActionHold {
		t.Fatalf("expected hold on entry with zero price, got %s", signal.Action)
	}
}

func TestSignal_Reset(t *testing.T) {
	s := NewSignal(config.SignalConfig{
		Symbol:       "BTC-USD",
		PositionSize: 1000,
	})
	state := &bot.BotState{
		Symbol: "BTC-USD",
		Ticker: &shared.Ticker{Price: 50000},
	}

	s.Trigger("entry")
	s.Evaluate(context.Background(), state)

	s.Reset()

	// Should be able to enter again after reset
	s.Trigger("entry")
	signal, _ := s.Evaluate(context.Background(), state)
	if signal.Action != bot.ActionBuy {
		t.Fatalf("expected buy after reset + entry, got %s", signal.Action)
	}
}

func TestSignal_ResetDrainsQueue(t *testing.T) {
	s := NewSignal(config.SignalConfig{
		Symbol:       "BTC-USD",
		PositionSize: 1000,
	})

	// Queue multiple triggers
	for i := 0; i < 10; i++ {
		s.Trigger("entry")
	}

	s.Reset()

	// Should be empty after reset
	if len(s.signalCh) != 0 {
		t.Fatalf("expected empty signal queue after reset, got %d", len(s.signalCh))
	}
}

func TestSignal_UnknownTrigger(t *testing.T) {
	s := NewSignal(config.SignalConfig{
		Symbol:       "BTC-USD",
		PositionSize: 1000,
	})
	state := &bot.BotState{
		Symbol: "BTC-USD",
		Ticker: &shared.Ticker{Price: 50000},
	}

	s.Trigger("unknown_trigger")
	signal, _ := s.Evaluate(context.Background(), state)
	if signal.Action != bot.ActionHold {
		t.Fatalf("expected hold on unknown trigger, got %s", signal.Action)
	}
}

func TestSignal_TriggerChannelNonBlocking(t *testing.T) {
	s := NewSignal(config.SignalConfig{
		Symbol:       "BTC-USD",
		PositionSize: 1000,
	})

	// Fill the channel (capacity 16)
	for i := 0; i < 20; i++ {
		s.Trigger("entry")
	}

	// Channel should not block (overflow drops silently)
	if len(s.signalCh) != 16 {
		t.Fatalf("expected channel cap 16, got %d", len(s.signalCh))
	}
}

func TestSignal_ConcurrentTriggerSafe(t *testing.T) {
	s := NewSignal(config.SignalConfig{
		Symbol:       "BTC-USD",
		PositionSize: 1000,
	})

	done := make(chan bool, 4)
	for i := 0; i < 4; i++ {
		go func() {
			for j := 0; j < 25; j++ {
				s.Trigger("entry")
				s.Trigger("exit")
			}
			done <- true
		}()
	}

	// Wait for goroutines
	for i := 0; i < 4; i++ {
		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("concurrent trigger test timed out")
		}
	}

	// Should not panic
	s.Reset()
}
