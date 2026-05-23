package kernel_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	exch "github.com/antonygiomarxdev/greedy/internal/exchange"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/db"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/paper"
	"github.com/antonygiomarxdev/greedy/internal/kernel"
	"github.com/antonygiomarxdev/greedy/internal/shared"
	"github.com/antonygiomarxdev/greedy/internal/trading"
	"github.com/antonygiomarxdev/greedy/internal/trading/strategy"
)

func tempDataDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "greedy_test")
}

func TestSnapshotRestoreRoundTrip(t *testing.T) {
	dataDir := tempDataDir(t)

	database, err := db.Open(dataDir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer db.Close(database)

	if err := db.RunMigrations(database); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ex := paper.New(shared.DefaultFeeRate)
	ex.AddMarket("ETH-USD", paper.NewRandomWalkFeed("ETH-USD", 3000, 0.1, 0.3, 100*time.Millisecond))
	ex.SeedLiquidity("ETH-USD", shared.DefaultLiquidityLevels, shared.DefaultLiquidityDepth)
	ex.StartFeeds(ctx)

	time.Sleep(200 * time.Millisecond)

	req := shared.OrderRequest{
		Symbol:   "ETH-USD",
		Side:     shared.SideBuy,
		Type:     shared.TypeMarket,
		Quantity: 0.01,
	}
	if _, err := ex.PlaceOrder(ctx, req); err != nil {
		t.Fatalf("PlaceOrder: %v", err)
	}

	if err := kernel.SnapshotExchange(database, ex); err != nil {
		t.Fatalf("SnapshotExchange: %v", err)
	}

	ex2 := paper.New(shared.DefaultFeeRate)
	ex2.AddMarket("ETH-USD", paper.NewRandomWalkFeed("ETH-USD", 3000, 0.1, 0.3, 100*time.Millisecond))

	if err := kernel.RestoreExchange(database, ex2); err != nil {
		t.Fatalf("RestoreExchange: %v", err)
	}

	bal, err := ex2.GetBalance(ctx, "USD")
	if err != nil {
		t.Fatalf("GetBalance: %v", err)
	}
	if bal.Free >= 100000 {
		t.Errorf("balance should reflect spent USD, got %.2f", bal.Free)
	}

	pos, err := ex2.GetPosition(ctx, "ETH-USD")
	if err != nil {
		t.Fatalf("GetPosition: %v", err)
	}
	if pos.Quantity <= 0 {
		t.Errorf("position should have ETH quantity > 0, got %.4f", pos.Quantity)
	}
}

func TestSnapshotEmptyExchange(t *testing.T) {
	dataDir := tempDataDir(t)

	database, err := db.Open(dataDir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer db.Close(database)

	if err := db.RunMigrations(database); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	ex := paper.New(shared.DefaultFeeRate)
	if err := kernel.SnapshotExchange(database, ex); err != nil {
		t.Fatalf("SnapshotExchange: %v", err)
	}

	ex2 := paper.New(shared.DefaultFeeRate)
	if err := kernel.RestoreExchange(database, ex2); err != nil {
		t.Fatalf("RestoreExchange: %v", err)
	}

	bal, _ := ex2.GetBalance(context.Background(), "USD")
	if bal.Free != 100000 {
		t.Errorf("fresh exchange should have 100000 USD, got %.2f", bal.Free)
	}
}

func TestSupervisorShutdownWaitsForBots(t *testing.T) {
	ex := paper.New(shared.DefaultFeeRate)
	ex.AddMarket("SOL-USD", paper.NewRandomWalkFeed("SOL-USD", 150, 0, 0, 100*time.Millisecond))

	tmpDir := t.TempDir()
	database, err := db.Open(tmpDir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer db.Close(database)
	if err := db.RunMigrations(database); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	reg := exch.NewRegistry(ex)
	sup := trading.NewSupervisor(reg, database, trading.RestartNever)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ex.StartFeeds(ctx)

	cfg := config.BotConfig{
		ID:   "test-sol",
		Name: "Test SOL",
		Strategy: config.StrategyConfig{
			Type:   "grid",
			Symbol: "SOL-USD",
			Params: map[string]interface{}{
				"lower_bound": float64(100),
				"upper_bound": float64(200),
				"grid_levels": float64(5),
				"order_size":  float64(1),
			},
		},
	}

	strat, err := strategy.Build(cfg.Strategy.Type, cfg.Strategy.Symbol, cfg.Strategy.Params)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}

	if err := sup.StartBot(ctx, cfg.ID, cfg, strat); err != nil {
		t.Fatalf("StartBot: %v", err)
	}

	time.Sleep(200 * time.Millisecond)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	cancel()
	if err := sup.ShutdownCtx(shutdownCtx); err != nil {
		t.Fatalf("ShutdownCtx: %v", err)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
