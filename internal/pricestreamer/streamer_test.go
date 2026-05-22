package pricestreamer_test

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/infrastructure/db"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/paper"
	"github.com/antonygiomarxdev/greedy/internal/pricestore"
	"github.com/antonygiomarxdev/greedy/internal/pricestreamer"
	"github.com/antonygiomarxdev/greedy/internal/shared"
)

func TestRegisterAndGetCached(t *testing.T) {
	ex := paper.New(shared.DefaultFeeRate)
	ex.AddMarket("BTC-USD", paper.NewRandomWalkFeed("BTC-USD", 50000, 0.1, 0.3, shared.DefaultTickInterval))
	ex.SeedLiquidity("BTC-USD", shared.DefaultLiquidityLevels, shared.DefaultLiquidityDepth)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ex.StartFeeds(ctx)

	streamer := pricestreamer.New(ex)
	if err := streamer.Register(ctx, "BTC-USD", 100*time.Millisecond); err != nil {
		t.Fatalf("Register: %v", err)
	}
	defer streamer.Unregister("BTC-USD")

	time.Sleep(300 * time.Millisecond)

	cached, ok := streamer.GetCached("BTC-USD")
	if !ok {
		t.Fatal("GetCached returned false")
	}
	if cached.Stale {
		t.Error("cached ticker should not be stale")
	}
	if cached.Price == 0 {
		t.Error("cached price should not be zero")
	}
}

func TestRefCountMultipleRegistrations(t *testing.T) {
	ex := paper.New(shared.DefaultFeeRate)
	ex.AddMarket("ETH-USD", paper.NewRandomWalkFeed("ETH-USD", 3000, 0.1, 0.3, shared.DefaultTickInterval))
	ex.SeedLiquidity("ETH-USD", shared.DefaultLiquidityLevels, shared.DefaultLiquidityDepth)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ex.StartFeeds(ctx)

	streamer := pricestreamer.New(ex)

	if err := streamer.Register(ctx, "ETH-USD", 100*time.Millisecond); err != nil {
		t.Fatalf("Register #1: %v", err)
	}
	if err := streamer.Register(ctx, "ETH-USD", 100*time.Millisecond); err != nil {
		t.Fatalf("Register #2: %v", err)
	}

	symbols := streamer.ActiveSymbols()
	if len(symbols) != 1 {
		t.Fatalf("expected 1 active symbol, got %d: %v", len(symbols), symbols)
	}

	streamer.Unregister("ETH-USD")
	symbols = streamer.ActiveSymbols()
	if len(symbols) != 1 {
		t.Fatalf("expected 1 active symbol after 1 unregister, got %d", len(symbols))
	}

	streamer.Unregister("ETH-USD")
	symbols = streamer.ActiveSymbols()
	if len(symbols) != 0 {
		t.Fatalf("expected 0 active symbols after 2 unregisters, got %d", len(symbols))
	}
}

func TestGetCachedUnknownSymbol(t *testing.T) {
	ex := paper.New(shared.DefaultFeeRate)
	streamer := pricestreamer.New(ex)

	_, ok := streamer.GetCached("UNKNOWN")
	if ok {
		t.Error("GetCached should return false for unknown symbol")
	}
}

func TestStaleOnError(t *testing.T) {
	ex := paper.New(shared.DefaultFeeRate)
	ex.AddMarket("SOL-USD", paper.NewRandomWalkFeed("SOL-USD", 150, 0.1, 0.3, shared.DefaultTickInterval))
	ex.SeedLiquidity("SOL-USD", shared.DefaultLiquidityLevels, shared.DefaultLiquidityDepth)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ex.StartFeeds(ctx)

	streamer := pricestreamer.New(ex)
	if err := streamer.Register(ctx, "SOL-USD", 100*time.Millisecond); err != nil {
		t.Fatalf("Register: %v", err)
	}

	time.Sleep(300 * time.Millisecond)

	cached, ok := streamer.GetCached("SOL-USD")
	if !ok {
		t.Fatal("GetCached returned false")
	}
	if cached.Stale {
		t.Error("cached ticker should not be stale initially")
	}

	cancel()
	time.Sleep(200 * time.Millisecond)

	cached2, ok := streamer.GetCached("SOL-USD")
	if !ok {
		t.Fatal("GetCached returned false")
	}
	if !cached2.Stale {
		t.Error("cached ticker should be stale after context cancellation")
	}

	streamer.Unregister("SOL-USD")
}

