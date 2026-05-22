package markettracker_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/infrastructure/db"
	"github.com/antonygiomarxdev/greedy/internal/markettracker"
	"github.com/antonygiomarxdev/greedy/internal/pricestore"
)

func TestRingBuffer(t *testing.T) {
	tr := markettracker.New(markettracker.BreakerConfig{
		MaxPriceDeltaPct: 5.0,
		WindowDuration:   30 * time.Second,
		CooldownDuration: 60 * time.Second,
	})

	now := time.Now()
	for i := range 300 {
		tr.Record("BTC-USD", 50000+float64(i)*0.01, now.Add(time.Duration(i)*time.Second))
	}

	snap := tr.GetSnapshot("BTC-USD")
	if snap.CurrentPrice == 0 {
		t.Error("current price should not be zero")
	}
}

func TestBreakerActivation(t *testing.T) {
	tr := markettracker.New(markettracker.BreakerConfig{
		MaxPriceDeltaPct: 1.0,
		WindowDuration:   10 * time.Second,
		CooldownDuration: 2 * time.Second,
	})

	now := time.Now()
	tr.Record("BTC-USD", 50000, now)
	tr.Record("BTC-USD", 51000, now.Add(1*time.Second))

	if !tr.IsBreakerActive("BTC-USD") {
		t.Error("breaker should be active after 2% price move")
	}

	snap := tr.GetSnapshot("BTC-USD")
	if !snap.BreakerActive {
		t.Error("snapshot should show breaker active")
	}
}

func TestBreakerInactiveWithinThreshold(t *testing.T) {
	tr := markettracker.New(markettracker.BreakerConfig{
		MaxPriceDeltaPct: 5.0,
		WindowDuration:   10 * time.Second,
		CooldownDuration: 60 * time.Second,
	})

	now := time.Now()
	tr.Record("BTC-USD", 50000, now)
	tr.Record("BTC-USD", 50500, now.Add(1*time.Second))

	if tr.IsBreakerActive("BTC-USD") {
		t.Error("breaker should not be active for 1% move")
	}
}

func TestBreakerCooldownExpires(t *testing.T) {
	tr := markettracker.New(markettracker.BreakerConfig{
		MaxPriceDeltaPct: 1.0,
		WindowDuration:   10 * time.Second,
		CooldownDuration: 200 * time.Millisecond,
	})

	now := time.Now()
	tr.Record("BTC-USD", 50000, now)
	tr.Record("BTC-USD", 51000, now.Add(1*time.Second))

	if !tr.IsBreakerActive("BTC-USD") {
		t.Fatal("breaker should be active initially")
	}

	time.Sleep(300 * time.Millisecond)

	if tr.IsBreakerActive("BTC-USD") {
		t.Error("breaker should be inactive after cooldown expires")
	}
}

func TestGetSnapshotUnknownSymbol(t *testing.T) {
	tr := markettracker.New(markettracker.BreakerConfig{
		MaxPriceDeltaPct: 5.0,
		WindowDuration:   30 * time.Second,
		CooldownDuration: 60 * time.Second,
	})

	snap := tr.GetSnapshot("UNKNOWN")
	if snap.Symbol != "UNKNOWN" {
		t.Errorf("expected UNKNOWN symbol, got %s", snap.Symbol)
	}
	if snap.CurrentPrice != 0 {
		t.Error("unknown symbol should have zero price")
	}
}

func TestRestore(t *testing.T) {
	dataDir := t.TempDir()
	database, err := db.Open(dataDir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()
	if err := db.RunMigrations(database); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	store := pricestore.NewSQLitePriceStore(database)
	ctx := context.Background()
	now := time.Now()

	for i := range 50 {
		ts := now.Add(time.Duration(i-50) * time.Second)
		_ = store.Insert(ctx, pricestore.PricePoint{
			Symbol:    "ETH-USD",
			Price:     3000 + float64(i),
			Timestamp: ts,
		})
	}

	tr := markettracker.New(markettracker.BreakerConfig{
		MaxPriceDeltaPct: 5.0,
		WindowDuration:   60 * time.Second,
		CooldownDuration: 60 * time.Second,
	})

	if err := tr.Restore(ctx, []string{"ETH-USD"}, store); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	snap := tr.GetSnapshot("ETH-USD")
	if snap.CurrentPrice == 0 {
		t.Error("restored tracker should have a current price")
	}
}

func TestRestoreNoData(t *testing.T) {
	dataDir := t.TempDir()
	database, err := db.Open(dataDir)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer database.Close()
	if err := db.RunMigrations(database); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	store := pricestore.NewSQLitePriceStore(database)
	ctx := context.Background()

	tr := markettracker.New(markettracker.BreakerConfig{
		MaxPriceDeltaPct: 5.0,
		WindowDuration:   60 * time.Second,
		CooldownDuration: 60 * time.Second,
	})

	if err := tr.Restore(ctx, []string{"SOL-USD"}, store); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	snap := tr.GetSnapshot("SOL-USD")
	if snap.CurrentPrice != 0 {
		t.Error("empty restore should have zero price")
	}
}

func TestConcurrentRecordAndSnapshot(t *testing.T) {
	tr := markettracker.New(markettracker.BreakerConfig{
		MaxPriceDeltaPct: 5.0,
		WindowDuration:   30 * time.Second,
		CooldownDuration: 60 * time.Second,
	})

	now := time.Now()
	done := make(chan struct{})

	go func() {
		defer close(done)
		for i := range 100 {
			tr.Record("BTC-USD", 50000+float64(i)*0.1, now.Add(time.Duration(i)*time.Second))
		}
	}()

	for range 50 {
		_ = tr.GetSnapshot("BTC-USD")
		_ = tr.IsBreakerActive("BTC-USD")
	}

	<-done
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
