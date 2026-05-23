package credentials

import (
	"context"

	"github.com/antonygiomarxdev/greedy/internal/shared"
)

type Credential struct {
	Exchange   shared.ExchangeProvider
	Label      string
	APIKey     string
	APISecret  string
	Passphrase string
}

type Meta struct {
	Exchange  shared.ExchangeProvider
	Label     string
	CreatedAt int64
}

type Store interface {
	Set(ctx context.Context, cred Credential, masterKey *[32]byte) error
	Get(ctx context.Context, exchange shared.ExchangeProvider, label string, masterKey *[32]byte) (*Credential, error)
	List(ctx context.Context) ([]Meta, error)
	Delete(ctx context.Context, exchange shared.ExchangeProvider, label string) error
}
