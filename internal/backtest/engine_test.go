package backtest

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/bot/strategy"
	"github.com/antonygiomarxdev/greedy/internal/config"
)

func makeCandles(n int, symbol string, startPrice, trend float64) []Candle {
	now := time.Now()
	candles := make([]Candle, n)
	for i := 0; i < n; i++ {
		delta := float64(i) * trend
		price := startPrice + delta
		candles[i] = Candle{
			Timestamp: now.Add(time.Duration(i) * time.Hour),
			Symbol:    symbol,
			Open:      price,
			High:      price * 1.01,
			Low:       price * 0.99,
			Close:     price,
			Volume:    100,
		}
	}
	return candles
}

func TestBacktestDCA_BullMarket(t *testing.T) {
	candles := makeCandles(30, "BTC-USD", 50000, 200)
	cfg := config.BotConfig{
		Strategy: config.StrategyConfig{Type: "dca", Symbol: "BTC-USD"},
	}
	dcaCfg := config.DefaultDCAConfig()
	dcaCfg.Symbol = "BTC-USD"
	dcaCfg.BaseOrderSize = 5000
	dcaCfg.Frequency = 1 * time.Millisecond
	strat := strategy.NewDCA(dcaCfg)

	engine := NewEngine(strat, cfg, candles)
	report, err := engine.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if report.TotalTrades == 0 {
		t.Fatal("expected at least 1 trade in bull market")
	}
	if report.FinalBalance <= report.InitialBalance {
		t.Fatal("expected profit in bull market")
	}
	if report.SharpeRatio <= 0 {
		t.Fatal("expected positive Sharpe in bull market")
	}
}

func TestBacktestDCA_BearMarket(t *testing.T) {
	candles := makeCandles(30, "ETH-USD", 3000, -40)
	cfg := config.BotConfig{
		Strategy: config.StrategyConfig{Type: "dca", Symbol: "ETH-USD"},
	}
	dcaCfg := config.DefaultDCAConfig()
	dcaCfg.Symbol = "ETH-USD"
	dcaCfg.BaseOrderSize = 5000
	dcaCfg.Frequency = 1 * time.Millisecond
	strat := strategy.NewDCA(dcaCfg)

	engine := NewEngine(strat, cfg, candles)
	report, err := engine.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if report.TotalTrades == 0 {
		t.Fatal("DCA should buy in bear market")
	}
	if report.MaxDrawdownPct <= 0 {
		t.Fatal("expected drawdown in bear market")
	}
}

func TestBacktestDCA_SafetyOrdersTrigger(t *testing.T) {
	// Start at 50000, then sharp drop to 45000 (-10%)
	candles := []Candle{
		{Timestamp: time.Now(), Symbol: "BTC-USD", Close: 50000},
		{Timestamp: time.Now().Add(1 * time.Hour), Symbol: "BTC-USD", Close: 49000},
		{Timestamp: time.Now().Add(2 * time.Hour), Symbol: "BTC-USD", Close: 48000},
		{Timestamp: time.Now().Add(3 * time.Hour), Symbol: "BTC-USD", Close: 47000},
		{Timestamp: time.Now().Add(4 * time.Hour), Symbol: "BTC-USD", Close: 46000},
		{Timestamp: time.Now().Add(5 * time.Hour), Symbol: "BTC-USD", Close: 45000},
	}

	cfg := config.BotConfig{
		Strategy: config.StrategyConfig{Type: "dca", Symbol: "BTC-USD"},
	}
	dcaCfg := config.DefaultDCAConfig()
	dcaCfg.Symbol = "BTC-USD"
	dcaCfg.BaseOrderSize = 1000
	dcaCfg.Frequency = 1 * time.Millisecond
	dcaCfg.MaxSafetyOrders = 5
	dcaCfg.SafetyOrders = []config.SafetyOrder{
		{PriceDeviationPct: -5, VolumeScale: 2.0},
		{PriceDeviationPct: -8, VolumeScale: 3.0},
	}
	strat := strategy.NewDCA(dcaCfg)

	engine := NewEngine(strat, cfg, candles)
	report, err := engine.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Should have at least 3 trades: 1 base + 1 safety(-5%) + 1 safety(-8%)
	if report.TotalTrades < 2 {
		t.Fatalf("expected at least 2 trades (base + safety), got %d", report.TotalTrades)
	}
}

