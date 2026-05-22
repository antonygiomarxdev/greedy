package bot

import (
	"context"
	"fmt"
	"time"
)

func (b *Bot) fullReconcile(ctx context.Context) error {
	if err := b.Exchange.Ping(ctx); err != nil {
		return fmt.Errorf("exchange ping: %w", err)
	}

	orders, err := b.Exchange.ListOpenOrders(ctx, b.Config.Strategy.Symbol)
	if err != nil {
		return fmt.Errorf("list open orders: %w", err)
	}

	b.logger.Info("full reconcile",
		"symbol", b.Config.Strategy.Symbol,
		"open_orders", len(orders),
		"time", time.Now().Format(time.RFC3339),
	)

	for _, o := range orders {
		b.logger.Debug("existing order",
			"order_id", o.ID,
			"side", o.Side,
			"qty", o.Quantity,
			"filled", o.FilledQuantity,
			"status", o.Status,
		)
	}

	return nil
}
