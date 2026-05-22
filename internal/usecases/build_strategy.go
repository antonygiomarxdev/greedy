package usecases

import (
	"fmt"

	"github.com/antonygiomarxdev/greedy/internal/bot"
	"github.com/antonygiomarxdev/greedy/internal/bot/strategy"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
)

func BuildStrategy(cfg *config.BotConfig) (bot.Strategy, error) {
	switch cfg.Strategy.Type {
	case "dca":
		dcaCfg := config.DefaultDCAConfig()
		dcaCfg.Symbol = cfg.Strategy.Symbol
		if v, ok := config.ParseFloatParam(cfg.Strategy.Params, "base_order_size"); ok {
			dcaCfg.BaseOrderSize = v
		}
		if v, ok := config.ParseDurationParam(cfg.Strategy.Params, "frequency"); ok {
			dcaCfg.Frequency = v
		}
		if v, ok := config.ParseIntParam(cfg.Strategy.Params, "max_safety_orders"); ok {
			dcaCfg.MaxSafetyOrders = int(v)
		}
		if soList, ok := cfg.Strategy.Params["safety_orders"].([]interface{}); ok {
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
				dcaCfg.SafetyOrders = sos
			}
		}
		return strategy.NewDCA(dcaCfg), nil
	case "grid":
		gridCfg := config.DefaultGridConfig()
		gridCfg.Symbol = cfg.Strategy.Symbol
		if v, ok := config.ParseFloatParam(cfg.Strategy.Params, "lower_bound"); ok {
			gridCfg.LowerBound = v
		}
		if v, ok := config.ParseFloatParam(cfg.Strategy.Params, "upper_bound"); ok {
			gridCfg.UpperBound = v
		}
		if v, ok := config.ParseIntParam(cfg.Strategy.Params, "grid_levels"); ok {
			gridCfg.GridLevels = int(v)
		}
		if v, ok := config.ParseFloatParam(cfg.Strategy.Params, "order_size"); ok {
			gridCfg.OrderSize = v
		}
		return strategy.NewGRID(gridCfg), nil
	case "signal":
		sigCfg := config.DefaultSignalConfig()
		sigCfg.Symbol = cfg.Strategy.Symbol
		if v, ok := config.ParseFloatParam(cfg.Strategy.Params, "position_size"); ok {
			sigCfg.PositionSize = v
		}
		return strategy.NewSignal(sigCfg), nil
	default:
		return nil, fmt.Errorf("unsupported strategy: %s", cfg.Strategy.Type)
	}
}
