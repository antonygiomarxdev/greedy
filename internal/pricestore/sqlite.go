package pricestore

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type SQLitePriceStore struct {
	db *sql.DB
}

func NewSQLitePriceStore(db *sql.DB) *SQLitePriceStore {
	return &SQLitePriceStore{db: db}
}

func (s *SQLitePriceStore) Insert(ctx context.Context, p PricePoint) error {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO price_ticks (symbol, price, timestamp, created_at) VALUES (?, ?, ?, ?)",
		p.Symbol, p.Price, p.Timestamp.UnixMilli(), time.Now().UnixMilli(),
	)
	if err != nil {
		return fmt.Errorf("insert price tick: %w", err)
	}
	return nil
}

func (s *SQLitePriceStore) QueryWindow(ctx context.Context, symbol string, from, to time.Time) ([]PricePoint, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT symbol, price, timestamp FROM price_ticks WHERE symbol = ? AND timestamp >= ? AND timestamp <= ? ORDER BY timestamp ASC",
		symbol, from.UnixMilli(), to.UnixMilli(),
	)
	if err != nil {
		return nil, fmt.Errorf("query price window: %w", err)
	}
	defer rows.Close()

	var points []PricePoint
	for rows.Next() {
		var p PricePoint
		var ts int64
		if err := rows.Scan(&p.Symbol, &p.Price, &ts); err != nil {
			return nil, fmt.Errorf("scan price point: %w", err)
		}
		p.Timestamp = time.UnixMilli(ts)
		points = append(points, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}
	return points, nil
}

func (s *SQLitePriceStore) Prune(ctx context.Context, olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan).UnixMilli()
	result, err := s.db.ExecContext(ctx,
		"DELETE FROM price_ticks WHERE timestamp < ?",
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("prune price ticks: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return n, nil
}
