package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/antonygiomarxdev/greedy/internal/backtest"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
	"github.com/antonygiomarxdev/greedy/internal/trading/strategy"
)

func BacktestCommand(ctx context.Context, logger *slog.Logger, stratFile, dataFile, reportFmt string) {
	if stratFile == "" || dataFile == "" {
		fmt.Fprintln(os.Stderr, "error: --strategy and --data are required")
		os.Exit(1)
	}
	cfg, err := config.LoadStrategyFile(stratFile, strategy.Validator())
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading strategy: %v\n", err)
		os.Exit(1)
	}
	candles, err := backtest.LoadCSV(dataFile, cfg.Strategy.Symbol)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading data: %v\n", err)
		os.Exit(1)
	}
	strat, err := strategy.Build(cfg.Strategy.Type, cfg.Strategy.Symbol, cfg.Strategy.Params)
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
	r, err := backtest.FormatReport(report, reportFmt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error formatting report: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(r)
}
