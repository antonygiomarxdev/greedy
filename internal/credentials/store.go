package credentials

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/antonygiomarxdev/greedy/internal/crypto"
	"github.com/antonygiomarxdev/greedy/internal/shared"
)

var _ Store = (*SQLiteStore)(nil)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

func (s *SQLiteStore) Set(ctx context.Context, cred Credential, masterKey *[32]byte) error {
	var existing int
	err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM credentials WHERE exchange = ? AND label = ?",
		cred.Exchange, cred.Label,
	).Scan(&existing)
	if err != nil {
		return fmt.Errorf("check existing credential: %w", err)
	}
	if existing > 0 {
		return shared.ErrCredentialExists
	}

	encKey, err := crypto.Encrypt([]byte(cred.APIKey), masterKey)
	if err != nil {
		return fmt.Errorf("encrypt api key: %w", err)
	}

	encSecret, err := crypto.Encrypt([]byte(cred.APISecret), masterKey)
	if err != nil {
		return fmt.Errorf("encrypt api secret: %w", err)
	}

	var encPassphrase []byte
	if cred.Passphrase != "" {
		encPassphrase, err = crypto.Encrypt([]byte(cred.Passphrase), masterKey)
		if err != nil {
			return fmt.Errorf("encrypt passphrase: %w", err)
		}
	}

	_, err = s.db.ExecContext(ctx,
		"INSERT INTO credentials (exchange, label, api_key, api_secret, passphrase) VALUES (?, ?, ?, ?, ?)",
		cred.Exchange, cred.Label, encKey, encSecret, encPassphrase,
	)
	if err != nil {
		return fmt.Errorf("insert credential: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Get(ctx context.Context, exchange, label string, masterKey *[32]byte) (*Credential, error) {
	var encKey, encSecret, encPassphrase []byte
	err := s.db.QueryRowContext(ctx,
		"SELECT api_key, api_secret, passphrase FROM credentials WHERE exchange = ? AND label = ?",
		exchange, label,
	).Scan(&encKey, &encSecret, &encPassphrase)
	if err == sql.ErrNoRows {
		return nil, shared.ErrCredentialNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query credential: %w", err)
	}

	apiKey, err := crypto.Decrypt(encKey, masterKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt api key: %w", err)
	}

	apiSecret, err := crypto.Decrypt(encSecret, masterKey)
	if err != nil {
		return nil, fmt.Errorf("decrypt api secret: %w", err)
	}

	var passphrase string
	if len(encPassphrase) > 0 {
		dec, err := crypto.Decrypt(encPassphrase, masterKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt passphrase: %w", err)
		}
		passphrase = string(dec)
	}

	return &Credential{
		Exchange:   exchange,
		Label:      label,
		APIKey:     string(apiKey),
		APISecret:  string(apiSecret),
		Passphrase: passphrase,
	}, nil
}

func (s *SQLiteStore) List(ctx context.Context) ([]Meta, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT exchange, label, created_at FROM credentials ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("list credentials: %w", err)
	}
	defer rows.Close()

	var metas []Meta
	for rows.Next() {
		var m Meta
		if err := rows.Scan(&m.Exchange, &m.Label, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan credential meta: %w", err)
		}
		metas = append(metas, m)
	}
	return metas, rows.Err()
}

func (s *SQLiteStore) Delete(ctx context.Context, exchange, label string) error {
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM credentials WHERE exchange = ? AND label = ?",
		exchange, label,
	)
	if err != nil {
		return fmt.Errorf("delete credential: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return shared.ErrCredentialNotFound
	}
	return nil
}
