package paper

import (
	"context"
	"testing"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/exchange"
)

func TestPaperExchange_New(t *testing.T) {
	pe := New(0.001)
	if pe.Name() != "paper" {
		t.Fatalf("expected name 'paper', got '%s'", pe.Name())
	}
}

func TestPaperExchange_Ping(t *testing.T) {
	pe := New(0.001)
	if err := pe.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestPaperExchange_GetTicker(t *testing.T) {
	pe := New(0.001)
	ticker, err := pe.GetTicker(context.Background(), "BTC-USD")
	if err != nil {
		t.Fatal(err)
	}
	if ticker.Price <= 0 {
		t.Fatal("expected positive price")
	}
	if ticker.Symbol != "BTC-USD" {
		t.Fatalf("expected symbol BTC-USD, got %s", ticker.Symbol)
	}
}

func TestPaperExchange_PlaceMarketOrder(t *testing.T) {
	pe := New(0.001)
	pe.SeedLiquidity("BTC-USD", 10, 100)
	ctx := context.Background()

	order, err := pe.PlaceOrder(ctx, exchange.OrderRequest{
		Symbol:   "BTC-USD",
		Side:     exchange.SideBuy,
		Type:     exchange.TypeMarket,
		Quantity: 0.01,
	})
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != exchange.StatusFilled {
		t.Fatalf("expected filled, got %s", order.Status)
	}
}

func TestPaperExchange_PlaceLimitOrder(t *testing.T) {
	pe := New(0.001)
	pe.SeedLiquidity("BTC-USD", 10, 100)
	ctx := context.Background()

	ticker, _ := pe.GetTicker(ctx, "BTC-USD")
	lowPrice := ticker.Price * 0.5 // far below market

	order, err := pe.PlaceOrder(ctx, exchange.OrderRequest{
		Symbol:   "BTC-USD",
		Side:     exchange.SideBuy,
		Type:     exchange.TypeLimit,
		Quantity: 0.01,
		Price:    lowPrice,
	})
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != exchange.StatusOpen {
		t.Fatalf("expected open, got %s", order.Status)
	}
}

func TestPaperExchange_GetOrder(t *testing.T) {
	pe := New(0.001)
	pe.SeedLiquidity("BTC-USD", 10, 100)
	ctx := context.Background()

	order, _ := pe.PlaceOrder(ctx, exchange.OrderRequest{
		Symbol:   "BTC-USD",
		Side:     exchange.SideBuy,
		Type:     exchange.TypeMarket,
		Quantity: 0.01,
	})

	retrieved, err := pe.GetOrder(ctx, order.ID)
	if err != nil {
		t.Fatal(err)
	}
	if retrieved.ID != order.ID {
		t.Fatal("order ID mismatch")
	}
}

func TestPaperExchange_CancelOrder(t *testing.T) {
	pe := New(0.001)
	pe.SeedLiquidity("BTC-USD", 10, 100)
	ctx := context.Background()

	ticker, _ := pe.GetTicker(ctx, "BTC-USD")
	order, _ := pe.PlaceOrder(ctx, exchange.OrderRequest{
		Symbol:   "BTC-USD",
		Side:     exchange.SideBuy,
		Type:     exchange.TypeLimit,
		Quantity: 0.01,
		Price:    ticker.Price * 0.5,
	})

	if err := pe.CancelOrder(ctx, order.ID); err != nil {
		t.Fatal(err)
	}

	retrieved, _ := pe.GetOrder(ctx, order.ID)
	if retrieved.Status != exchange.StatusCancelled {
		t.Fatalf("expected cancelled, got %s", retrieved.Status)
	}
}

func TestPaperExchange_ListOpenOrders(t *testing.T) {
	pe := New(0.001)
	pe.SeedLiquidity("BTC-USD", 10, 100)
	ctx := context.Background()

	ticker, _ := pe.GetTicker(ctx, "BTC-USD")
	pe.PlaceOrder(ctx, exchange.OrderRequest{
		Symbol:   "BTC-USD",
		Side:     exchange.SideBuy,
		Type:     exchange.TypeLimit,
		Quantity: 0.01,
		Price:    ticker.Price * 0.5,
	})

	orders, err := pe.ListOpenOrders(ctx, "BTC-USD")
	if err != nil {
		t.Fatal(err)
	}
	if len(orders) == 0 {
		t.Fatal("expected at least one open order")
	}
}

func TestPaperExchange_GetBalance(t *testing.T) {
	pe := New(0.001)
	bal, err := pe.GetBalance(context.Background(), "USD")
	if err != nil {
		t.Fatal(err)
	}
	if bal.Total <= 0 {
		t.Fatal("expected positive USD balance")
	}
}

func TestPaperExchange_GetPosition(t *testing.T) {
	pe := New(0.001)
	pe.SeedLiquidity("BTC-USD", 10, 100)
	ctx := context.Background()

	pe.PlaceOrder(ctx, exchange.OrderRequest{
		Symbol:   "BTC-USD",
		Side:     exchange.SideBuy,
		Type:     exchange.TypeMarket,
		Quantity: 0.01,
	})

	pos, err := pe.GetPosition(ctx, "BTC-USD")
	if err != nil {
		t.Fatal(err)
	}
	if pos.Quantity <= 0 {
		t.Fatal("expected positive BTC position after buy")
	}
}

func TestPaperExchange_GetOrder_NotFound(t *testing.T) {
	pe := New(0.001)
	_, err := pe.GetOrder(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent order")
	}
	if err != exchange.ErrOrderNotFound {
		t.Fatalf("expected ErrOrderNotFound, got %v", err)
	}
}

func TestRandomWalkFeed(t *testing.T) {
	feed := NewRandomWalkFeed("BTC-USD", 50000, 0.1, 0.3, 10*time.Millisecond)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	go feed.Run(ctx)

	_, ch := feed.Subscribe()
	select {
	case price := <-ch:
		if price <= 0 {
			t.Fatal("expected positive price")
		}
	case <-ctx.Done():
		t.Fatal("timeout waiting for price update")
	}
}

func TestStaticFeed(t *testing.T) {
	feed := NewStaticFeed("BTC-USD", 50000)
	if feed.Price() != 50000 {
		t.Fatal("expected 50000")
	}
	feed.SetPrice(51000)
	if feed.Price() != 51000 {
		t.Fatal("expected 51000")
	}
}
