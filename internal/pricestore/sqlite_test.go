package pricestore_test

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/infrastructure/db"
	"github.com/antonygiomarxdev/greedy/internal/pricestore"
)

func setupDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.RunMigrations(database); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return database
}

func TestInsertAndQuery(t *testing.T) {
	database := setupDB(t)
	store := pricestore.NewSQLitePriceStore(database)

	ctx := context.Background()
	now := time.Now().Truncate(time.Millisecond)

	for i := range 100 {
		ts := now.Add(time.Duration(i) * time.Second)
		err := store.Insert(ctx, pricestore.PricePoint{
			Symbol:    "BTC-USD",
			Price:     50000 + float64(i),
			Timestamp: ts,
		})
		if err != nil {
			t.Fatalf("Insert #%d: %v", i, err)
		}
	}

	points, err := store.QueryWindow(ctx, "BTC-USD", now, now.Add(99*time.Second))
	if err != nil {
		t.Fatalf("QueryWindow: %v", err)
	}

	if len(points) != 100 {
		t.Errorf("expected 100 points, got %d", len(points))
	}

	if points[0].Price != 50000 {
		t.Errorf("first price = %.0f, want 50000", points[0].Price)
	}
	if points[99].Price != 50099 {
		t.Errorf("last price = %.0f, want 50099", points[99].Price)
	}

	for i, p := range points {
		expected := now.Add(time.Duration(i) * time.Second)
		if !p.Timestamp.Equal(expected) {
			t.Errorf("point[%d].Timestamp = %v, want %v", i, p.Timestamp, expected)
		}
	}
}

func TestQueryEmptySymbol(t *testing.T) {
	database := setupDB(t)
	store := pricestore.NewSQLitePriceStore(database)

	ctx := context.Background()
	now := time.Now().Truncate(time.Millisecond)

	points, err := store.QueryWindow(ctx, "ETH-USD", now.Add(-1*time.Hour), now)
	if err != nil {
		t.Fatalf("QueryWindow: %v", err)
	}

	if len(points) != 0 {
		t.Errorf("expected 0 points, got %d", len(points))
	}
}

func TestPrune(t *testing.T) {
	database := setupDB(t)
	store := pricestore.NewSQLitePriceStore(database)

	ctx := context.Background()

	old := time.Now().Add(-2 * time.Hour)
	if err := store.Insert(ctx, pricestore.PricePoint{
		Symbol: "BTC-USD", Price: 40000, Timestamp: old,
	}); err != nil {
		t.Fatalf("Insert old: %v", err)
	}

	recent := time.Now()
	if err := store.Insert(ctx, pricestore.PricePoint{
		Symbol: "BTC-USD", Price: 50000, Timestamp: recent,
	}); err != nil {
		t.Fatalf("Insert recent: %v", err)
	}

	n, err := store.Prune(ctx, 1*time.Hour)
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}

	if n != 1 {
		t.Errorf("expected 1 pruned, got %d", n)
	}

	points, err := store.QueryWindow(ctx, "BTC-USD", time.Time{}, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("QueryWindow: %v", err)
	}

	if len(points) != 1 {
		t.Errorf("expected 1 point remaining, got %d", len(points))
	}
	if points[0].Price != 50000 {
		t.Errorf("remaining price = %.0f, want 50000", points[0].Price)
	}
}

func TestConcurrentInsertAndQuery(t *testing.T) {
	database := setupDB(t)
	store := pricestore.NewSQLitePriceStore(database)

	ctx := context.Background()
	now := time.Now().Truncate(time.Millisecond)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for i := range 50 {
			_ = store.Insert(ctx, pricestore.PricePoint{
				Symbol: "BTC-USD", Price: float64(i), Timestamp: now,
			})
		}
	}()

	for range 50 {
		_, err := store.QueryWindow(ctx, "BTC-USD", now.Add(-time.Hour), now.Add(time.Hour))
		if err != nil {
			t.Errorf("QueryWindow during concurrent Insert: %v", err)
		}
	}

	<-done
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
