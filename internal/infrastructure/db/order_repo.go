package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/shared"
)

type OrderRecord struct {
	ID              string
	BotID           string
	ExchangeOrderID string
	Symbol          string
	Side            string
	Type            string
	Price           float64
	Quantity        float64
	FilledQuantity  float64
	Status          string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type OrderRepository struct {
	db *sql.DB
}

func NewOrderRepository(db *sql.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) Insert(o OrderRecord) error {
	_, err := r.db.Exec(`
		INSERT INTO orders (id, bot_id, exchange_order_id, symbol, side, type, price, quantity, filled_quantity, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, o.ID, o.BotID, o.ExchangeOrderID, o.Symbol, o.Side, o.Type,
		priceOrNil(o.Price), o.Quantity, o.FilledQuantity, o.Status,
		o.CreatedAt.Format(time.RFC3339), o.UpdatedAt.Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("insert order: %w", err)
	}
	return nil
}

func (r *OrderRepository) UpdateStatus(id, status string, filledQuantity float64) error {
	_, err := r.db.Exec(`
		UPDATE orders SET status = ?, filled_quantity = ?, updated_at = ? WHERE id = ?
	`, status, filledQuantity, time.Now().Format(time.RFC3339), id)
	if err != nil {
		return fmt.Errorf("update order: %w", err)
	}
	return nil
}

func (r *OrderRepository) ListByBot(botID string, limit int) ([]OrderRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(`
		SELECT id, bot_id, exchange_order_id, symbol, side, type, price, quantity, filled_quantity, status, created_at, updated_at
		FROM orders WHERE bot_id = ? ORDER BY created_at DESC LIMIT ?
	`, botID, limit)
	if err != nil {
		return nil, fmt.Errorf("list orders by bot: %w", err)
	}
	defer rows.Close()
	return scanOrders(rows)
}

func (r *OrderRepository) ListBySymbol(symbol string, limit int) ([]OrderRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(`
		SELECT id, bot_id, exchange_order_id, symbol, side, type, price, quantity, filled_quantity, status, created_at, updated_at
		FROM orders WHERE symbol = ? ORDER BY created_at DESC LIMIT ?
	`, symbol, limit)
	if err != nil {
		return nil, fmt.Errorf("list orders by symbol: %w", err)
	}
	defer rows.Close()
	return scanOrders(rows)
}

func (r *OrderRepository) ListAll(limit int) ([]OrderRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.Query(`
		SELECT id, bot_id, exchange_order_id, symbol, side, type, price, quantity, filled_quantity, status, created_at, updated_at
		FROM orders ORDER BY created_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	defer rows.Close()
	return scanOrders(rows)
}

func scanOrders(rows *sql.Rows) ([]OrderRecord, error) {
	var orders []OrderRecord
	for rows.Next() {
		var o OrderRecord
		var price sql.NullFloat64
		var ca, ua string
		if err := rows.Scan(&o.ID, &o.BotID, &o.ExchangeOrderID, &o.Symbol, &o.Side, &o.Type,
			&price, &o.Quantity, &o.FilledQuantity, &o.Status, &ca, &ua); err != nil {
			return nil, fmt.Errorf("scan order: %w", err)
		}
		o.Price = price.Float64
		o.CreatedAt, _ = time.Parse(time.RFC3339, ca)
		o.UpdatedAt, _ = time.Parse(time.RFC3339, ua)
		orders = append(orders, o)
	}
	return orders, rows.Err()
}

func (r *OrderRepository) ToShared(o OrderRecord) shared.Order {
	return shared.Order{
		ID:             o.ID,
		ClientOrderID:  o.ExchangeOrderID,
		Symbol:         o.Symbol,
		Side:           shared.OrderSide(o.Side),
		Type:           shared.OrderType(o.Type),
		Price:          o.Price,
		Quantity:       o.Quantity,
		FilledQuantity: o.FilledQuantity,
		Status:         shared.OrderStatus(o.Status),
		CreatedAt:      o.CreatedAt,
		UpdatedAt:      o.UpdatedAt,
	}
}

func priceOrNil(p float64) interface{} {
	if p == 0 {
		return nil
	}
	return p
}
