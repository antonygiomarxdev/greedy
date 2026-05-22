package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	dexchange "github.com/antonygiomarxdev/greedy/internal/domain/exchange"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/db"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/exchange/paper"
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
	exchange := paper.New(dexchange.DefaultFeeRate)
	exchange.AddMarket(cfg.Strategy.Symbol, paper.NewRandomWalkFeed(cfg.Strategy.Symbol, dexchange.DefaultBasePrice, dexchange.DefaultRandomWalkDrift, dexchange.DefaultRandomWalkVolatility, dexchange.DefaultTickInterval))
	exchange.SeedLiquidity(cfg.Strategy.Symbol, dexchange.DefaultLiquidityLevels, dexchange.DefaultLiquidityDepth)
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
	supervisor := trading.NewSupervisor(exchange, database, trading.RestartNever)
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
