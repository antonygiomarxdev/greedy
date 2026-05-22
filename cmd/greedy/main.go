package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/bot"
	"github.com/antonygiomarxdev/greedy/internal/bot/strategy"
	"github.com/antonygiomarxdev/greedy/internal/config"
	"github.com/antonygiomarxdev/greedy/internal/db"
	"github.com/antonygiomarxdev/greedy/internal/exchange/paper"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Greedy - Sovereign Algorithmic Trading Engine

Usage:
  greedy run --strategy <file>    Run a trading strategy
  greedy status                   Show active bots
  greedy mcp-serve                Start MCP server (stdio)
  greedy version                  Print version

`)
	}
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cmd := args[0]

	switch cmd {
	case "run":
		runCmd := flag.NewFlagSet("run", flag.ExitOnError)
		stratFile := runCmd.String("strategy", "", "strategy YAML file to run")
		runCmd.Parse(args[1:])
		runCommand(ctx, logger, *stratFile)
	case "status":
		statusCommand(ctx, logger)
	case "mcp-serve":
		mcpServeCommand(ctx, logger)
	case "version":
		fmt.Println("greedy version 0.1.0")
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		flag.Usage()
		os.Exit(1)
	}
}

func runCommand(ctx context.Context, logger *slog.Logger, path string) {
	if path == "" {
		fmt.Fprintln(os.Stderr, "error: --strategy flag is required for run command")
		os.Exit(1)
	}

	// Load strategy config
	cfg, err := config.LoadStrategyFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading strategy: %v\n", err)
		os.Exit(1)
	}

	// Open DB
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

	// Create paper exchange
	exchange := paper.New(0.001) // 0.1% fee
	exchange.AddMarket(cfg.Strategy.Symbol, paper.NewRandomWalkFeed(cfg.Strategy.Symbol, 50000, 0.1, 0.3, 100*time.Millisecond))
	exchange.SeedLiquidity(cfg.Strategy.Symbol, 10, 100)

	// Start price feeds
	exchange.StartFeeds(ctx)

	// Create strategy instance
	var strat bot.Strategy
	switch cfg.Strategy.Type {
	case "dca":
		dcaCfg := config.DefaultDCAConfig()
		dcaCfg.Symbol = cfg.Strategy.Symbol
		if v, ok := config.ParseFloatParam(cfg.Strategy.Params, "base_order_size"); ok {
			dcaCfg.BaseOrderSize = v
		}
		if v, ok := config.ParseDurationParam(cfg.Strategy.Params, "frequency"); ok {
			dcaCfg.Frequency = v
		}
		if v, ok := config.ParseIntParam(cfg.Strategy.Params, "max_safety_orders"); ok {
			dcaCfg.MaxSafetyOrders = int(v)
		}
		if soList, ok := cfg.Strategy.Params["safety_orders"].([]interface{}); ok {
			var sos []config.SafetyOrder
			for _, s := range soList {
				if sm, ok := s.(map[string]interface{}); ok {
					so := config.SafetyOrder{}
					if v, ok := config.ParseFloatParam(sm, "price_deviation_pct"); ok {
						so.PriceDeviationPct = v
					}
					if v, ok := config.ParseFloatParam(sm, "volume_scale"); ok {
						so.VolumeScale = v
					}
					sos = append(sos, so)
				}
			}
			if len(sos) > 0 {
				dcaCfg.SafetyOrders = sos
			}
		}
		strat = strategy.NewDCA(dcaCfg)
	default:
		fmt.Fprintf(os.Stderr, "unknown strategy type: %s\n", cfg.Strategy.Type)
		os.Exit(1)
	}

	// Create bot ID
	botID := cfg.ID
	if botID == "" {
		botID = fmt.Sprintf("bot-%d", time.Now().Unix())
	}
	botName := cfg.Name
	if botName == "" {
		botName = botID
	}

	// Start supervisor
	supervisor := bot.NewSupervisor(exchange, database, bot.RestartNever)
	if err := supervisor.StartBot(ctx, botID, *cfg, strat); err != nil {
		fmt.Fprintf(os.Stderr, "error starting bot: %v\n", err)
		os.Exit(1)
	}

	logger.Info("bot running", "id", botID, "strategy", cfg.Strategy.Type, "symbol", cfg.Strategy.Symbol)
	logger.Info("press Ctrl+C to stop")

	// Wait for signal
	<-ctx.Done()
	logger.Info("shutting down...")
	supervisor.Shutdown()
	logger.Info("shutdown complete")
}

func statusCommand(ctx context.Context, logger *slog.Logger) {
	fmt.Println("status: not yet implemented")
}

func mcpServeCommand(ctx context.Context, logger *slog.Logger) {
	fmt.Println("mcp-serve: not yet implemented")
}
