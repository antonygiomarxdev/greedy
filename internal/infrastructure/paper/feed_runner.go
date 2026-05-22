package paper

import "context"

type feedRunner interface {
	run(ctx context.Context, f *PriceFeed)
}