func TestOnTickCallback(t *testing.T) {
	ex := paper.New(shared.DefaultFeeRate)
	ex.AddMarket("BTC-USD", paper.NewRandomWalkFeed("BTC-USD", 50000, 0.1, 0.3, shared.DefaultTickInterval))
	ex.SeedLiquidity("BTC-USD", shared.DefaultLiquidityLevels, shared.DefaultLiquidityDepth)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ex.StartFeeds(ctx)

	var mu sync.Mutex
	var ticks []float64
	streamer := pricestreamer.New(ex)
	streamer.OnTick(func(symbol string, price float64, ts time.Time) {
		mu.Lock()
		ticks = append(ticks, price)
		mu.Unlock()
	})

	if err := streamer.Register(ctx, "BTC-USD", 100*time.Millisecond); err != nil {
		t.Fatalf("Register: %v", err)
	}
	defer streamer.Unregister("BTC-USD")

	time.Sleep(500 * time.Millisecond)

	mu.Lock()
	count := len(ticks)
	mu.Unlock()
	if count < 2 {
		t.Errorf("expected at least 2 tick callbacks, got %d", count)
	}
}

func TestPriceStoreIntegration(t *testing.T) {
	dataDir := t.TempDir()
	database, err := db.Open(dataDir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()
	if err := db.RunMigrations(database); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	store := pricestore.NewSQLitePriceStore(database)

	ex := paper.New(shared.DefaultFeeRate)
	ex.AddMarket("BTC-USD", paper.NewRandomWalkFeed("BTC-USD", 50000, 0.1, 0.3, shared.DefaultTickInterval))
	ex.SeedLiquidity("BTC-USD", shared.DefaultLiquidityLevels, shared.DefaultLiquidityDepth)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ex.StartFeeds(ctx)

	streamer := pricestreamer.New(ex)
	streamer.SetPriceStore(store)

	if err := streamer.Register(ctx, "BTC-USD", 200*time.Millisecond); err != nil {
		t.Fatalf("Register: %v", err)
	}
	defer streamer.Unregister("BTC-USD")

	time.Sleep(600 * time.Millisecond)

	points, err := store.QueryWindow(ctx, "BTC-USD", time.Time{}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("QueryWindow: %v", err)
	}
	if len(points) < 2 {
		t.Errorf("expected at least 2 price points persisted, got %d", len(points))
	}
}

func TestConcurrentRegisterAndGetCached(t *testing.T) {
	ex := paper.New(shared.DefaultFeeRate)
	ex.AddMarket("BTC-USD", paper.NewRandomWalkFeed("BTC-USD", 50000, 0.1, 0.3, shared.DefaultTickInterval))
	ex.AddMarket("ETH-USD", paper.NewRandomWalkFeed("ETH-USD", 3000, 0.1, 0.3, shared.DefaultTickInterval))
	ex.SeedLiquidity("BTC-USD", shared.DefaultLiquidityLevels, shared.DefaultLiquidityDepth)
	ex.SeedLiquidity("ETH-USD", shared.DefaultLiquidityLevels, shared.DefaultLiquidityDepth)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ex.StartFeeds(ctx)

	streamer := pricestreamer.New(ex)

	var wg sync.WaitGroup
	for _, sym := range []string{"BTC-USD", "ETH-USD"} {
		wg.Add(2)
		sym := sym
		go func() {
			defer wg.Done()
			_ = streamer.Register(ctx, sym, 200*time.Millisecond)
		}()
		go func() {
			defer wg.Done()
			time.Sleep(50 * time.Millisecond)
			for range 10 {
				streamer.GetCached(sym)
				time.Sleep(10 * time.Millisecond)
			}
		}()
	}

	wg.Wait()

	for _, sym := range []string{"BTC-USD", "ETH-USD"} {
		streamer.Unregister(sym)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
