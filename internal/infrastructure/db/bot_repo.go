package db

import (
	"database/sql"
	"fmt"
	"time"
)

type BotRecord struct {
	ID         string
	Name       string
	Strategy   string
	Symbol     string
	ConfigYAML string
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type BotRepository struct {
	db *sql.DB
}

func NewBotRepository(db *sql.DB) *BotRepository {
	return &BotRepository{db: db}
}

func (r *BotRepository) Insert(bot BotRecord) error {
	_, err := r.db.Exec(`
		INSERT INTO bots (id, name, strategy, symbol, config_yaml, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, bot.ID, bot.Name, bot.Strategy, bot.Symbol, bot.ConfigYAML, bot.Status,
		bot.CreatedAt.Format(time.RFC3339), bot.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert bot: %w", err)
	}
	return nil
}

func (r *BotRepository) UpdateStatus(id, status string) error {
	_, err := r.db.Exec(`
		UPDATE bots SET status = ?, updated_at = ? WHERE id = ?
	`, status, time.Now().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("update bot status: %w", err)
	}
	return nil
}

func (r *BotRepository) Get(id string) (*BotRecord, error) {
	row := r.db.QueryRow(`
		SELECT id, name, strategy, symbol, config_yaml, status, created_at, updated_at
		FROM bots WHERE id = ?
	`, id)

	var bot BotRecord
	var ca, ua string
	err := row.Scan(&bot.ID, &bot.Name, &bot.Strategy, &bot.Symbol, &bot.ConfigYAML, &bot.Status, &ca, &ua)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get bot: %w", err)
	}

	bot.CreatedAt, _ = time.Parse(time.RFC3339, ca)
	bot.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
	return &bot, nil
}

func (r *BotRepository) List() ([]BotRecord, error) {
	rows, err := r.db.Query(`
		SELECT id, name, strategy, symbol, config_yaml, status, created_at, updated_at
		FROM bots ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list bots: %w", err)
	}
	defer rows.Close()

	var bots []BotRecord
	for rows.Next() {
		var b BotRecord
		var ca, ua string
		if err := rows.Scan(&b.ID, &b.Name, &b.Strategy, &b.Symbol, &b.ConfigYAML, &b.Status, &ca, &ua); err != nil {
			return nil, fmt.Errorf("scan bot: %w", err)
		}
		b.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		b.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
		bots = append(bots, b)
	}
	return bots, rows.Err()
}

func (r *BotRepository) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM bots WHERE id = ?", id)
	return err
}
