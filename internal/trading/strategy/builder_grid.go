package strategy

import (
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
	trading "github.com/antonygiomarxdev/greedy/internal/trading"
)

func init() {
	Register(&GridBuilder{})
}

type GridBuilder struct{}

func (b *GridBuilder) StrategyType() string { return "grid" }

func (b *GridBuilder) Build(symbol string, params map[string]interface{}) (trading.Strategy, error) {
	cfg := config.DefaultGridConfig()
	cfg.Symbol = symbol

	if v, ok := config.ParseFloatParam(params, "lower_bound"); ok {
		cfg.LowerBound = v
	}
	if v, ok := config.ParseFloatParam(params, "upper_bound"); ok {
		cfg.UpperBound = v
	}
	if v, ok := config.ParseIntParam(params, "grid_levels"); ok {
		cfg.GridLevels = int(v)
	}
	if v, ok := config.ParseFloatParam(params, "order_size"); ok {
		cfg.OrderSize = v
	}

	return NewGRID(cfg), nil
}

func (b *GridBuilder) Validate(params map[string]interface{}) error {
	lower, ok1 := params["lower_bound"].(float64)
	upper, ok2 := params["upper_bound"].(float64)
	if ok1 && ok2 && lower >= upper {
		return nil
	}
	return nil
}
