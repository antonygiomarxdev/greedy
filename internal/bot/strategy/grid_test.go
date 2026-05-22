package strategy

import (
	"context"
	"fmt"
	"testing"

	"github.com/antonygiomarxdev/greedy/internal/bot"
	"github.com/antonygiomarxdev/greedy/internal/config"
	"github.com/antonygiomarxdev/greedy/internal/exchange"
)

func TestGRID_FirstTickBuildsGrid(t *testing.T) {
	cfg := config.GridConfig{
		Symbol:     "ETH-USD",
		LowerBound: 2000,
		UpperBound: 3000,
		GridLevels: 5,
		OrderSize:  500,
	}
	g := NewGRID(cfg)

	state := &bot.BotState{
		Symbol:     "ETH-USD",
		Ticker:     &exchange.Ticker{Price: 2500},
		OpenOrders: []exchange.Order{},
	}

	// Place all 5 grid levels
	for i := 0; i < 5; i++ {
		signal, err := g.Evaluate(context.Background(), state)
		if err != nil {
			t.Fatal(err)
		}
		if signal.Action == bot.ActionHold {
			t.Fatalf("expected order at tick %d, got hold", i)
		}
		if signal.Type != exchange.TypeLimit {
			t.Fatalf("expected limit order at tick %d, got %s", i, signal.Type)
		}
		// Simulate order confirmation
		g.ConfirmOrder(signal.Price, fmt.Sprintf("order-%d", i))
	}

	// After all levels placed, should hold
	signal, _ := g.Evaluate(context.Background(), state)
	if signal.Action != bot.ActionHold {
		t.Fatalf("expected hold after grid complete, got %s", signal.Action)
	}
}

func TestGRID_BuildsCorrectLevels(t *testing.T) {
	cfg := config.GridConfig{
		LowerBound: 100,
		UpperBound: 200,
		GridLevels: 3,
		OrderSize:  100,
	}
	g := NewGRID(cfg)
	g.buildGrid(150)

	if len(g.gridLevels) != 3 {
		t.Fatalf("expected 3 grid levels, got %d", len(g.gridLevels))
	}
	if g.gridLevels[0] != 100.0 {
		t.Fatalf("expected first level 100, got %f", g.gridLevels[0])
	}
	if g.gridLevels[1] != 150.0 {
		t.Fatalf("expected middle level 150, got %f", g.gridLevels[1])
	}
	if g.gridLevels[2] != 200.0 {
		t.Fatalf("expected last level 200, got %f", g.gridLevels[2])
	}
}

func TestGRID_ReplenishAfterFill(t *testing.T) {
	cfg := config.GridConfig{
		Symbol:     "ETH-USD",
		LowerBound: 100,
		UpperBound: 200,
		GridLevels: 2,
		OrderSize:  100,
	}
	g := NewGRID(cfg)

	state := &bot.BotState{
		Symbol:     "ETH-USD",
		Ticker:     &exchange.Ticker{Price: 150},
		OpenOrders: []exchange.Order{},
	}

	// Place both levels
	sig1, _ := g.Evaluate(context.Background(), state)
	g.ConfirmOrder(sig1.Price, "order-1")
	sig2, _ := g.Evaluate(context.Background(), state)
	g.ConfirmOrder(sig2.Price, "order-2")

	// Both placed
	sig3, _ := g.Evaluate(context.Background(), state)
	if sig3.Action != bot.ActionHold {
		t.Fatalf("expected hold after both placed, got %s", sig3.Action)
	}

	// Simulate order at price 100 filled
	g.OrderFilled(100)

	// Should replenish the missing level
	sig4, _ := g.Evaluate(context.Background(), state)
	if sig4.Action != bot.ActionBuy || sig4.Type != exchange.TypeLimit {
		t.Fatalf("expected replenishment order, got action=%s type=%s", sig4.Action, sig4.Type)
	}
}

func TestGRID_ReplenishWithOpenOrders(t *testing.T) {
	cfg := config.GridConfig{
		Symbol:     "ETH-USD",
		LowerBound: 100,
		UpperBound: 200,
		GridLevels: 2,
		OrderSize:  100,
	}
	g := NewGRID(cfg)

	// Place both levels
	state := &bot.BotState{
		Symbol:     "ETH-USD",
		Ticker:     &exchange.Ticker{Price: 150},
		OpenOrders: []exchange.Order{},
	}
	sig1, _ := g.Evaluate(context.Background(), state)
	g.ConfirmOrder(sig1.Price, "order-1")
	sig2, _ := g.Evaluate(context.Background(), state)
	g.ConfirmOrder(sig2.Price, "order-2")

	// Simulate open orders from exchange perspective
	state.OpenOrders = []exchange.Order{
		{ID: "order-1", Price: sig1.Price, Status: exchange.StatusOpen},
		{ID: "order-2", Price: sig2.Price, Status: exchange.StatusOpen},
	}

	// All active on exchange — should hold
	sig3, _ := g.Evaluate(context.Background(), state)
	if sig3.Action != bot.ActionHold {
		t.Fatalf("expected hold when all orders active, got %s", sig3.Action)
	}
}

