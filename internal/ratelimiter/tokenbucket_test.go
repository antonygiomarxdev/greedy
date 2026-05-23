package ratelimiter_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/ratelimiter"
)

func TestTokenBucketBurst(t *testing.T) {
	tb := ratelimiter.NewTokenBucket(5, time.Second)
	for i := 0; i < 5; i++ {
		if err := tb.Wait(context.Background()); err != nil {
			t.Fatalf("token %d: %v", i, err)
		}
	}
}

func TestTokenBucketContextCancel(t *testing.T) {
	tb := ratelimiter.NewTokenBucket(5, time.Second)
	// drain
	for i := 0; i < 5; i++ {
		_ = tb.Wait(context.Background())
	}
	// next call should block
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := tb.Wait(ctx); err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
