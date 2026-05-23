package credentials

import "context"

type Credential struct {
	Exchange   string
	Label      string
	APIKey     string
	APISecret  string
	Passphrase string
}

type Meta struct {
	Exchange  string
	Label     string
	CreatedAt int64
}

type Store interface {
	Set(ctx context.Context, cred Credential, masterKey *[32]byte) error
	Get(ctx context.Context, exchange, label string, masterKey *[32]byte) (*Credential, error)
	List(ctx context.Context) ([]Meta, error)
	Delete(ctx context.Context, exchange, label string) error
}
