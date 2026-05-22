package trading

import (
	"context"
	"fmt"

	"github.com/antonygiomarxdev/greedy/internal/domain/exchange"
)

type GetMarketDataUseCase struct {
	ex exchange.Exchange
}

func NewGetMarketDataUseCase(ex exchange.Exchange) *GetMarketDataUseCase {
	return &GetMarketDataUseCase{ex: ex}
}

func (uc *GetMarketDataUseCase) GetTicker(ctx context.Context, symbol string) (*exchange.Ticker, error) {
	ticker, err := uc.ex.GetTicker(ctx, symbol)
	if err != nil {
		return nil, fmt.Errorf("get ticker: %w", err)
	}
	return ticker, nil
}

func (uc *GetMarketDataUseCase) GetOrderBook(ctx context.Context, symbol string, depth int) (*exchange.OrderBook, error) {
	book, err := uc.ex.GetOrderBook(ctx, symbol, depth)
	if err != nil {
		return nil, fmt.Errorf("get order book: %w", err)
	}
	return book, nil
}

func (uc *GetMarketDataUseCase) GetCandles(ctx context.Context, symbol string, interval exchange.CandleInterval, limit int) ([]exchange.Candle, error) {
	candles, err := uc.ex.GetCandles(ctx, symbol, interval, limit)
	if err != nil {
		return nil, fmt.Errorf("get candles: %w", err)
	}
	return candles, nil
}

func (uc *GetMarketDataUseCase) GetPositions(ctx context.Context) ([]exchange.Position, error) {
	positions, err := uc.ex.ListPositions(ctx)
	if err != nil {
		return nil, fmt.Errorf("list positions: %w", err)
	}
	return positions, nil
}

func (uc *GetMarketDataUseCase) GetBalances(ctx context.Context) ([]exchange.Balance, error) {
	balances, err := uc.ex.ListBalances(ctx)
	if err != nil {
		return nil, fmt.Errorf("list balances: %w", err)
	}
	return balances, nil
}
