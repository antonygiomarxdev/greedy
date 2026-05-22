package pricestreamer

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/pricestore"
	"github.com/antonygiomarxdev/greedy/internal/shared"
)

type symbolState struct {
	refCount int
	cancel   context.CancelFunc
	cache    CachedTicker
	mu       sync.RWMutex
}

type Streamer struct {
	exchange   shared.Exchange
	priceStore pricestore.PriceStore
	onTick     func(string, float64, time.Time)
	symbols    map[string]*symbolState
	mu         sync.RWMutex
	logger     *slog.Logger
}

func New(exchange shared.Exchange) *Streamer {
	return &Streamer{
		exchange: exchange,
		symbols:  make(map[string]*symbolState),
		logger:   slog.Default().With("component", "pricestreamer"),
	}
}

func (s *Streamer) SetPriceStore(store pricestore.PriceStore) {
	s.priceStore = store
}

func (s *Streamer) OnTick(fn func(symbol string, price float64, ts time.Time)) {
	s.onTick = fn
}

func (s *Streamer) Register(ctx context.Context, symbol string, interval time.Duration) error {
	s.mu.Lock()
	state, exists := s.symbols[symbol]
	if exists {
		state.refCount++
		s.mu.Unlock()
		s.logger.Info("streamer refCount incremented", "symbol", symbol, "refCount", state.refCount)
		return nil
	}

	fetchCtx, cancel := context.WithCancel(ctx)
	state = &symbolState{
		refCount: 1,
		cancel:   cancel,
	}
	s.symbols[symbol] = state
	s.mu.Unlock()

	go s.fetchLoop(fetchCtx, symbol, interval, state)

	s.logger.Info("streamer registered symbol", "symbol", symbol, "interval", interval)
	return nil
}

func (s *Streamer) Unregister(symbol string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, ok := s.symbols[symbol]
	if !ok {
		return
	}

	state.refCount--
	if state.refCount > 0 {
		s.logger.Info("streamer refCount decremented", "symbol", symbol, "refCount", state.refCount)
		return
	}

	state.cancel()
	delete(s.symbols, symbol)
	s.logger.Info("streamer unregistered symbol", "symbol", symbol)
}

func (s *Streamer) GetCached(symbol string) (CachedTicker, bool) {
	s.mu.RLock()
	state, ok := s.symbols[symbol]
	s.mu.RUnlock()
	if !ok {
		return CachedTicker{}, false
	}

	state.mu.RLock()
	defer state.mu.RUnlock()
	return state.cache, true
}

func (s *Streamer) ActiveSymbols() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	symbols := make([]string, 0, len(s.symbols))
	for sym := range s.symbols {
		symbols = append(symbols, sym)
	}
	return symbols
}

func (s *Streamer) fetchLoop(ctx context.Context, symbol string, interval time.Duration, state *symbolState) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	markStale := func() {
		state.mu.Lock()
		state.cache.Stale = true
		state.mu.Unlock()
	}

	fetch := func() {
		t, err := s.exchange.GetTicker(ctx, symbol)
		if err != nil {
			s.logger.Warn("streamer fetch failed", "symbol", symbol, "error", err)
			markStale()
			return
		}

		state.mu.Lock()
		state.cache = CachedTicker{
			Symbol:    symbol,
			Price:     t.Price,
			Bid:       t.Price * 0.999,
			Ask:       t.Price * 1.001,
			Timestamp: t.Time,
			Stale:     false,
		}
		state.mu.Unlock()

		if s.priceStore != nil {
			if err := s.priceStore.Insert(ctx, pricestore.PricePoint{
				Symbol:    symbol,
				Price:     t.Price,
				Timestamp: t.Time,
			}); err != nil {
				s.logger.Warn("streamer price store insert failed", "symbol", symbol, "error", err)
			}
		}

		if s.onTick != nil {
			s.onTick(symbol, t.Price, t.Time)
		}
	}

	fetch()

	for {
		select {
		case <-ctx.Done():
			markStale()
			return
		case <-ticker.C:
			fetch()
		}
	}
}
