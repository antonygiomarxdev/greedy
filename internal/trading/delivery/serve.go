package delivery

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/credentials"
	"github.com/antonygiomarxdev/greedy/internal/crypto"
	"github.com/antonygiomarxdev/greedy/internal/exchange"
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
	Exchanges []config.ExchangeConfig
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
		cfg.Exchanges = root.Exchanges
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

	paperEx := paper.New(shared.DefaultFeeRate)

	if err := kernel.RestoreExchange(database, paperEx); err != nil {
		logger.Warn("could not restore exchange state", "error", err)
	}

	reg := exchange.NewRegistry(paperEx)

	masterPassword := os.Getenv("GREEDY_MASTER_PASSWORD")
	var masterKey *[32]byte
	if masterPassword != "" {
		k := crypto.DeriveKey(masterPassword, nil)
		masterKey = &k

		credStore := credentials.NewSQLiteStore(database)
		for _, exCfg := range cfg.Exchanges {
			credLabel := exCfg.Label
			if credLabel == "" {
				credLabel = "default"
			}
			cred, err := credStore.Get(ctx, exCfg.Provider, credLabel, masterKey)
			if err != nil {
				logger.Warn("credential not found for exchange, skipping", "exchange", exCfg.Name, "provider", exCfg.Provider, "label", credLabel, "error", err)
				continue
			}
			ex, err := exchange.NewFromConfig(exCfg, cred)
			if err != nil {
				logger.Warn("failed to create exchange connector", "exchange", exCfg.Name, "error", err)
				continue
			}
			reg.Add(exCfg.Provider, ex)
			logger.Info("registered exchange", "exchange", exCfg.Name, "provider", exCfg.Provider)
		}
	} else {
		logger.Info("GREEDY_MASTER_PASSWORD not set — real exchanges disabled, paper only")
	}

	paperMarkets := map[string]bool{shared.DefaultSymbol: false}
	for _, bot := range cfg.Bootstrap {
		ex := bot.Exchange
		if ex == "" || ex == shared.ProviderPaper {
			sym := bot.Strategy.Symbol
			if _, ok := paperMarkets[sym]; !ok {
				paperEx.AddMarket(sym, paper.NewRandomWalkFeed(sym, shared.DefaultBasePrice, shared.DefaultRandomWalkDrift, shared.DefaultRandomWalkVolatility, shared.DefaultTickInterval))
				paperEx.SeedLiquidity(sym, shared.DefaultLiquidityLevels, shared.DefaultLiquidityDepth)
				paperMarkets[sym] = true
			}
		}
	}
	if !paperMarkets[shared.DefaultSymbol] {
		paperEx.AddMarket(shared.DefaultSymbol, paper.NewRandomWalkFeed(shared.DefaultSymbol, shared.DefaultBasePrice, shared.DefaultRandomWalkDrift, shared.DefaultRandomWalkVolatility, shared.DefaultTickInterval))
		paperEx.SeedLiquidity(shared.DefaultSymbol, shared.DefaultLiquidityLevels, shared.DefaultLiquidityDepth)
	}

	paperEx.StartFeeds(ctx)

	priceStore := pricestore.NewSQLitePriceStore(database)
	streamer := pricestreamer.New(paperEx)
	streamer.SetPriceStore(priceStore)

	tracker := markettracker.New(markettracker.BreakerConfig{
		MaxPriceDeltaPct: 5.0,
		WindowDuration:   30 * time.Second,
		CooldownDuration: 60 * time.Second,
	})

	streamer.OnTick(func(symbol string, price float64, ts time.Time) {
		tracker.Record(symbol, price, ts)
		_ = paperEx.SetPrice(symbol, price)
	})

	for _, bot := range cfg.Bootstrap {
		sym := bot.Strategy.Symbol
		exProvider := bot.Exchange
		if exProvider == "" {
			exProvider = shared.ProviderPaper
		}
		ex := reg.GetOrDefault(exProvider)

		// If the bot runs on paper, but a real exchange is available in the registry,
		// use the real exchange to stream real-time prices into the paper exchange.
		streamEx := ex
		if exProvider == shared.ProviderPaper {
			if binanceEx, ok := reg.Get(shared.ExchangeProvider("binance")); ok {
				streamEx = binanceEx
			} else if coinbaseEx, ok := reg.Get(shared.ExchangeProvider("coinbase")); ok {
				streamEx = coinbaseEx
			}
		}

		if err := streamer.RegisterWithExchange(ctx, sym, 100*time.Millisecond, streamEx); err != nil {
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

	supervisor := trading.NewSupervisor(reg, database, trading.RestartNever)
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
		server := mcp.NewServer(reg, supervisor, database, masterKey)
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

	if err := kernel.SnapshotExchange(database, paperEx); err != nil {
		logger.Error("could not persist exchange state", "error", err)
	}

	logger.Info("shutdown complete")
}