func TestGRID_ZeroPrice(t *testing.T) {
	cfg := config.DefaultGridConfig()
	cfg.Symbol = "ETH-USD"
	cfg.LowerBound = 1000
	cfg.UpperBound = 2000
	cfg.GridLevels = 5
	g := NewGRID(cfg)

	state := &bot.BotState{
		Symbol: "ETH-USD",
		Ticker: &exchange.Ticker{Price: 0},
	}

	signal, err := g.Evaluate(context.Background(), state)
	if err != nil {
		t.Fatal(err)
	}
	if signal.Action != bot.ActionHold {
		t.Fatalf("expected hold on zero price, got %s", signal.Action)
	}
}

func TestGRID_Reset(t *testing.T) {
	cfg := config.GridConfig{
		Symbol:     "ETH-USD",
		LowerBound: 1000,
		UpperBound: 2000,
		GridLevels: 5,
		OrderSize:  100,
	}
	g := NewGRID(cfg)

	state := &bot.BotState{
		Symbol:     "ETH-USD",
		Ticker:     &exchange.Ticker{Price: 1500},
		OpenOrders: []exchange.Order{},
	}
	g.Evaluate(context.Background(), state) // Build grid + first order

	g.Reset()
	if len(g.gridLevels) != 0 {
		t.Fatalf("expected empty gridLevels after reset, got %d", len(g.gridLevels))
	}
	if len(g.placedOrders) != 0 {
		t.Fatalf("expected empty placedOrders after reset, got %d", len(g.placedOrders))
	}
	if len(g.pendingOrders) != 0 {
		t.Fatalf("expected empty pendingOrders after reset, got %d", len(g.pendingOrders))
	}

	// Should rebuild grid after reset
	signal, _ := g.Evaluate(context.Background(), state)
	if signal.Type != exchange.TypeLimit {
		t.Fatalf("expected grid to rebuild after reset, got %s", signal.Action)
	}
}

func TestGRID_ManualPriceSet(t *testing.T) {
	cfg := config.GridConfig{
		Symbol:     "ETH-USD",
		LowerBound: 100,
		UpperBound: 500,
		GridLevels: 3,
		OrderSize:  300,
	}
	g := NewGRID(cfg)
	g.buildGrid(300)

	if len(g.gridLevels) != 3 {
		t.Fatalf("expected 3 levels, got %d", len(g.gridLevels))
	}
	if g.gridLevels[0] != 100.0 || g.gridLevels[2] != 500.0 {
		t.Fatalf("unexpected grid bounds: %v", g.gridLevels)
	}
}

func TestGRID_ConfirmOrderClearsPending(t *testing.T) {
	cfg := config.GridConfig{
		Symbol:     "ETH-USD",
		LowerBound: 100,
		UpperBound: 200,
		GridLevels: 2,
		OrderSize:  100,
	}
	g := NewGRID(cfg)

	state := &bot.BotState{
		Symbol:     "ETH-USD",
		Ticker:     &exchange.Ticker{Price: 150},
		OpenOrders: []exchange.Order{},
	}

	sig1, _ := g.Evaluate(context.Background(), state)
	if len(g.pendingOrders) != 1 {
		t.Fatalf("expected 1 pending after signal, got %d", len(g.pendingOrders))
	}

	g.ConfirmOrder(sig1.Price, "order-1")
	if len(g.pendingOrders) != 0 {
		t.Fatalf("expected 0 pending after confirm, got %d", len(g.pendingOrders))
	}
}

func TestGRID_OrderFilledClearsAll(t *testing.T) {
	cfg := config.GridConfig{
		Symbol:     "ETH-USD",
		LowerBound: 100,
		UpperBound: 200,
		GridLevels: 2,
		OrderSize:  100,
	}
	g := NewGRID(cfg)

	state := &bot.BotState{
		Symbol:     "ETH-USD",
		Ticker:     &exchange.Ticker{Price: 150},
		OpenOrders: []exchange.Order{},
	}

	sig1, _ := g.Evaluate(context.Background(), state)
	g.ConfirmOrder(sig1.Price, "order-1")

	// Should be in placedOrders
	if _, ok := g.placedOrders[sig1.Price]; !ok {
		t.Fatal("expected order in placedOrders after confirm")
	}

	g.OrderFilled(sig1.Price)
	if _, ok := g.placedOrders[sig1.Price]; ok {
		t.Fatal("expected order removed from placedOrders after fill")
	}
}
