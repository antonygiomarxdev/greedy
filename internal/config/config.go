package config

import (
	"os"
	"time"
)

type RootConfig struct {
	DataDir string      `yaml:"data_dir"`
	Bots    []BotConfig `yaml:"bots,omitempty"`
}

type BotConfig struct {
	ID          string         `yaml:"id"`
	Name        string         `yaml:"name"`
	Exchange    string         `yaml:"exchange"` // "paper" for paper trading
	DataDirPath string         `yaml:"data_dir,omitempty"`
	Strategy    StrategyConfig `yaml:"strategy"`
}

func (b *BotConfig) DataDir() string {
	if b.DataDirPath != "" {
		return b.DataDirPath
	}
	home, _ := os.UserHomeDir()
	return home + "/.greedy"
}

type StrategyConfig struct {
	Type   string                 `yaml:"type"` // "dca", "grid", "signal"
	Symbol string                 `yaml:"symbol"`
	Params map[string]interface{} `yaml:"params"`
}

type DCAConfig struct {
	Symbol          string        `yaml:"symbol"`
	BaseOrderSize   float64       `yaml:"base_order_size"`
	Frequency       time.Duration `yaml:"frequency"`
	SafetyOrders    []SafetyOrder `yaml:"safety_orders"`
	MaxSafetyOrders int           `yaml:"max_safety_orders"`
}

type SafetyOrder struct {
	PriceDeviationPct float64 `yaml:"price_deviation_pct"`
	VolumeScale       float64 `yaml:"volume_scale"`
}

type GridConfig struct {
	Symbol     string  `yaml:"symbol"`
	LowerBound float64 `yaml:"lower_bound"`
	UpperBound float64 `yaml:"upper_bound"`
	GridLevels int     `yaml:"grid_levels"`
	OrderSize  float64 `yaml:"order_size"`
}

type SignalConfig struct {
	Symbol         string  `yaml:"symbol"`
	EntryCondition string  `yaml:"entry_condition"`
	ExitCondition  string  `yaml:"exit_condition"`
	PositionSize   float64 `yaml:"position_size"`
}
