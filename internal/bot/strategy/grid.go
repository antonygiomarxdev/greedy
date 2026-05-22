package strategy

import (
	"context"
	"math"
	"sync"

	"github.com/antonygiomarxdev/greedy/internal/bot"
	"github.com/antonygiomarxdev/greedy/internal/config"
	"github.com/antonygiomarxdev/greedy/internal/exchange"
)

type GRID struct {
	cfg config.GridConfig

	mu sync.Mutex

	gridLevels    []float64
	placedOrders  map[float64]string
	pendingOrders map[float64]bool
}

func NewGRID(cfg config.GridConfig) *GRID {
	return &GRID{
		cfg:           cfg,
		placedOrders:  make(map[float64]string),
		pendingOrders: make(map[float64]bool),
	}
}

func (g *GRID) Name() string { return "grid" }

func (g *GRID) Evaluate(ctx context.Context, state *bot.BotState) (*bot.Signal, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	price := state.Ticker.Price
	if price <= 0 {
		return &bot.Signal{Action: bot.ActionHold}, nil
	}

	if len(g.gridLevels) == 0 {
		g.buildGrid(price)
	}

	for _, o := range state.OpenOrders {
		_ = o
	}

	for _, level := range g.gridLevels {
		if g.pendingOrders[level] {
			continue
		}
		if _, placed := g.placedOrders[level]; placed {
			continue
		}

		g.pendingOrders[level] = true
		return &bot.Signal{
			Action:   bot.ActionBuy,
			Symbol:   state.Symbol,
			Quantity: g.cfg.OrderSize / level,
			Price:    level,
			Type:     exchange.TypeLimit,
		}, nil
	}

	return &bot.Signal{Action: bot.ActionHold}, nil
}

func (g *GRID) ConfirmOrder(price float64, orderID string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.placedOrders[price] = orderID
	delete(g.pendingOrders, price)
}

func (g *GRID) OrderFilled(price float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.placedOrders, price)
	delete(g.pendingOrders, price)
}

func (g *GRID) Reset() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.gridLevels = nil
	g.placedOrders = make(map[float64]string)
	g.pendingOrders = make(map[float64]bool)
}

func (g *GRID) buildGrid(currentPrice float64) {
	spread := (g.cfg.UpperBound - g.cfg.LowerBound) / float64(g.cfg.GridLevels-1)
	g.gridLevels = make([]float64, g.cfg.GridLevels)
	for i := 0; i < g.cfg.GridLevels; i++ {
		g.gridLevels[i] = roundTo(g.cfg.LowerBound+spread*float64(i), 2)
	}
}

func roundTo(v float64, decimals int) float64 {
	pow := math.Pow(10, float64(decimals))
	return math.Round(v*pow) / pow
}
