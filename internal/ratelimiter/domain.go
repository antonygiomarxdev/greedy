package ratelimiter

import (
	"context"
	"time"
)

type RateLimiter interface {
	Wait(ctx context.Context) error
}

type Config struct {
	Burst  int
	Refill time.Duration
}
