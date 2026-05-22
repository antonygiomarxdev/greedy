package idempotency

import (
	"context"
	"database/sql"
	"fmt"
)

var _ Store = (*SQLiteStore)(nil)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(db *sql.DB) *SQLiteStore {
	return &SQLiteStore{db: db}
}

func (s *SQLiteStore) Reserve(ctx context.Context, clientOrderID, botID, symbol string) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO idempotency_keys (client_order_id, bot_id, symbol, status) VALUES (?, ?, ?, 'reserved')",
		clientOrderID, botID, symbol,
	)
	if err != nil {
		return fmt.Errorf("reserve idempotency key: %w", err)
	}
	return nil
}

func (s *SQLiteStore) Confirm(ctx context.Context, clientOrderID, exchangeOrderID string) error {
	result, err := s.db.ExecContext(ctx,
		"UPDATE idempotency_keys SET exchange_order_id = ?, status = 'confirmed' WHERE client_order_id = ?",
		exchangeOrderID, clientOrderID,
	)
	if err != nil {
		return fmt.Errorf("confirm idempotency key: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("idempotency key not found: %s", clientOrderID)
	}
	return nil
}

func (s *SQLiteStore) Lookup(ctx context.Context, clientOrderID string) (Record, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT client_order_id, exchange_order_id, bot_id, symbol, status FROM idempotency_keys WHERE client_order_id = ?",
		clientOrderID,
	)
	var r Record
	var exchangeOrderID sql.NullString
	err := row.Scan(&r.ClientOrderID, &exchangeOrderID, &r.BotID, &r.Symbol, &r.Status)
	if err != nil {
		return Record{}, fmt.Errorf("lookup idempotency key: %w", err)
	}
	if exchangeOrderID.Valid {
		r.ExchangeOrderID = exchangeOrderID.String
	}
	return r, nil
}
