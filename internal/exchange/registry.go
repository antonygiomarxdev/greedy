package exchange

import (
	"github.com/antonygiomarxdev/greedy/internal/shared"
)

type Registry struct {
	exchanges map[shared.ExchangeProvider]shared.Exchange
	defaultEx shared.Exchange
}

func NewRegistry(defaultEx shared.Exchange) *Registry {
	return &Registry{
		exchanges: make(map[shared.ExchangeProvider]shared.Exchange),
		defaultEx: defaultEx,
	}
}

func (r *Registry) Add(provider shared.ExchangeProvider, ex shared.Exchange) {
	r.exchanges[provider] = ex
}

func (r *Registry) Get(provider shared.ExchangeProvider) (shared.Exchange, bool) {
	ex, ok := r.exchanges[provider]
	return ex, ok
}

func (r *Registry) Default() shared.Exchange {
	return r.defaultEx
}

func (r *Registry) GetOrDefault(provider shared.ExchangeProvider) shared.Exchange {
	if ex, ok := r.Get(provider); ok {
		return ex
	}
	return r.defaultEx
}
