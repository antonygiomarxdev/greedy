package config

import (
	"os"
	"testing"
	"time"
)

func TestLoadStrategyFile_DCA(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/dca.yaml"
	yaml := `id: test-dca
name: "Test DCA"
exchange: paper
strategy:
  type: dca
  symbol: BTC-USD
  params:
    base_order_size: 1000
    frequency: "1h"
    max_safety_orders: 2
    safety_orders:
      - price_deviation_pct: -3
        volume_scale: 1.5
      - price_deviation_pct: -6
        volume_scale: 2.0
`
	os.WriteFile(path, []byte(yaml), 0644)

	cfg, err := LoadStrategyFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.ID != "test-dca" {
		t.Fatalf("expected id test-dca, got %s", cfg.ID)
	}
	if cfg.Strategy.Type != "dca" {
		t.Fatalf("expected strategy dca, got %s", cfg.Strategy.Type)
	}
	if cfg.Strategy.Symbol != "BTC-USD" {
		t.Fatalf("expected BTC-USD, got %s", cfg.Strategy.Symbol)
	}
	if v, ok := ParseFloatParam(cfg.Strategy.Params, "base_order_size"); !ok || v != 1000 {
		t.Fatal("missing base_order_size param")
	}
	if v, ok := ParseDurationParam(cfg.Strategy.Params, "frequency"); !ok || v != time.Hour {
		t.Fatalf("expected 1h frequency, got %v", v)
	}
}

func TestLoadStrategyFile_Grid(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/grid.yaml"
	os.WriteFile(path, []byte(`id: test-grid
name: "Test Grid"
strategy:
  type: grid
  symbol: ETH-USD
  params:
    lower_bound: 2000
    upper_bound: 4000
    grid_levels: 10
    order_size: 500
`), 0644)

	cfg, err := LoadStrategyFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Strategy.Type != "grid" {
		t.Fatal("expected grid")
	}
	if v, ok := ParseFloatParam(cfg.Strategy.Params, "lower_bound"); !ok || v != 2000 {
		t.Fatal("wrong lower_bound")
	}
	if v, ok := ParseIntParam(cfg.Strategy.Params, "grid_levels"); !ok || v != 10 {
		t.Fatal("wrong grid_levels")
	}
}

func TestLoadStrategyFile_Signal(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/signal.yaml"
	os.WriteFile(path, []byte(`id: test-sig
name: "Test Signal"
strategy:
  type: signal
  symbol: SOL-USD
  params:
    position_size: 100
    entry_condition: "ema_cross_above"
    exit_condition: "ema_cross_below"
`), 0644)

	cfg, err := LoadStrategyFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Strategy.Type != "signal" {
		t.Fatal("expected signal")
	}
	if cfg.Strategy.Symbol != "SOL-USD" {
		t.Fatal("wrong symbol")
	}
}

func TestLoadStrategyFile_NotExist(t *testing.T) {
	_, err := LoadStrategyFile("/nonexistent/config.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadStrategyFile_InvalidYAML(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/bad.yaml"
	os.WriteFile(path, []byte("{{{bad yaml"), 0644)

	_, err := LoadStrategyFile(path)
	if err == nil {
		t.Fatal("expected error for invalid yaml")
	}
}

func TestLoadStrategyFile_UnknownStrategy(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/unknown.yaml"
	os.WriteFile(path, []byte(`strategy:
  type: unknown
  symbol: BTC-USD
`), 0644)

	_, err := LoadStrategyFile(path)
	if err == nil {
		t.Fatal("expected error for unknown strategy")
	}
}

func TestBotConfig_Validate_MissingSymbol(t *testing.T) {
	cfg := &BotConfig{
		Exchange: "paper",
		Strategy: StrategyConfig{Type: "dca"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing symbol")
	}
}

func TestBotConfig_Validate_MissingType(t *testing.T) {
	cfg := &BotConfig{
		Exchange: "paper",
		Strategy: StrategyConfig{Symbol: "BTC-USD"},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for missing type")
	}
}

func TestBotConfig_Validate_DefaultExchange(t *testing.T) {
	cfg := &BotConfig{
		Strategy: StrategyConfig{
			Type:   "dca",
			Symbol: "BTC-USD",
			Params: map[string]interface{}{
				"base_order_size": 1000.0,
				"frequency":       "1h",
			},
		},
	}
	err := cfg.Validate()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Exchange != "paper" {
		t.Fatal("expected exchange to default to paper")
	}
}

func TestBotConfig_Validate_DCA_SafetyOrders(t *testing.T) {
	cfg := &BotConfig{
		Strategy: StrategyConfig{
			Type:   "dca",
			Symbol: "BTC-USD",
			Params: map[string]interface{}{
				"base_order_size": 1000.0,
				"frequency":       "1h",
				"safety_orders": []interface{}{
					map[string]interface{}{"price_deviation_pct": -5.0, "volume_scale": 2.0},
				},
			},
		},
	}
	err := cfg.Validate()
	if err != nil {
		t.Fatal(err)
	}
}

func TestBotConfig_Validate_Grid_Bounds(t *testing.T) {
	cfg := &BotConfig{
		Strategy: StrategyConfig{
			Type:   "grid",
			Symbol: "ETH-USD",
			Params: map[string]interface{}{
				"lower_bound": 2000.0,
				"upper_bound": 4000.0,
				"grid_levels": 10,
			},
		},
	}
	err := cfg.Validate()
	if err != nil {
		t.Fatal(err)
	}
}

func TestBotConfig_Validate_Grid_BoundsInverted(t *testing.T) {
	cfg := &BotConfig{
		Strategy: StrategyConfig{
			Type:   "grid",
			Symbol: "ETH-USD",
			Params: map[string]interface{}{
				"lower_bound": 4000.0,
				"upper_bound": 2000.0,
			},
		},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for inverted grid bounds")
	}
}

func TestDefaultDCAConfig(t *testing.T) {
	cfg := DefaultDCAConfig()
	if cfg.BaseOrderSize == 0 {
		t.Fatal("expected non-zero base order size")
	}
	if cfg.Frequency == 0 {
		t.Fatal("expected non-zero frequency")
	}
}

func TestDefaultGridConfig(t *testing.T) {
	cfg := DefaultGridConfig()
	if cfg.LowerBound >= cfg.UpperBound {
		t.Fatal("lower_bound must be < upper_bound")
	}
	if cfg.GridLevels <= 1 {
		t.Fatal("expected >1 grid levels")
	}
}

func TestDefaultSignalConfig(t *testing.T) {
	cfg := DefaultSignalConfig()
	if cfg.PositionSize == 0 {
		t.Fatal("expected non-zero position size")
	}
}

func TestParseFloatParam_Missing(t *testing.T) {
	params := map[string]interface{}{"other": 42}
	if _, ok := ParseFloatParam(params, "missing"); ok {
		t.Fatal("expected false for missing param")
	}
}

func TestParseDurationParam_Invalid(t *testing.T) {
	params := map[string]interface{}{"frequency": "not_a_duration"}
	if _, ok := ParseDurationParam(params, "frequency"); ok {
		t.Fatal("expected false for invalid duration")
	}
}

func TestDataDir(t *testing.T) {
	cfg := &BotConfig{}
	dir := cfg.DataDir()
	if dir == "" {
		t.Fatal("expected non-empty data dir")
	}
}
