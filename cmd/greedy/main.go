package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/antonygiomarxdev/greedy/internal/cli"
	"github.com/antonygiomarxdev/greedy/internal/version"
)

type commandHandler func(ctx context.Context, logger *slog.Logger, args []string)

var commands = map[string]commandHandler{
	"run": func(ctx context.Context, logger *slog.Logger, args []string) {
		fset := flag.NewFlagSet("run", flag.ExitOnError)
		stratFile := fset.String("strategy", "", "strategy YAML file to run")
		if err := fset.Parse(args); err != nil {
			fmt.Fprintf(os.Stderr, "error parsing run flags: %v\n", err)
			os.Exit(1)
		}
		cli.RunCommand(ctx, logger, *stratFile)
	},
	"backtest": func(ctx context.Context, logger *slog.Logger, args []string) {
		fset := flag.NewFlagSet("backtest", flag.ExitOnError)
		stratFile := fset.String("strategy", "", "strategy YAML file")
		dataFile := fset.String("data", "", "CSV data file (timestamp,open,high,low,close,volume)")
		reportFmt := fset.String("report", "text", "report format: text, json")
		if err := fset.Parse(args); err != nil {
			fmt.Fprintf(os.Stderr, "error parsing backtest flags: %v\n", err)
			os.Exit(1)
		}
		cli.BacktestCommand(ctx, logger, *stratFile, *dataFile, *reportFmt)
	},
	"status": func(ctx context.Context, logger *slog.Logger, args []string) {
		cli.StatusCommand(ctx, logger)
	},
	"mcp-serve": func(ctx context.Context, logger *slog.Logger, args []string) {
		cli.MCPServeCommand(ctx, logger)
	},
	"version": func(ctx context.Context, logger *slog.Logger, args []string) {
		fmt.Printf("greedy version %s (commit %s, built %s)\n", version.Version, version.Commit, version.Date)
	},
}

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, `Greedy - Sovereign Algorithmic Trading Engine

Usage:
  greedy run --strategy <file>        Run a trading strategy
  greedy backtest --strategy <file> --data <csv>   Run backtest
  greedy status                       Show active bots
  greedy mcp-serve                    Start MCP server (stdio)
  greedy version                      Print version

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
	handler, ok := commands[cmd]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		flag.Usage()
		os.Exit(1)
	}

	handler(ctx, logger, args[1:])
}
