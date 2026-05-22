package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/antonygiomarxdev/greedy/internal/delivery/cli"
	"github.com/antonygiomarxdev/greedy/internal/version"
)

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

	switch cmd {
	case "run":
		runCmd := flag.NewFlagSet("run", flag.ExitOnError)
		stratFile := runCmd.String("strategy", "", "strategy YAML file to run")
		if err := runCmd.Parse(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "error parsing run flags: %v\n", err)
			os.Exit(1)
		}
		cli.RunCommand(ctx, logger, *stratFile)
	case "backtest":
		backtestCmd := flag.NewFlagSet("backtest", flag.ExitOnError)
		stratFile := backtestCmd.String("strategy", "", "strategy YAML file")
		dataFile := backtestCmd.String("data", "", "CSV data file (timestamp,open,high,low,close,volume)")
		reportFmt := backtestCmd.String("report", "text", "report format: text, json")
		if err := backtestCmd.Parse(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "error parsing backtest flags: %v\n", err)
			os.Exit(1)
		}
		cli.BacktestCommand(ctx, logger, *stratFile, *dataFile, *reportFmt)
	case "status":
		cli.StatusCommand(ctx, logger)
	case "mcp-serve":
		cli.MCPServeCommand(ctx, logger)
	case "version":
		fmt.Printf("greedy version %s (commit %s, built %s)\n", version.Version, version.Commit, version.Date)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		flag.Usage()
		os.Exit(1)
	}
}
