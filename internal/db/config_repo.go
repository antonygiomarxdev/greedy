package db

import (
	"database/sql"
	"fmt"
	"time"
)

type ConfigRepository struct {
	db *sql.DB
}

func NewConfigRepository(db *sql.DB) *ConfigRepository {
	return &ConfigRepository{db: db}
}

func (r *ConfigRepository) Set(key string, value []byte) error {
	now := time.Now().Format(time.RFC3339)
	_, err := r.db.Exec(`
		INSERT INTO configs (key, value, created_at, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at
	`, key, value, now, now)
	if err != nil {
		return fmt.Errorf("set config: %w", err)
	}
	return nil
}

func (r *ConfigRepository) Get(key string) ([]byte, error) {
	row := r.db.QueryRow("SELECT value FROM configs WHERE key = ?", key)
	var value []byte
	if err := row.Scan(&value); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("get config: %w", err)
	}
	return value, nil
}

func (r *ConfigRepository) Delete(key string) error {
	_, err := r.db.Exec("DELETE FROM configs WHERE key = ?", key)
	return err
}
