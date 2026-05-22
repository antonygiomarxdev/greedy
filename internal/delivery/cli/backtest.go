package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/antonygiomarxdev/greedy/internal/backtest"
	"github.com/antonygiomarxdev/greedy/internal/bot/strategy"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
)

func BacktestCommand(ctx context.Context, logger *slog.Logger, stratFile, dataFile, reportFmt string) {
	if stratFile == "" || dataFile == "" {
		fmt.Fprintln(os.Stderr, "error: --strategy and --data are required")
		os.Exit(1)
	}
	stratReg := strategy.NewRegistry()
	strategy.RegisterAll(stratReg)

	cfg, err := config.LoadStrategyFile(stratFile, stratReg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading strategy: %v\n", err)
		os.Exit(1)
	}
	candles, err := backtest.LoadCSV(dataFile, cfg.Strategy.Symbol)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading data: %v\n", err)
		os.Exit(1)
	}
	strat, err := stratReg.Build(cfg.Strategy.Type, cfg.Strategy.Symbol, cfg.Strategy.Params)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error building strategy: %v\n", err)
		os.Exit(1)
	}
	engine := backtest.NewEngine(strat, *cfg, candles)
	report, err := engine.Run(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "backtest error: %v\n", err)
		os.Exit(1)
	}
	r, _ := backtest.FormatReport(report, reportFmt)
	fmt.Print(r)
}
