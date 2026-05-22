package paper

import (
	"context"
	"encoding/csv"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"
)

type FeedMode int

const (
	FeedStatic FeedMode = iota
	FeedRandomWalk
	FeedCSVReplay
)

type PriceFeed struct {
	mu        sync.RWMutex
	mode      FeedMode
	symbol    string
	price     float64
	drift     float64
	vol       float64
	tick      time.Duration
	replay    []CandleRow
	replayIdx int
	subs      map[int]chan float64
	nextSub   int
}

type CandleRow struct {
	Timestamp int64
	Open      float64
	High      float64
	Low       float64
	Close     float64
	Volume    float64
}

func NewStaticFeed(symbol string, price float64) *PriceFeed {
	return &PriceFeed{
		mode:   FeedStatic,
		symbol: symbol,
		price:  price,
		subs:   make(map[int]chan float64),
	}
}

func NewRandomWalkFeed(symbol string, startPrice, drift, vol float64, tick time.Duration) *PriceFeed {
	return &PriceFeed{
		mode:   FeedRandomWalk,
		symbol: symbol,
		price:  startPrice,
		drift:  drift,
		vol:    vol,
		tick:   tick,
		subs:   make(map[int]chan float64),
	}
}

func NewCSVReplayFeed(symbol, csvPath string) (*PriceFeed, error) {
	/* #nosec G304 — csvPath is user-provided for backtesting */
	f, err := os.Open(csvPath)
	if err != nil {
		return nil, fmt.Errorf("open csv: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read csv: %w", err)
	}

	rows := make([]CandleRow, 0, len(records)-1)
	for i, rec := range records {
		if i == 0 { // skip header
			continue
		}
		if len(rec) < 6 {
			continue
		}
		ts, _ := strconv.ParseInt(rec[0], 10, 64)
		o, _ := strconv.ParseFloat(rec[1], 64)
		h, _ := strconv.ParseFloat(rec[2], 64)
		l, _ := strconv.ParseFloat(rec[3], 64)
		c, _ := strconv.ParseFloat(rec[4], 64)
		v, _ := strconv.ParseFloat(rec[5], 64)
		rows = append(rows, CandleRow{
			Timestamp: ts,
			Open:      o,
			High:      h,
			Low:       l,
			Close:     c,
			Volume:    v,
		})
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("no data rows in csv")
	}

	return &PriceFeed{
		mode:   FeedCSVReplay,
		symbol: symbol,
		price:  rows[0].Close,
		replay: rows,
		subs:   make(map[int]chan float64),
	}, nil
}

func (f *PriceFeed) Price() float64 {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.price
}

func (f *PriceFeed) SetPrice(p float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.price = p
}

func (f *PriceFeed) Subscribe() (int, <-chan float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.nextSub
	f.nextSub++
	ch := make(chan float64, 16)
	f.subs[id] = ch
	return id, ch
}

func (f *PriceFeed) Unsubscribe(id int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if ch, ok := f.subs[id]; ok {
		close(ch)
		delete(f.subs, id)
	}
}

func (f *PriceFeed) broadcast(p float64) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, ch := range f.subs {
		select {
		case ch <- p:
		default:
		}
	}
}

func (f *PriceFeed) Run(ctx context.Context) {
	switch f.mode {
	case FeedStatic:
		<-ctx.Done()
	case FeedRandomWalk:
		f.runRandomWalk(ctx)
	case FeedCSVReplay:
		f.runCSVReplay(ctx)
	}
}

func (f *PriceFeed) runRandomWalk(ctx context.Context) {
	ticker := time.NewTicker(f.tick)
	defer ticker.Stop()

	rng := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404 — weak RNG is acceptable for paper trading sim

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			dt := f.tick.Seconds() / (365 * 24 * 3600)
			shock := rng.NormFloat64() * f.vol * math.Sqrt(dt)
			newPrice := f.Price() * math.Exp((f.drift-0.5*f.vol*f.vol)*dt+shock)
			if newPrice <= 0 {
				newPrice = 0.01
			}
			f.SetPrice(newPrice)
			f.broadcast(newPrice)
		}
	}
}

func (f *PriceFeed) runCSVReplay(ctx context.Context) {
	if len(f.replay) == 0 {
		<-ctx.Done()
		return
	}

	ticker := time.NewTicker(f.tick)
	if f.tick == 0 {
		f.tick = 1 * time.Second
		ticker = time.NewTicker(f.tick)
	}
	defer ticker.Stop()

	for f.replayIdx < len(f.replay) {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			row := f.replay[f.replayIdx]
			f.SetPrice(row.Close)
			f.broadcast(row.Close)
			f.replayIdx++
		}
	}
}
