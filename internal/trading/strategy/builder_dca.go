package strategy

import (
	domain "github.com/antonygiomarxdev/greedy/internal/domain/bot"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
)

func init() {
	Register(&DCABuilder{})
}

type DCABuilder struct{}

func (b *DCABuilder) StrategyType() string { return "dca" }

func (b *DCABuilder) Build(symbol string, params map[string]interface{}) (domain.Strategy, error) {
	cfg := config.DefaultDCAConfig()
	cfg.Symbol = symbol

	if v, ok := config.ParseFloatParam(params, "base_order_size"); ok {
		cfg.BaseOrderSize = v
	}
	if v, ok := config.ParseDurationParam(params, "frequency"); ok {
		cfg.Frequency = v
	}
	if v, ok := config.ParseIntParam(params, "max_safety_orders"); ok {
		cfg.MaxSafetyOrders = int(v)
	}
	if soList, ok := params["safety_orders"].([]interface{}); ok {
		var sos []config.SafetyOrder
		for _, s := range soList {
			if sm, ok := s.(map[string]interface{}); ok {
				so := config.SafetyOrder{}
				if v, ok := config.ParseFloatParam(sm, "price_deviation_pct"); ok {
					so.PriceDeviationPct = v
				}
				if v, ok := config.ParseFloatParam(sm, "volume_scale"); ok {
					so.VolumeScale = v
				}
				sos = append(sos, so)
			}
		}
		if len(sos) > 0 {
			cfg.SafetyOrders = sos
		}
	}

	return NewDCA(cfg), nil
}

func (b *DCABuilder) Validate(params map[string]interface{}) error {
	if _, ok := params["base_order_size"]; !ok {
		return nil
	}
	v, ok := params["base_order_size"].(float64)
	if !ok || v <= 0 {
		return nil
	}
	return nil
}
