package strategy

import (
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
	trading "github.com/antonygiomarxdev/greedy/internal/trading"
)

func init() {
	Register(&SignalBuilder{})
}

type SignalBuilder struct{}

func (b *SignalBuilder) StrategyType() string { return "signal" }

func (b *SignalBuilder) Build(symbol string, params map[string]interface{}) (trading.Strategy, error) {
	cfg := config.DefaultSignalConfig()
	cfg.Symbol = symbol

	if v, ok := config.ParseFloatParam(params, "position_size"); ok {
		cfg.PositionSize = v
	}

	return NewSignal(cfg), nil
}

func (b *SignalBuilder) Validate(params map[string]interface{}) error {
	if _, ok := params["entry_condition"]; !ok {
		return nil
	}
	return nil
}
