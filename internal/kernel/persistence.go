package kernel

import (
	"database/sql"
	"fmt"

	"github.com/antonygiomarxdev/greedy/internal/shared"
)

func SnapshotExchange(db *sql.DB, ex shared.Exchange) error {
	pe, ok := ex.(interface {
		Snapshot() ([]shared.Balance, []shared.Position)
	})
	if !ok {
		return fmt.Errorf("exchange does not support snapshot")
	}

	balances, positions := pe.Snapshot()

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec("DELETE FROM exchange_balances"); err != nil {
		return fmt.Errorf("clear exchange_balances: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM exchange_positions"); err != nil {
		return fmt.Errorf("clear exchange_positions: %w", err)
	}

	for _, b := range balances {
		if _, err := tx.Exec(
			"INSERT INTO exchange_balances (asset, free) VALUES (?, ?)",
			b.Asset, b.Free,
		); err != nil {
			return fmt.Errorf("insert exchange_balances: %w", err)
		}
	}

	for _, p := range positions {
		if _, err := tx.Exec(
			"INSERT INTO exchange_positions (symbol, quantity, avg_entry_price) VALUES (?, ?, ?)",
			p.Symbol, p.Quantity, p.AvgEntryPrice,
		); err != nil {
			return fmt.Errorf("insert exchange_positions: %w", err)
		}
	}

	return tx.Commit()
}

func RestoreExchange(db *sql.DB, ex shared.Exchange) error {
	pe, ok := ex.(interface {
		Restore(balances []shared.Balance, positions []shared.Position)
	})
	if !ok {
		return fmt.Errorf("exchange does not support restore")
	}

	rows, err := db.Query("SELECT asset, free FROM exchange_balances")
	if err != nil {
		return fmt.Errorf("query exchange_balances: %w", err)
	}
	defer rows.Close()

	var balances []shared.Balance
	for rows.Next() {
		var b shared.Balance
		if err := rows.Scan(&b.Asset, &b.Free); err != nil {
			return fmt.Errorf("scan exchange_balances: %w", err)
		}
		b.Total = b.Free
		balances = append(balances, b)
	}

	posRows, err := db.Query("SELECT symbol, quantity, avg_entry_price FROM exchange_positions")
	if err != nil {
		return fmt.Errorf("query exchange_positions: %w", err)
	}
	defer posRows.Close()

	var positions []shared.Position
	for posRows.Next() {
		var p shared.Position
		if err := posRows.Scan(&p.Symbol, &p.Quantity, &p.AvgEntryPrice); err != nil {
			return fmt.Errorf("scan exchange_positions: %w", err)
		}
		positions = append(positions, p)
	}

	pe.Restore(balances, positions)
	return nil
}
