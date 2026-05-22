package paper

import "context"

type staticRunner struct{}

func (r *staticRunner) run(ctx context.Context, f *PriceFeed) {
	<-ctx.Done()
}
