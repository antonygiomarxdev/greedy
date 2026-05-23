package trading_test

import (
	"context"
	"testing"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/paper"
	"github.com/antonygiomarxdev/greedy/internal/shared"
	"github.com/antonygiomarxdev/greedy/internal/trading"
	"github.com/antonygiomarxdev/greedy/internal/trading/strategy"
)

func TestMultiBotConcurrent(t *testing.T) {
	ex := paper.New(0.001)
	ex.AddMarket("BTC-USD", paper.NewStaticFeed("BTC-USD", 50000))
	ex.AddMarket("ETH-USD", paper.NewStaticFeed("ETH-USD", 3000))
	ex.SeedLiquidity("BTC-USD", 20, 50)
	ex.SeedLiquidity("ETH-USD", 20, 50)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ex.StartFeeds(ctx)

	sup := trading.NewSupervisor(ex, nil, trading.RestartNever)

	// Start DCA bot
	dcaCfg := config.DefaultDCAConfig()
	dcaCfg.Symbol = "BTC-USD"
	dcaCfg.Frequency = 500 * time.Millisecond
	dcaStrat := strategy.NewDCA(dcaCfg)
	if err := sup.StartBot(ctx, "dca-1", config.BotConfig{
		ID: "dca-1", Name: "DCA Test", Strategy: config.StrategyConfig{Type: "dca", Symbol: "BTC-USD"},
		Exchange: shared.ProviderPaper,
	}, dcaStrat); err != nil {
		t.Fatal(err)
	}
	// Start GRID bot
	gridCfg := config.DefaultGridConfig()
	gridCfg.Symbol = "ETH-USD"
	gridCfg.LowerBound = 2000
	gridCfg.UpperBound = 4000
	gridCfg.GridLevels = 5
	gridCfg.OrderSize = 500
	gridStrat := strategy.NewGRID(gridCfg)
	if err := sup.StartBot(ctx, "grid-1", config.BotConfig{
		ID: "grid-1", Name: "GRID Test", Strategy: config.StrategyConfig{Type: "grid", Symbol: "ETH-USD"},
		Exchange: shared.ProviderPaper,
	}, gridStrat); err != nil {
		t.Fatal(err)
	}

	// Start Signal bot
	sigCfg := config.DefaultSignalConfig()
	sigCfg.Symbol = "BTC-USD"
	sigCfg.PositionSize = 1000
	sigStrat := strategy.NewSignal(sigCfg)
	if err := sup.StartBot(ctx, "sig-1", config.BotConfig{
		ID: "sig-1", Name: "Signal Test", Strategy: config.StrategyConfig{Type: "signal", Symbol: "BTC-USD"},
		Exchange: shared.ProviderPaper,
	}, sigStrat); err != nil {
		t.Fatal(err)
	}

	// Give bots time to run
	time.Sleep(1500 * time.Millisecond)

	// Verify all three bots are running
	bots := sup.ListBots()
	if len(bots) != 3 {
		t.Fatalf("expected 3 bots, got %d", len(bots))
	}

	for id, status := range bots {
		if status.Status != trading.StatusRunning {
			t.Fatalf("bot %s expected running, got %s", id, status.Status)
		}
	}

	// Stop one bot — others should keep running
	if err := sup.StopBot("dca-1"); err != nil {
		t.Fatal(err)
	}

	time.Sleep(500 * time.Millisecond)

	bots = sup.ListBots()
	if len(bots) != 2 {
		t.Fatalf("expected 2 bots after stopping one, got %d", len(bots))
	}
	if _, exists := bots["dca-1"]; exists {
		t.Fatal("dca-1 should be gone after stop")
	}

	// Clean up
	sup.Shutdown()
}
