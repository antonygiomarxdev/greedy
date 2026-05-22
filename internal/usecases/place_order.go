package usecases

import (
	"context"
	"fmt"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/domain/exchange"
)

type PlaceOrderUseCase struct {
	ex exchange.Exchange
}

func NewPlaceOrderUseCase(ex exchange.Exchange) *PlaceOrderUseCase {
	return &PlaceOrderUseCase{ex: ex}
}

func (uc *PlaceOrderUseCase) Execute(ctx context.Context, req exchange.OrderRequest) (*exchange.Order, error) {
	req.ClientOrderID = fmt.Sprintf("order-%d", time.Now().UnixNano())
	order, err := uc.ex.PlaceOrder(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("place order: %w", err)
	}
	return order, nil
}

func (uc *PlaceOrderUseCase) CancelOrder(ctx context.Context, orderID string) error {
	if err := uc.ex.CancelOrder(ctx, orderID); err != nil {
		return fmt.Errorf("cancel order: %w", err)
	}
	return nil
}
