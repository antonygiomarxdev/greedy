package delivery

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/antonygiomarxdev/greedy/internal/crypto"
	exch "github.com/antonygiomarxdev/greedy/internal/exchange"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/db"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/paper"
	"github.com/antonygiomarxdev/greedy/internal/mcp"
	"github.com/antonygiomarxdev/greedy/internal/shared"
	"github.com/antonygiomarxdev/greedy/internal/trading"
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
	exchange := paper.New(shared.DefaultFeeRate)
	exchange.AddMarket(shared.DefaultSymbol, paper.NewRandomWalkFeed(shared.DefaultSymbol, shared.DefaultBasePrice, shared.DefaultRandomWalkDrift, shared.DefaultRandomWalkVolatility, shared.DefaultTickInterval))
	exchange.SeedLiquidity(shared.DefaultSymbol, shared.DefaultLiquidityLevels, shared.DefaultLiquidityDepth)
	exchange.StartFeeds(ctx)
	reg := exch.NewRegistry(exchange)
	supervisor := trading.NewSupervisor(reg, database, trading.RestartNever)

	var masterKey *[32]byte
	if pwd := os.Getenv("GREEDY_MASTER_PASSWORD"); pwd != "" {
		k := crypto.DeriveKey(pwd, nil)
		masterKey = &k
	}

	server := mcp.NewServer(reg, supervisor, database, masterKey)
	logger.Info("mcp server starting on stdio")
	if err := server.ServeStdio(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "mcp server error: %v\n", err)
		os.Exit(1)
	}
}
