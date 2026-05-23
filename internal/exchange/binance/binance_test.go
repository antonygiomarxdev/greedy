package binance

import (
	"testing"

	"github.com/antonygiomarxdev/greedy/internal/shared"
)

func TestConnectorName(t *testing.T) {
	c := New(Config{})
	if c.Name() != string(shared.ProviderBinance) {
		t.Errorf("Name = %s, want %s", c.Name(), string(shared.ProviderBinance))
	}
}

func TestNewDefaults(t *testing.T) {
	c := New(Config{})
	if c.cfg.RESTBaseURL != RESTURL {
		t.Errorf("RESTBaseURL = %s, want %s", c.cfg.RESTBaseURL, RESTURL)
	}
}

func TestNewTestnet(t *testing.T) {
	c := New(Config{RESTBaseURL: TestnetRESTURL})
	if c.cfg.RESTBaseURL != TestnetRESTURL {
		t.Errorf("RESTBaseURL = %s, want %s", c.cfg.RESTBaseURL, TestnetRESTURL)
	}
}

func TestValidSymbol(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"BTCUSDT", true},
		{"ETHUSDT", true},
		{"SOLUSDT", true},
		{"ADABTC", true},
		{"1000PEPEUSDT", true},
		{"btcusdt", false},
		{"BTC-USD", false},
		{"BTC", false},
		{"", false},
	}
	for _, tt := range tests {
		got := validSymbol(tt.in)
		if got != tt.want {
			t.Errorf("validSymbol(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestConvertInterval(t *testing.T) {
	tests := []struct {
		in   shared.CandleInterval
		want string
	}{
		{shared.Interval1m, "1m"},
		{shared.Interval5m, "5m"},
		{shared.Interval15m, "15m"},
		{shared.Interval1h, "1h"},
		{shared.Interval4h, "4h"},
		{shared.Interval1d, "1d"},
	}
	for _, tt := range tests {
		got := convertInterval(tt.in)
		if got != tt.want {
			t.Errorf("convertInterval(%s) = %s, want %s", tt.in, got, tt.want)
		}
	}
}

func TestConvertStatus(t *testing.T) {
	tests := []struct {
		in   string
		want shared.OrderStatus
	}{
		{"NEW", shared.StatusOpen},
		{"PARTIALLY_FILLED", shared.StatusOpen},
		{"FILLED", shared.StatusFilled},
		{"CANCELED", shared.StatusCancelled},
		{"PENDING_CANCEL", shared.StatusCancelled},
		{"REJECTED", shared.StatusRejected},
		{"EXPIRED", shared.StatusRejected},
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
	sig1 := c.sign("symbol=BTCUSDT&timestamp=1234567890")
	sig2 := c.sign("symbol=BTCUSDT&timestamp=1234567890")
	if sig1 != sig2 {
		t.Error("sign should be deterministic for same inputs")
	}
	if sig1 == "" {
		t.Error("sign should produce output")
	}
}

func TestSignDifferentInputs(t *testing.T) {
	c := New(Config{APIKey: "key", APISecret: "secret"})
	sig1 := c.sign("symbol=BTCUSDT&timestamp=1234567890")
	sig2 := c.sign("symbol=BTCUSDT&timestamp=1234567891")
	if sig1 == sig2 {
		t.Error("different timestamps should produce different signatures")
	}
}

func TestEncodeParams(t *testing.T) {
	params := map[string]string{
		"symbol":    "BTCUSDT",
		"timestamp": "1234567890",
	}
	got := encodeParams(params)
	want := "symbol=BTCUSDT&timestamp=1234567890"
	if got != want {
		t.Errorf("encodeParams = %q, want %q", got, want)
	}
}

func TestEncodeParamsSorted(t *testing.T) {
	params := map[string]string{
		"z": "last",
		"a": "first",
	}
	got := encodeParams(params)
	want := "a=first&z=last"
	if got != want {
		t.Errorf("encodeParams = %q, want %q", got, want)
	}
}

func TestCheckError(t *testing.T) {
	err := checkError([]byte(`{"code":-2011,"msg":"Unknown order sent."}`), 400)
	if err == nil {
		t.Error("expected error for 400")
	}
}

func TestCheckErrorOK(t *testing.T) {
	err := checkError([]byte(`{"symbol":"BTCUSDT","price":"50000.00"}`), 200)
	if err != nil {
		t.Errorf("expected nil for 200, got %v", err)
	}
}
