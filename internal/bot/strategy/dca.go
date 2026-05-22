package strategy

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/bot"
	"github.com/antonygiomarxdev/greedy/internal/config"
	"github.com/antonygiomarxdev/greedy/internal/exchange"
)

type DCA struct {
	cfg config.DCAConfig

	mu              sync.Mutex
	lastBuy         time.Time
	safetyTriggered map[int]bool    // deviation index → triggered
	triggerPrices   map[int]float64 // deviation index → trigger price
	initialPrice    float64
}

func NewDCA(cfg config.DCAConfig) *DCA {
	return &DCA{
		cfg:             cfg,
		safetyTriggered: make(map[int]bool),
		triggerPrices:   make(map[int]float64),
	}
}

func (d *DCA) Name() string { return "dca" }

func (d *DCA) Evaluate(ctx context.Context, state *bot.BotState) (*bot.Signal, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	price := state.Ticker.Price
	if price <= 0 {
		return &bot.Signal{Action: bot.ActionHold}, nil
	}

	if d.initialPrice == 0 {
		d.initialPrice = price
		d.computeTriggerPrices(price)
	}

	// Base order: time-based DCA
	if time.Since(d.lastBuy) >= d.cfg.Frequency {
		d.lastBuy = time.Now()
		qty := d.cfg.BaseOrderSize / price
		return &bot.Signal{
			Action:   bot.ActionBuy,
			Symbol:   state.Symbol,
			Quantity: qty,
			Type:     exchange.TypeMarket,
		}, nil
	}

	// Safety orders: price-based averaging down
	pctChange := (price - d.initialPrice) / d.initialPrice * 100

	for i, so := range d.cfg.SafetyOrders {
		if d.safetyTriggered[i] {
			continue
		}
		triggerPct := so.PriceDeviationPct
		if pctChange <= triggerPct {
			d.safetyTriggered[i] = true
			qty := (d.cfg.BaseOrderSize * so.VolumeScale) / price
			return &bot.Signal{
				Action:   bot.ActionBuy,
				Symbol:   state.Symbol,
				Quantity: math.Round(qty*1e8) / 1e8,
				Type:     exchange.TypeMarket,
			}, nil
		}
	}

	return &bot.Signal{Action: bot.ActionHold}, nil
}

func (d *DCA) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.lastBuy = time.Time{}
	d.initialPrice = 0
	for k := range d.safetyTriggered {
		delete(d.safetyTriggered, k)
	}
	for k := range d.triggerPrices {
		delete(d.triggerPrices, k)
	}
}

func (d *DCA) computeTriggerPrices(currentPrice float64) {
	for i, so := range d.cfg.SafetyOrders {
		d.triggerPrices[i] = d.initialPrice * (1 + so.PriceDeviationPct/100)
	}
}
