package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/antonygiomarxdev/greedy/internal/bot"
	"github.com/antonygiomarxdev/greedy/internal/delivery/mcp"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/db"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/exchange/paper"

	dexchange "github.com/antonygiomarxdev/greedy/internal/domain/exchange"
)

func MCPServeCommand(ctx context.Context, logger *slog.Logger) {
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
	if err := db.RunMigrations(database); err != nil {
		fmt.Fprintf(os.Stderr, "error running migrations: %v\n", err)
		os.Exit(1)
	}
	exchange := paper.New(dexchange.DefaultFeeRate)
	exchange.AddMarket(dexchange.DefaultSymbol, paper.NewRandomWalkFeed(dexchange.DefaultSymbol, dexchange.DefaultBasePrice, dexchange.DefaultRandomWalkDrift, dexchange.DefaultRandomWalkVolatility, dexchange.DefaultTickInterval))
	exchange.SeedLiquidity(dexchange.DefaultSymbol, dexchange.DefaultLiquidityLevels, dexchange.DefaultLiquidityDepth)
	exchange.StartFeeds(ctx)
	supervisor := bot.NewSupervisor(exchange, database, bot.RestartNever)
	server := mcp.NewServer(exchange, supervisor, database)
	logger.Info("mcp server starting on stdio")
	if err := server.ServeStdio(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "mcp server error: %v\n", err)
		os.Exit(1)
	}
}
