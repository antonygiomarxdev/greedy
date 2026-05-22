package delivery

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/idempotency"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/db"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/paper"
	"github.com/antonygiomarxdev/greedy/internal/kernel"
	"github.com/antonygiomarxdev/greedy/internal/markettracker"
	"github.com/antonygiomarxdev/greedy/internal/mcp"
	"github.com/antonygiomarxdev/greedy/internal/pricestore"
	"github.com/antonygiomarxdev/greedy/internal/pricestreamer"
	"github.com/antonygiomarxdev/greedy/internal/shared"
	"github.com/antonygiomarxdev/greedy/internal/trading"
	"github.com/antonygiomarxdev/greedy/internal/trading/strategy"
)

type ServeConfig struct {
	DataDir   string
	Bootstrap []config.BotConfig
	MCP       bool
}

func ServeCommand(ctx context.Context, logger *slog.Logger, args []string) {
	cfg := parseServeFlags(args)
	ServeCommandWithConfig(ctx, logger, cfg)
}

func parseServeFlags(args []string) ServeConfig {
	cfg := ServeConfig{MCP: true}

	fset := flag.NewFlagSet("serve", flag.ExitOnError)
	configFile := fset.String("config", "", "daemon config YAML")
	stratFile := fset.String("strategy", "", "single strategy YAML")
	mcpFlag := fset.Bool("mcp", true, "enable MCP server (stdio)")

	if err := fset.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "error parsing serve flags: %v\n", err)
		os.Exit(1)
	}

	cfg.MCP = *mcpFlag

	if *configFile != "" {
		root, err := config.Load(*configFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
			os.Exit(1)
		}
		cfg.DataDir = root.DataDir
		cfg.Bootstrap = root.Bots
	}

	if *stratFile != "" {
		botCfg, err := config.LoadStrategyFile(*stratFile, strategy.Validator())
		if err != nil {
			fmt.Fprintf(os.Stderr, "error loading strategy: %v\n", err)
			os.Exit(1)
		}
		if botCfg.ID == "" {
			botCfg.ID = fmt.Sprintf("%s-%s", botCfg.Strategy.Type, botCfg.Strategy.Symbol)
		}
		cfg.Bootstrap = append(cfg.Bootstrap, *botCfg)
	}

	return cfg
}

func ServeCommandWithConfig(ctx context.Context, logger *slog.Logger, cfg ServeConfig) {
	if cfg.DataDir == "" {
		home, _ := os.UserHomeDir()
		cfg.DataDir = home + "/.greedy"
	}

	database, err := db.Open(cfg.DataDir)
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

	if err := kernel.RestoreExchange(database, exchange); err != nil {
		logger.Warn("could not restore exchange state", "error", err)
	}

	marketsSeeded := map[string]bool{shared.DefaultSymbol: false}
	for _, bot := range cfg.Bootstrap {
		sym := bot.Strategy.Symbol
		if _, ok := marketsSeeded[sym]; !ok {
			exchange.AddMarket(sym, paper.NewRandomWalkFeed(sym, shared.DefaultBasePrice, shared.DefaultRandomWalkDrift, shared.DefaultRandomWalkVolatility, shared.DefaultTickInterval))
			exchange.SeedLiquidity(sym, shared.DefaultLiquidityLevels, shared.DefaultLiquidityDepth)
			marketsSeeded[sym] = true
		}
	}
	if !marketsSeeded[shared.DefaultSymbol] {
		exchange.AddMarket(shared.DefaultSymbol, paper.NewRandomWalkFeed(shared.DefaultSymbol, shared.DefaultBasePrice, shared.DefaultRandomWalkDrift, shared.DefaultRandomWalkVolatility, shared.DefaultTickInterval))
		exchange.SeedLiquidity(shared.DefaultSymbol, shared.DefaultLiquidityLevels, shared.DefaultLiquidityDepth)
	}

	exchange.StartFeeds(ctx)

	priceStore := pricestore.NewSQLitePriceStore(database)
	streamer := pricestreamer.New(exchange)
	streamer.SetPriceStore(priceStore)

	tracker := markettracker.New(markettracker.BreakerConfig{
		MaxPriceDeltaPct: 5.0,
		WindowDuration:   30 * time.Second,
		CooldownDuration: 60 * time.Second,
	})

	streamer.OnTick(func(symbol string, price float64, ts time.Time) {
		tracker.Record(symbol, price, ts)
	})

	for _, bot := range cfg.Bootstrap {
		sym := bot.Strategy.Symbol
		if err := streamer.Register(ctx, sym, 100*time.Millisecond); err != nil {
			logger.Error("streamer register failed", "symbol", sym, "error", err)
		}
	}

	symbols := make([]string, 0, len(cfg.Bootstrap))
	for _, bot := range cfg.Bootstrap {
		symbols = append(symbols, bot.Strategy.Symbol)
	}
	if err := tracker.Restore(ctx, symbols, priceStore); err != nil {
		logger.Warn("market tracker restore failed", "error", err)
	}

	idempotencyStore := idempotency.NewSQLiteStore(database)

	supervisor := trading.NewSupervisor(exchange, database, trading.RestartNever)
	supervisor.SetStreamer(streamer)
	supervisor.SetTracker(tracker)
	supervisor.SetIdempotency(idempotencyStore)

	for _, botCfg := range cfg.Bootstrap {
		botCfg := botCfg
		strat, err := strategy.Build(botCfg.Strategy.Type, botCfg.Strategy.Symbol, botCfg.Strategy.Params)
		if err != nil {
			logger.Error("error building strategy", "bot", botCfg.ID, "error", err)
			continue
		}
		botID := botCfg.ID
		if botID == "" {
			botID = fmt.Sprintf("%s-%s", botCfg.Strategy.Type, botCfg.Strategy.Symbol)
		}
		if err := supervisor.StartBot(ctx, botID, botCfg, strat); err != nil {
			logger.Error("error starting bot", "bot", botID, "error", err)
		}
	}

	if len(cfg.Bootstrap) > 0 {
		logger.Info("bots running", "count", len(cfg.Bootstrap))
	}

	if cfg.MCP {
		server := mcp.NewServer(exchange, supervisor, database)
		logger.Info("mcp server starting on stdio")
		go func() {
			if err := server.ServeStdio(ctx); err != nil {
				logger.Error("mcp server error", "error", err)
			}
		}()
	} else {
		logger.Info("daemon running, press Ctrl+C to stop")
	}

	<-ctx.Done()
	logger.Info("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shared.DefaultShutdownTimeout)
	defer shutdownCancel()

	if err := supervisor.ShutdownCtx(shutdownCtx); err != nil {
		logger.Warn("supervisor shutdown", "error", err)
	}

	if err := kernel.SnapshotExchange(database, exchange); err != nil {
		logger.Error("could not persist exchange state", "error", err)
	}

	logger.Info("shutdown complete")
}
