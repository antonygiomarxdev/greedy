package strategy

import (
	"context"
	"testing"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/bot"
	"github.com/antonygiomarxdev/greedy/internal/config"
	"github.com/antonygiomarxdev/greedy/internal/domain/exchange"
)

func TestDCA_FirstBuy(t *testing.T) {
	cfg := config.DCAConfig{
		Symbol:        "BTC-USD",
		BaseOrderSize: 100,
		Frequency:     1 * time.Second,
		SafetyOrders: []config.SafetyOrder{
			{PriceDeviationPct: -5, VolumeScale: 1.5},
		},
		MaxSafetyOrders: 10,
	}

	dca := NewDCA(cfg)

	state := &bot.BotState{
		Symbol: "BTC-USD",
		Ticker: &exchange.Ticker{Price: 50000},
	}

	signal, err := dca.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	if signal.Action != bot.ActionBuy {
		t.Fatalf("expected buy on first tick, got %s", signal.Action)
	}
}

func TestDCA_HoldBeforeFrequency(t *testing.T) {
	cfg := config.DCAConfig{
		Symbol:        "BTC-USD",
		BaseOrderSize: 100,
		Frequency:     1 * time.Hour,
		SafetyOrders: []config.SafetyOrder{
			{PriceDeviationPct: -5, VolumeScale: 1.5},
		},
		MaxSafetyOrders: 10,
	}

	dca := NewDCA(cfg)

	state := &bot.BotState{
		Symbol: "BTC-USD",
		Ticker: &exchange.Ticker{Price: 50000},
	}

	// First call: buy (no previous buy)
	signal, _ := dca.Evaluate(context.Background(), state)
	if signal.Action != bot.ActionBuy {
		t.Fatalf("expected buy on first call, got %s", signal.Action)
	}

	// Second call immediately after: should hold (within frequency window)
	signal, _ = dca.Evaluate(context.Background(), state)
	if signal.Action != bot.ActionHold {
		t.Fatalf("expected hold within frequency window, got %s", signal.Action)
	}
}

func TestDCA_SafetyOrderTriggered(t *testing.T) {
	cfg := config.DCAConfig{
		Symbol:        "BTC-USD",
		BaseOrderSize: 100,
		Frequency:     1 * time.Second,
		SafetyOrders: []config.SafetyOrder{
			{PriceDeviationPct: -5, VolumeScale: 1.5},
		},
		MaxSafetyOrders: 10,
	}

	dca := NewDCA(cfg)

	// First tick establishes initial price
	state1 := &bot.BotState{
		Symbol: "BTC-USD",
		Ticker: &exchange.Ticker{Price: 50000},
	}
	signal, _ := dca.Evaluate(context.Background(), state1)
	if signal.Action != bot.ActionBuy {
		t.Fatalf("expected first buy, got %s", signal.Action)
	}

	// Price drops >5% — safety order should trigger
	stateDrop := &bot.BotState{
		Symbol: "BTC-USD",
		Ticker: &exchange.Ticker{Price: 47000}, // -6%
	}
	signal, _ = dca.Evaluate(context.Background(), stateDrop)
	if signal.Action != bot.ActionBuy {
		t.Fatalf("expected safety order on drop, got %s", signal.Action)
	}
	// Safety order should have scaled quantity
	baseQty := 100.0 / 50000.0           // ~0.002
	safetyQty := (100.0 * 1.5) / 47000.0 // ~0.00319
	if signal.Quantity < baseQty {
		t.Fatalf("expected scaled quantity > base, got %f", signal.Quantity)
	}
	_ = safetyQty
}

func TestDCA_Reset(t *testing.T) {
	cfg := config.DCAConfig{
		Symbol:        "BTC-USD",
		BaseOrderSize: 100,
		Frequency:     1 * time.Hour,
		SafetyOrders: []config.SafetyOrder{
			{PriceDeviationPct: -5, VolumeScale: 1.5},
		},
		MaxSafetyOrders: 10,
	}

	dca := NewDCA(cfg)

	state := &bot.BotState{
		Symbol: "BTC-USD",
		Ticker: &exchange.Ticker{Price: 50000},
	}
	dca.Evaluate(context.Background(), state)

	dca.Reset()

	// After reset, should buy again immediately
	signal, _ := dca.Evaluate(context.Background(), state)
	if signal.Action != bot.ActionBuy {
		t.Fatalf("expected buy after reset, got %s", signal.Action)
	}
}

func TestDCA_ZeroPrice(t *testing.T) {
	cfg := config.DefaultDCAConfig()
	cfg.Frequency = 1 * time.Second
	dca := NewDCA(cfg)

	state := &bot.BotState{
		Symbol: "BTC-USD",
		Ticker: &exchange.Ticker{Price: 0},
	}

	signal, err := dca.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	if signal.Action != bot.ActionHold {
		t.Fatalf("expected hold on zero price, got %s", signal.Action)
	}
}
