package exchange

import (
	"fmt"

	"github.com/antonygiomarxdev/greedy/internal/credentials"
	"github.com/antonygiomarxdev/greedy/internal/exchange/binance"
	"github.com/antonygiomarxdev/greedy/internal/exchange/coinbase"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
	"github.com/antonygiomarxdev/greedy/internal/shared"
)

func NewFromConfig(cfg config.ExchangeConfig, cred *credentials.Credential) (shared.Exchange, error) {
	switch cfg.Provider {
	case shared.ProviderCoinbase:
		return newCoinbase(cfg, cred)
	case shared.ProviderBinance:
		return newBinance(cfg, cred)
	default:
		return nil, fmt.Errorf("unsupported exchange provider: %s", cfg.Provider)
	}
}

func newCoinbase(cfg config.ExchangeConfig, cred *credentials.Credential) (shared.Exchange, error) {
	cc := coinbase.Config{
		APIKey:     cred.APIKey,
		APISecret:  cred.APISecret,
		Passphrase: cred.Passphrase,
	}
	if cfg.Coinbase != nil && cfg.Coinbase.RESTBaseURL != "" {
		cc.RESTBaseURL = cfg.Coinbase.RESTBaseURL
	}
	if cfg.Sandbox && cc.RESTBaseURL == "" {
		cc.RESTBaseURL = coinbase.SandboxRESTURL
	}
	return coinbase.New(cc), nil
}

func newBinance(cfg config.ExchangeConfig, cred *credentials.Credential) (shared.Exchange, error) {
	bc := binance.Config{
		APIKey:    cred.APIKey,
		APISecret: cred.APISecret,
	}
	if cfg.Binance != nil && cfg.Binance.RESTBaseURL != "" {
		bc.RESTBaseURL = cfg.Binance.RESTBaseURL
	}
	if cfg.Sandbox && bc.RESTBaseURL == "" {
		bc.RESTBaseURL = binance.TestnetRESTURL
	}
	return binance.New(bc), nil
}
