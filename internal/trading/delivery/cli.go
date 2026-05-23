package delivery

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	exch "github.com/antonygiomarxdev/greedy/internal/exchange"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/db"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/paper"
	"github.com/antonygiomarxdev/greedy/internal/shared"
	"github.com/antonygiomarxdev/greedy/internal/trading"
	"github.com/antonygiomarxdev/greedy/internal/trading/strategy"
)

func RunCommand(ctx context.Context, logger *slog.Logger, path string) {
	if path == "" {
		fmt.Fprintln(os.Stderr, "error: --strategy flag is required for run command")
		os.Exit(1)
	}
	cfg, err := config.LoadStrategyFile(path, strategy.Validator())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading strategy: %v\n", err)
		os.Exit(1)
	}
	database, err := db.Open(cfg.DataDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close(database)
	if err := db.RunMigrations(database); err != nil {
		fmt.Fprintf(os.Stderr, "error running migrations: %v\n", err)
		os.Exit(1)
	}
	exchange := paper.New(shared.DefaultFeeRate)
	exchange.AddMarket(cfg.Strategy.Symbol, paper.NewRandomWalkFeed(cfg.Strategy.Symbol, shared.DefaultBasePrice, shared.DefaultRandomWalkDrift, shared.DefaultRandomWalkVolatility, shared.DefaultTickInterval))
	exchange.SeedLiquidity(cfg.Strategy.Symbol, shared.DefaultLiquidityLevels, shared.DefaultLiquidityDepth)
	exchange.StartFeeds(ctx)

	strat, err := strategy.Build(cfg.Strategy.Type, cfg.Strategy.Symbol, cfg.Strategy.Params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building strategy: %v\n", err)
		os.Exit(1)
	}
	botID := cfg.ID
	if botID == "" {
		botID = fmt.Sprintf("bot-%d", time.Now().Unix())
	}
	reg := exch.NewRegistry(exchange)
	supervisor := trading.NewSupervisor(reg, database, trading.RestartNever)
	if err := supervisor.StartBot(ctx, botID, *cfg, strat); err != nil {
		fmt.Fprintf(os.Stderr, "error starting bot: %v\n", err)
		os.Exit(1)
	}
	logger.Info("bot running", "id", botID, "strategy", cfg.Strategy.Type, "symbol", cfg.Strategy.Symbol)
	logger.Info("press Ctrl+C to stop")
	<-ctx.Done()
	logger.Info("shutting down...")
	supervisor.Shutdown()
	logger.Info("shutdown complete")
}

func StatusCommand(ctx context.Context, logger *slog.Logger) {
	dataDir := os.Getenv("GREEDY_HOME")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = home + "/.greedy"
	}

	database, err := db.Open(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close(database)

	repo := db.NewBotRepository(database)
	bots, err := repo.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error listing bots: %v\n", err)
		os.Exit(1)
	}

	if len(bots) == 0 {
		fmt.Println("No bots found.")
		return
	}

	fmt.Printf("%-24s %-12s %-10s %-10s %-20s\n", "ID", "NAME", "STRATEGY", "SYMBOL", "STATUS")
	fmt.Println("-----------------------------------------------------------------------------")
	for _, b := range bots {
		fmt.Printf("%-24s %-12s %-10s %-10s %-20s\n",
			b.ID, truncate(b.Name, 12), b.Strategy, b.Symbol, b.Status,
		)
	}
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}
