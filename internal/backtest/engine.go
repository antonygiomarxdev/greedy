package backtest

import (
	"context"
	"fmt"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/domain/bot"
	"github.com/antonygiomarxdev/greedy/internal/domain/exchange"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/config"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/exchange/paper"
)

type Engine struct {
	exchange *paper.PaperExchange
	strategy bot.Strategy
	cfg      config.BotConfig
	data     []Candle

	initialBalance float64
	finalBalance   float64
	trades         []Trade
	equityCurve    []EquityPoint
}

type Trade struct {
	Timestamp time.Time
	Symbol    string
	Side      string
	Price     float64
	Quantity  float64
	PnL       float64
}

type EquityPoint struct {
	Timestamp time.Time
	Equity    float64
}

type Report struct {
	Symbol         string
	Strategy       string
	Period         string
	InitialBalance float64
	FinalBalance   float64
	TotalReturn    float64
	TotalReturnPct float64
	SharpeRatio    float64
	MaxDrawdownPct float64
	WinRate        float64
	ProfitFactor   float64
	TotalTrades    int
	WinningTrades  int
	LosingTrades   int
	EquityCurve    []EquityPoint
}

func NewEngine(strat bot.Strategy, cfg config.BotConfig, data []Candle) *Engine {
	ex := paper.New(0.001)
	if len(data) > 0 {
		ex.AddMarket(cfg.Strategy.Symbol, paper.NewStaticFeed(cfg.Strategy.Symbol, data[0].Close))
		ex.SeedLiquidity(cfg.Strategy.Symbol, 20, data[0].Close*0.01)
	}

	return &Engine{
		exchange:       ex,
		strategy:       strat,
		cfg:            cfg,
		data:           data,
		initialBalance: 100000,
		finalBalance:   100000,
	}
}

func (e *Engine) Run(ctx context.Context) (*Report, error) {
	if len(e.data) == 0 {
		return nil, fmt.Errorf("no data provided")
	}

	e.exchange.StartFeeds(ctx)

	startTime := e.data[0].Timestamp
	endTime := e.data[len(e.data)-1].Timestamp

	// Calculate effective frequency in candles
	candleDuration := e.data[1].Timestamp.Sub(e.data[0].Timestamp)
	tickFreq := candleDuration
	if len(e.data) > 1 {
		tickFreq = candleDuration
	}

	// Time acceleration: process each candle at an accelerated rate
	// By default, each candle takes 50ms, so strategies with time-based
	// logic (like DCA) can execute multiple trades during the backtest.
	accelTick := 50 * time.Millisecond
	accelTicker := time.NewTicker(accelTick)
	defer accelTicker.Stop()

	for i, candle := range e.data {
		_ = i
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-accelTicker.C:
		}

		e.exchange.AddMarket(e.cfg.Strategy.Symbol, paper.NewStaticFeed(e.cfg.Strategy.Symbol, candle.Close))
		e.exchange.SeedLiquidity(e.cfg.Strategy.Symbol, 20, candle.Close*0.01)

		ticker := &exchange.Ticker{Symbol: candle.Symbol, Price: candle.Close, Time: candle.Timestamp}
		position, _ := e.exchange.GetPosition(ctx, candle.Symbol)
		balance, _ := e.exchange.GetBalance(ctx, "USD")
		openOrders, _ := e.exchange.ListOpenOrders(ctx, candle.Symbol)

		state := &bot.BotState{
			Symbol:     candle.Symbol,
			Ticker:     ticker,
			Position:   position,
			Balance:    balance,
			OpenOrders: openOrders,
		}

		signal, err := e.strategy.Evaluate(ctx, state)
		if err != nil {
			e.recordEquity(candle.Timestamp, candle.Close, position)
			continue
		}

		if signal.Action == bot.ActionHold {
			e.recordEquity(candle.Timestamp, candle.Close, position)
			continue
		}

		side := exchange.SideBuy
		if signal.Action == bot.ActionSell {
			side = exchange.SideSell
		}

		order, err := e.exchange.PlaceOrder(ctx, exchange.OrderRequest{
			Symbol:   signal.Symbol,
			Side:     side,
			Type:     signal.Type,
			Quantity: signal.Quantity,
			Price:    signal.Price,
		})
		if err != nil {
			e.recordEquity(candle.Timestamp, candle.Close, position)
			continue
		}

		var pnl float64
		if order.FilledQuantity > 0 {
			if side == exchange.SideSell && position != nil && position.AvgEntryPrice > 0 {
				pnl = (candle.Close - position.AvgEntryPrice) * order.FilledQuantity
			}
		}

		e.trades = append(e.trades, Trade{
			Timestamp: candle.Timestamp,
			Symbol:    candle.Symbol,
			Side:      string(side),
			Price:     candle.Close,
			Quantity:  order.FilledQuantity,
			PnL:       pnl,
		})

		if confirmer, ok := e.strategy.(interface{ ConfirmOrder(float64, string) }); ok {
			confirmer.ConfirmOrder(signal.Price, order.ID)
		}
		if order.Status == exchange.StatusFilled || order.Status == exchange.StatusPartiallyFilled {
			if filler, ok := e.strategy.(interface{ OrderFilled(float64) }); ok {
				filler.OrderFilled(signal.Price)
			}
		}

		pos, _ := e.exchange.GetPosition(ctx, candle.Symbol)
		e.recordEquity(candle.Timestamp, candle.Close, pos)
		_ = tickFreq
	}

	bal, _ := e.exchange.GetBalance(ctx, "USD")
	pos, _ := e.exchange.GetPosition(ctx, e.cfg.Strategy.Symbol)
	e.finalBalance = bal.Total + pos.Quantity*e.data[len(e.data)-1].Close

	return e.buildReport(startTime, endTime), nil
}

