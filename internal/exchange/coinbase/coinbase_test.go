package coinbase

import (
	"context"
	"os"
	"testing"

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

func TestSignatureVector(t *testing.T) {
	c := New(Config{APIKey: "test-key", APISecret: "dGVzdC1zZWNyZXQ="})
	sig := c.sign("GET", "/api/v3/brokerage/accounts", "1234567890", nil)
	if sig == "" {
		t.Error("expected non-empty signature")
	}
	if len(sig) != 64 {
		t.Errorf("signature length = %d, want 64", len(sig))
	}
}

func TestValidSymbol(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"BTC-USD", true},
		{"ETH-USD", true},
		{"SOL-USDT", true},
		{"btc-usd", false},
		{"BTCUSD", false},
		{"../../../etc/passwd", false},
		{"BTC-USD?limit=1", false},
		{"", false},
	}
	for _, tt := range tests {
		got := validSymbol(tt.in)
		if got != tt.want {
			t.Errorf("validSymbol(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestGetTickerInvalidSymbol(t *testing.T) {
	c := New(Config{})
	_, err := c.GetTicker(context.Background(), "../../etc/passwd")
	if err == nil {
		t.Error("expected error for invalid symbol")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
