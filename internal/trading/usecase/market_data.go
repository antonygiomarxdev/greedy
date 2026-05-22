package usecase

import (
	"context"
	"fmt"

	"github.com/antonygiomarxdev/greedy/internal/shared"
)

type GetMarketDataUseCase struct {
	ex shared.Exchange
}

func NewGetMarketDataUseCase(ex shared.Exchange) *GetMarketDataUseCase {
	return &GetMarketDataUseCase{ex: ex}
}

func (uc *GetMarketDataUseCase) GetTicker(ctx context.Context, symbol string) (*shared.Ticker, error) {
	ticker, err := uc.ex.GetTicker(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("get ticker: %w", err)
	}
	return ticker, nil
}

func (uc *GetMarketDataUseCase) GetOrderBook(ctx context.Context, symbol string, depth int) (*shared.OrderBook, error) {
	book, err := uc.ex.GetOrderBook(ctx, symbol, depth)
	if err != nil {
		return nil, fmt.Errorf("get order book: %w", err)
	}
	return book, nil
}

func (uc *GetMarketDataUseCase) GetCandles(ctx context.Context, symbol string, interval shared.CandleInterval, limit int) ([]shared.Candle, error) {
	candles, err := uc.ex.GetCandles(ctx, symbol, interval, limit)
	if err != nil {
		return nil, fmt.Errorf("get candles: %w", err)
	}
	return candles, nil
}

func (uc *GetMarketDataUseCase) GetPositions(ctx context.Context) ([]shared.Position, error) {
	positions, err := uc.ex.ListPositions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list positions: %w", err)
	}
	return positions, nil
}

func (uc *GetMarketDataUseCase) GetBalances(ctx context.Context) ([]shared.Balance, error) {
	balances, err := uc.ex.ListBalances(ctx)
	if err != nil {
		return nil, fmt.Errorf("list balances: %w", err)
	}
	return balances, nil
}