func (e *Engine) recordEquity(ts time.Time, price float64, pos *exchange.Position) {
	bal, _ := e.exchange.GetBalance(context.Background(), "USD")
	var equity float64
	if pos != nil {
		equity = bal.Total + pos.Quantity*price
	} else {
		equity = bal.Total
	}
	e.equityCurve = append(e.equityCurve, EquityPoint{Timestamp: ts, Equity: equity})
}

func (e *Engine) buildReport(start, end time.Time) *Report {
	r := &Report{
		Symbol:         e.cfg.Strategy.Symbol,
		Strategy:       e.cfg.Strategy.Type,
		Period:         fmt.Sprintf("%s → %s", start.Format("2006-01-02"), end.Format("2006-01-02")),
		InitialBalance: e.initialBalance,
		FinalBalance:   e.finalBalance,
		TotalReturn:    e.finalBalance - e.initialBalance,
		TotalReturnPct: ((e.finalBalance - e.initialBalance) / e.initialBalance) * 100,
		TotalTrades:    len(e.trades),
		EquityCurve:    e.equityCurve,
	}

	totalProfit := 0.0
	totalLoss := 0.0
	for _, trade := range e.trades {
		if trade.PnL > 0 {
			r.WinningTrades++
			totalProfit += trade.PnL
		} else if trade.PnL < 0 {
			r.LosingTrades++
			totalLoss += -trade.PnL
		}
	}
	if r.TotalTrades > 0 {
		r.WinRate = float64(r.WinningTrades) / float64(r.TotalTrades) * 100
	}
	if totalLoss > 0 {
		r.ProfitFactor = totalProfit / totalLoss
	} else if totalProfit > 0 {
		r.ProfitFactor = 999
	}

	peak := e.initialBalance
	maxDD := 0.0
	for _, pt := range e.equityCurve {
		if pt.Equity > peak {
			peak = pt.Equity
		}
		dd := (peak - pt.Equity) / peak * 100
		if dd > maxDD {
			maxDD = dd
		}
	}
	r.MaxDrawdownPct = maxDD

	if len(e.equityCurve) > 1 {
		returns := make([]float64, len(e.equityCurve)-1)
		for i := 1; i < len(e.equityCurve); i++ {
			returns[i-1] = (e.equityCurve[i].Equity - e.equityCurve[i-1].Equity) / e.equityCurve[i-1].Equity
		}
		meanRet := 0.0
		for _, ret := range returns {
			meanRet += ret
		}
		meanRet /= float64(len(returns))
		stdDev := 0.0
		for _, ret := range returns {
			stdDev += (ret - meanRet) * (ret - meanRet)
		}
		stdDev /= float64(len(returns))
		if stdDev > 0 {
			r.SharpeRatio = meanRet / stdDev
		}
	}

	return r
}
