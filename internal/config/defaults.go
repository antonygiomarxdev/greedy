package config

import "time"

const (
	DefaultDBName    = "greedy.db"
	DefaultLogLevel  = "info"
)

func DefaultDCAConfig() DCAConfig {
	return DCAConfig{
		BaseOrderSize: 100,
		Frequency:     1 * time.Hour,
		SafetyOrders: []SafetyOrder{
			{PriceDeviationPct: -5, VolumeScale: 1.5},
			{PriceDeviationPct: -10, VolumeScale: 2.0},
		},
		MaxSafetyOrders: 10,
	}
}

func DefaultGridConfig() GridConfig {
	return GridConfig{
		LowerBound: 1000,
		UpperBound: 2000,
		GridLevels: 10,
		OrderSize:  100,
	}
}

func DefaultSignalConfig() SignalConfig {
	return SignalConfig{
		PositionSize: 100,
	}
}
