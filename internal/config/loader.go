package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

func Load(path string) (*RootConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var cfg RootConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.DataDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("data_dir not set and cannot determine home dir: %w", err)
		}
		cfg.DataDir = home + "/.greedy"
	}

	return &cfg, nil
}

func LoadStrategyFile(path string) (*BotConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read strategy file: %w", err)
	}

	var bot BotConfig
	if err := yaml.Unmarshal(data, &bot); err != nil {
		return nil, fmt.Errorf("parse strategy file: %w", err)
	}

	if err := bot.Validate(); err != nil {
		return nil, fmt.Errorf("validate strategy: %w", err)
	}

	return &bot, nil
}

func (b *BotConfig) Validate() error {
	if b.Exchange == "" {
		b.Exchange = "paper"
	}
	if b.Strategy.Type == "" {
		return fmt.Errorf("strategy.type is required")
	}
	if b.Strategy.Symbol == "" {
		return fmt.Errorf("strategy.symbol is required")
	}

	switch b.Strategy.Type {
	case "dca":
		return validateDCA(b.Strategy.Params)
	case "grid":
		return validateGrid(b.Strategy.Params)
	case "signal":
		return validateSignal(b.Strategy.Params)
	default:
		return fmt.Errorf("unknown strategy type: %s", b.Strategy.Type)
	}
}

func parseDuration(raw interface{}) (time.Duration, error) {
	s, ok := raw.(string)
	if !ok {
		return 0, fmt.Errorf("expected string for duration, got %T", raw)
	}
	return time.ParseDuration(s)
}

func parseFloat(raw interface{}) (float64, bool) {
	switch v := raw.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	default:
		return 0, false
	}
}

func parseInt(raw interface{}) (int, bool) {
	switch v := raw.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func validateDCA(params map[string]interface{}) error {
	if _, ok := params["base_order_size"]; !ok {
		return fmt.Errorf("dca: base_order_size is required")
	}
	if v, ok := parseFloat(params["base_order_size"]); !ok || v <= 0 {
		return fmt.Errorf("dca: base_order_size must be > 0")
	}
	if _, ok := params["frequency"]; !ok {
		return fmt.Errorf("dca: frequency is required")
	}
	if _, err := parseDuration(params["frequency"]); err != nil {
		return fmt.Errorf("dca: invalid frequency: %w", err)
	}
	return nil
}

func validateGrid(params map[string]interface{}) error {
	lower, ok1 := parseFloat(params["lower_bound"])
	upper, ok2 := parseFloat(params["upper_bound"])
	if !ok1 || !ok2 || lower >= upper {
		return fmt.Errorf("grid: lower_bound must be < upper_bound")
	}
	levels, ok := parseInt(params["grid_levels"])
	if !ok || levels < 2 {
		return fmt.Errorf("grid: grid_levels must be >= 2")
	}
	return nil
}

func validateSignal(params map[string]interface{}) error {
	if _, ok := params["entry_condition"]; !ok {
		return fmt.Errorf("signal: entry_condition is required")
	}
	if _, ok := params["exit_condition"]; !ok {
		return fmt.Errorf("signal: exit_condition is required")
	}
	return nil
}
