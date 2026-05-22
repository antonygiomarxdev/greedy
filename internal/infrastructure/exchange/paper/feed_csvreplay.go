package paper

import (
	"context"
	"time"
)

type csvReplayRunner struct{}

func (r *csvReplayRunner) run(ctx context.Context, f *PriceFeed) {
	if len(f.replay) == 0 {
		<-ctx.Done()
		return
	}

	tick := f.tick
	if tick == 0 {
		tick = 1 * time.Second
	}
	ticker := time.NewTicker(tick)
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
