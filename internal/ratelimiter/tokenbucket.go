package ratelimiter

import (
	"context"
	"sync"
	"time"
)

var _ RateLimiter = (*TokenBucket)(nil)

type TokenBucket struct {
	mu         sync.Mutex
	tokens     float64
	lastRefill time.Time
	burst      int
	refillDur  time.Duration
}

func NewTokenBucket(burst int, refillDur time.Duration) *TokenBucket {
	return &TokenBucket{
		tokens:    float64(burst),
		burst:     burst,
		refillDur: refillDur,
	}
}

func (tb *TokenBucket) Wait(ctx context.Context) error {
	tb.mu.Lock()
	now := time.Now()

	elapsed := now.Sub(tb.lastRefill).Seconds()
	tb.tokens += elapsed * float64(tb.burst) / tb.refillDur.Seconds()
	if tb.tokens > float64(tb.burst) {
		tb.tokens = float64(tb.burst)
	}
	tb.lastRefill = now

	if tb.tokens >= 1 {
		tb.tokens--
		tb.mu.Unlock()
		return nil
	}
	tb.mu.Unlock()

	waitTime := tb.calculateWait()
	timer := time.NewTimer(waitTime)
	defer timer.Stop()
	select {
	case <-timer.C:
	case <-ctx.Done():
		return ctx.Err()
	}

	tb.mu.Lock()
	tb.tokens = 0
	tb.lastRefill = time.Now()
	tb.mu.Unlock()
	return nil
}

func (tb *TokenBucket) calculateWait() time.Duration {
	fraction := 1.0 / float64(tb.burst)
	return time.Duration(fraction * float64(tb.refillDur))
}
