package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/bot"
	"github.com/antonygiomarxdev/greedy/internal/delivery/mcp"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/db"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/exchange/paper"
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
	exchange := paper.New(0.001)
	exchange.AddMarket("BTC-USD", paper.NewRandomWalkFeed("BTC-USD", 50000, 0.1, 0.3, 100*time.Millisecond))
	exchange.SeedLiquidity("BTC-USD", 10, 100)
	exchange.StartFeeds(ctx)
	supervisor := bot.NewSupervisor(exchange, database, bot.RestartNever)
	server := mcp.NewServer(exchange, supervisor, database)
	logger.Info("mcp server starting on stdio")
	if err := server.ServeStdio(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "mcp server error: %v\n", err)
		os.Exit(1)
	}
}
