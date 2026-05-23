package coinbase

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/shared"
)

func TestConnectorName(t *testing.T) {
	c := New(Config{})
	if c.Name() != string(shared.ProviderCoinbase) {
		t.Errorf("Name = %s, want %s", c.Name(), string(shared.ProviderCoinbase))
	}
}

func TestNewDefaults(t *testing.T) {
	c := New(Config{})
	if c.cfg.RESTBaseURL != SandboxRESTURL {
		t.Errorf("RESTBaseURL = %s, want %s", c.cfg.RESTBaseURL, SandboxRESTURL)
	}
	if c.burst != 30 {
		t.Errorf("burst = %d, want 30", c.burst)
	}
}

func TestConvertGranularity(t *testing.T) {
	tests := []struct {
		in   shared.CandleInterval
		want string
	}{
		{shared.Interval1m, "ONE_MINUTE"},
		{shared.Interval5m, "FIVE_MINUTE"},
		{shared.Interval15m, "FIFTEEN_MINUTE"},
		{shared.Interval1h, "ONE_HOUR"},
		{shared.Interval4h, "FOUR_HOUR"},
		{shared.Interval1d, "ONE_DAY"},
	}
	for _, tt := range tests {
		got := convertGranularity(tt.in)
		if got != tt.want {
			t.Errorf("convertGranularity(%s) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestConvertStatus(t *testing.T) {
	tests := []struct {
		in   string
		want shared.OrderStatus
	}{
		{"OPEN", shared.StatusOpen},
		{"PENDING", shared.StatusOpen},
		{"FILLED", shared.StatusFilled},
		{"CANCELLED", shared.StatusCancelled},
		{"EXPIRED", shared.StatusRejected},
		{"REJECTED", shared.StatusRejected},
	}
	for _, tt := range tests {
		got := convertStatus(tt.in)
		if got != tt.want {
			t.Errorf("convertStatus(%s) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestSignIdempotent(t *testing.T) {
	c := New(Config{APIKey: "key", APISecret: "secret"})
	sig1 := c.sign("GET", "/api/v3/brokerage/time", "123456", nil)
	sig2 := c.sign("GET", "/api/v3/brokerage/time", "123456", nil)
	if sig1 != sig2 {
		t.Error("sign should be deterministic for same inputs")
	}
	if sig1 == "" {
		t.Error("sign should produce output")
	}
}

func TestSignDifferentInputs(t *testing.T) {
	c := New(Config{APIKey: "key", APISecret: "secret"})
	sig1 := c.sign("GET", "/api/v3/brokerage/time", "123456", nil)
	sig2 := c.sign("GET", "/api/v3/brokerage/time", "123457", nil)
	if sig1 == sig2 {
		t.Error("different timestamps should produce different signatures")
	}
}

func TestRateLimiterDoesNotBlockOnFirstCall(t *testing.T) {
	c := New(Config{APIKey: "key", APISecret: "secret"})
	for i := 0; i < c.burst; i++ {
		if err := c.waitToken(nil); err != nil {
			t.Fatalf("token %d: %v", i, err)
		}
	}
}

func TestWaitTokenWithCancelledContext(t *testing.T) {
	c := New(Config{APIKey: "key", APISecret: "secret"})
	c.tokens = 0
	c.lastRefill = time.Now()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.waitToken(ctx); err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestSignatureVector(t *testing.T) {
	c := New(Config{APIKey: "test-key", APISecret: "dGVzdC1zZWNyZXQ="})
	ts := "1234567890"
	method := "GET"
	path := "/api/v3/brokerage/accounts"
	sig := c.sign(method, path, ts, nil)
	if sig == "" {
		t.Error("expected non-empty signature")
	}
	expectLength := 64
	if len(sig) != expectLength {
		t.Errorf("signature length = %d, want %d", len(sig), expectLength)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