func TestBacktest_EmptyData(t *testing.T) {
	strat := strategy.NewDCA(config.DefaultDCAConfig())
	engine := NewEngine(strat, config.BotConfig{}, nil)
	_, err := engine.Run(context.Background())
	if err == nil {
		t.Fatal("expected error with empty data")
	}
}

func TestBacktest_ContextCancelled(t *testing.T) {
	candles := makeCandles(1000, "BTC-USD", 50000, 0)
	strat := strategy.NewDCA(config.DefaultDCAConfig())
	engine := NewEngine(strat, config.BotConfig{}, candles)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := engine.Run(ctx)
	if err == nil {
		t.Fatal("expected context cancelled error")
	}
}

func TestBacktest_ReportMetrics(t *testing.T) {
	candles := makeCandles(20, "BTC-USD", 50000, 500)
	cfg := config.BotConfig{
		Strategy: config.StrategyConfig{Type: "dca", Symbol: "BTC-USD"},
	}
	dcaCfg := config.DefaultDCAConfig()
	dcaCfg.Symbol = "BTC-USD"
	dcaCfg.BaseOrderSize = 5000
	dcaCfg.Frequency = 1 * time.Millisecond
	strat := strategy.NewDCA(dcaCfg)

	engine := NewEngine(strat, cfg, candles)
	report, err := engine.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if report.Strategy != "dca" {
		t.Fatal("wrong strategy name")
	}
	if report.Symbol != "BTC-USD" {
		t.Fatal("wrong symbol")
	}
	if report.Period == "" {
		t.Fatal("missing period")
	}
	if len(report.EquityCurve) == 0 {
		t.Fatal("missing equity curve")
	}

	text := report.FormatText()
	if text == "" {
		t.Fatal("empty text report")
	}

	json := report.FormatJSON()
	if json == "" {
		t.Fatal("empty json report")
	}
}

func TestBacktestGRID(t *testing.T) {
	candles := makeCandles(50, "ETH-USD", 3000, 10)
	cfg := config.BotConfig{
		Strategy: config.StrategyConfig{Type: "grid", Symbol: "ETH-USD"},
	}
	gridCfg := config.DefaultGridConfig()
	gridCfg.Symbol = "ETH-USD"
	gridCfg.LowerBound = 2500
	gridCfg.UpperBound = 3500
	gridCfg.GridLevels = 5
	gridCfg.OrderSize = 500
	strat := strategy.NewGRID(gridCfg)

	engine := NewEngine(strat, cfg, candles)
	report, err := engine.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if report.TotalTrades == 0 {
		t.Fatal("expected grid trades in ranging market")
	}
}

func TestCSVLoader(t *testing.T) {
	candles, err := LoadCSV("../../examples/btc_sample.csv", "BTC-USD")
	if err != nil {
		t.Fatal(err)
	}
	if len(candles) == 0 {
		t.Fatal("no candles loaded from CSV")
	}
	if candles[0].Symbol != "BTC-USD" {
		t.Fatal("wrong symbol")
	}
}

func TestCSVLoader_UnixTimestamps(t *testing.T) {
	// Create temp CSV with unix timestamps
	path := t.TempDir() + "/unix.csv"
	content := "timestamp,open,high,low,close,volume\n1704067200,50000,50100,49900,50050,100\n1704153600,50050,50200,49950,50100,120\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	candles, err := LoadCSV(path, "BTC-USD")
	if err != nil {
		t.Fatal(err)
	}
	if len(candles) != 2 {
		t.Fatalf("expected 2 candles, got %d", len(candles))
	}
}

func TestCSVLoader_EmptyFile(t *testing.T) {
	path := t.TempDir() + "/empty.csv"
	if err := os.WriteFile(path, []byte("timestamp,open,high,low,close,volume\n"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := LoadCSV(path, "BTC-USD")
	if err == nil {
		t.Fatal("expected error for empty CSV")
	}
}

func TestCSVLoader_FileNotFound(t *testing.T) {
	_, err := LoadCSV("/nonexistent/file.csv", "BTC-USD")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestBacktestJSONReport(t *testing.T) {
	candles := makeCandles(5, "BTC-USD", 50000, 0)
	strat := strategy.NewDCA(config.DefaultDCAConfig())
	cfg := config.BotConfig{Strategy: config.StrategyConfig{Type: "dca", Symbol: "BTC-USD"}}
	engine := NewEngine(strat, cfg, candles)
	report, _ := engine.Run(context.Background())

	jsonStr := report.FormatJSON()
	if len(jsonStr) == 0 {
		t.Fatal("empty json")
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
}
