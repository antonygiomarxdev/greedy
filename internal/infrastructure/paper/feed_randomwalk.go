package paper

import (
	"context"
	"math"
	"math/rand"
	"time"
)

type randomWalkRunner struct{}

func (r *randomWalkRunner) run(ctx context.Context, f *PriceFeed) {
	ticker := time.NewTicker(f.tick)
	defer ticker.Stop()

	rng := rand.New(rand.NewSource(time.Now().UnixNano())) // #nosec G404 — weak RNG acceptable for paper sim

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
