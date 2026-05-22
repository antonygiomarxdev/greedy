package trading

import (
	"context"
	"fmt"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/shared"
)

type PlaceOrderUseCase struct {
	ex shared.Exchange
}

func NewPlaceOrderUseCase(ex shared.Exchange) *PlaceOrderUseCase {
	return &PlaceOrderUseCase{ex: ex}
}

func (uc *PlaceOrderUseCase) Execute(ctx context.Context, req shared.OrderRequest) (*shared.Order, error) {
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
