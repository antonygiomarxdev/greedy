package strategy

import (
	"fmt"

	domain "github.com/antonygiomarxdev/greedy/internal/domain/bot"
)

type StrategyBuilder interface {
	StrategyType() string
	Build(symbol string, params map[string]interface{}) (domain.Strategy, error)
	Validate(params map[string]interface{}) error
}

type Registry struct {
	builders map[string]StrategyBuilder
}

func NewRegistry() *Registry {
	return &Registry{builders: make(map[string]StrategyBuilder)}
}

func (r *Registry) Register(b StrategyBuilder) {
	r.builders[b.StrategyType()] = b
}

func (r *Registry) Build(strategyType, symbol string, params map[string]interface{}) (domain.Strategy, error) {
	b, ok := r.builders[strategyType]
	if !ok {
		return nil, fmt.Errorf("unknown strategy type: %s", strategyType)
	}
	return b.Build(symbol, params)
}

func (r *Registry) Validate(strategyType string, params map[string]interface{}) error {
	b, ok := r.builders[strategyType]
	if !ok {
		return fmt.Errorf("unknown strategy type: %s", strategyType)
	}
	return b.Validate(params)
}
