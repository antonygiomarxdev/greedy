package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

func Load(path string) (*RootConfig, error) {
	/* #nosec G304 — path is user-provided CLI argument */
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

type StrategyValidator interface {
	Validate(strategyType string, params map[string]interface{}) error
}

func LoadStrategyFile(path string, validator StrategyValidator) (*BotConfig, error) {
	/* #nosec G304 — path is user-provided via CLI or MCP */
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read strategy file: %w", err)
	}

	var bot BotConfig
	if err := yaml.Unmarshal(data, &bot); err != nil {
		return nil, fmt.Errorf("parse strategy file: %w", err)
	}

	if err := bot.Validate(validator); err != nil {
		return nil, fmt.Errorf("validate strategy: %w", err)
	}

	return &bot, nil
}

func (b *BotConfig) Validate(validator StrategyValidator) error {
	if b.Exchange == "" {
		b.Exchange = "paper"
	}
	if b.Strategy.Type == "" {
		return fmt.Errorf("strategy.type is required")
	}
	if b.Strategy.Symbol == "" {
		return fmt.Errorf("strategy.symbol is required")
	}

	if validator != nil {
		return validator.Validate(b.Strategy.Type, b.Strategy.Params)
	}
	return nil
}
