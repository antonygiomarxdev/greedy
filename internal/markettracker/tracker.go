package markettracker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/pricestore"
)

var _ MarketTracker = (*Tracker)(nil)

const defaultRingSize = 256

type pricePoint struct {
	price float64
	ts    time.Time
}

type symbolTracker struct {
	ring         []pricePoint
	head         int
	count        int
	breaker      bool
	breakerUntil time.Time
}

type Tracker struct {
	cfg     BreakerConfig
	symbols map[string]*symbolTracker
	mu      sync.Mutex
	logger  *slog.Logger
}

func New(cfg BreakerConfig) *Tracker {
	return &Tracker{
		cfg:     cfg,
		symbols: make(map[string]*symbolTracker),
		logger:  slog.Default().With("component", "markettracker"),
	}
}

func (t *Tracker) getOrCreate(symbol string) *symbolTracker {
	st, ok := t.symbols[symbol]
	if !ok {
		st = &symbolTracker{ring: make([]pricePoint, defaultRingSize)}
		t.symbols[symbol] = st
	}
	return st
}

func (t *Tracker) Record(symbol string, price float64, timestamp time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	st := t.getOrCreate(symbol)

	st.ring[st.head] = pricePoint{price: price, ts: timestamp}
	st.head = (st.head + 1) % len(st.ring)
	if st.count < len(st.ring) {
		st.count++
	}

	t.checkBreaker(symbol, st)
}

func (t *Tracker) checkBreaker(symbol string, st *symbolTracker) {
	if st.count < 2 {
		return
	}

	if st.breaker && time.Now().Before(st.breakerUntil) {
		return
	}

	if st.breaker {
		st.breaker = false
		t.logger.Info("breaker deactivated", "symbol", symbol)
		return
	}

	oldest := t.oldestPoint(st)
	newest := t.newestPoint(st)
	if oldest.price == 0 {
		return
	}

	delta := (newest.price - oldest.price) / oldest.price * 100
	if delta < 0 {
		delta = -delta
	}

	if delta >= t.cfg.MaxPriceDeltaPct && newest.ts.Sub(oldest.ts) <= t.cfg.WindowDuration {
		st.breaker = true
		st.breakerUntil = time.Now().Add(t.cfg.CooldownDuration)
		t.logger.Warn("breaker activated",
			"symbol", symbol,
			"delta_pct", delta,
			"oldest_price", oldest.price,
			"newest_price", newest.price,
		)
	}
}

func (t *Tracker) oldestPoint(st *symbolTracker) pricePoint {
	if st.count == 0 {
		return pricePoint{}
	}
	idx := st.head - st.count
	if idx < 0 {
		idx += len(st.ring)
	}
	return st.ring[idx]
}

func (t *Tracker) newestPoint(st *symbolTracker) pricePoint {
	if st.count == 0 {
		return pricePoint{}
	}
	idx := st.head - 1
	if idx < 0 {
		idx += len(st.ring)
	}
	return st.ring[idx]
}

func (t *Tracker) GetSnapshot(symbol string) MarketSnap {
	t.mu.Lock()
	defer t.mu.Unlock()

	st, ok := t.symbols[symbol]
	if !ok || st.count == 0 {
		return MarketSnap{Symbol: symbol}
	}

	oldest := t.oldestPoint(st)
	newest := t.newestPoint(st)

	var delta float64
	if oldest.price != 0 {
		delta = (newest.price - oldest.price) / oldest.price * 100
	}

	return MarketSnap{
		Symbol:        symbol,
		CurrentPrice:  newest.price,
		DeltaPct:      delta,
		BreakerActive: st.breaker && time.Now().Before(st.breakerUntil),
		BreakerUntil:  st.breakerUntil,
	}
}

func (t *Tracker) IsBreakerActive(symbol string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	st, ok := t.symbols[symbol]
	if !ok {
		return false
	}
	return st.breaker && time.Now().Before(st.breakerUntil)
}

func (t *Tracker) Restore(ctx context.Context, symbols []string, store pricestore.PriceStore) error {
	now := time.Now()
	windowStart := now.Add(-t.cfg.WindowDuration)

	for _, symbol := range symbols {
		points, err := store.QueryWindow(ctx, symbol, windowStart, now)
		if err != nil {
			t.logger.Warn("tracker restore failed for symbol", "symbol", symbol, "error", err)
			continue
		}

		t.mu.Lock()
		st := t.getOrCreate(symbol)
		for _, p := range points {
			st.ring[st.head] = pricePoint{price: p.Price, ts: p.Timestamp}
			st.head = (st.head + 1) % len(st.ring)
			if st.count < len(st.ring) {
				st.count++
			}
		}
		t.mu.Unlock()

		t.logger.Info("tracker restored",
			"symbol", symbol,
			"points", len(points),
		)
	}
	return nil
}
