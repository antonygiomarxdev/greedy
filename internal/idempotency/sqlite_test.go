package idempotency_test

import (
	"context"
	"os"
	"testing"

	"github.com/antonygiomarxdev/greedy/internal/idempotency"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/db"
)

func setupDB(t *testing.T) *idempotency.SQLiteStore {
	t.Helper()
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.RunMigrations(database); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return idempotency.NewSQLiteStore(database)
}

func TestReserveAndConfirm(t *testing.T) {
	store := setupDB(t)
	ctx := context.Background()

	err := store.Reserve(ctx, "bot-1-1716400000000-0001", "bot-1", "BTC-USD")
	if err != nil {
		t.Fatalf("Reserve: %v", err)
	}

	err = store.Confirm(ctx, "bot-1-1716400000000-0001", "order-abc123")
	if err != nil {
		t.Fatalf("Confirm: %v", err)
	}

	rec, err := store.Lookup(ctx, "bot-1-1716400000000-0001")
	if err != nil {
		t.Fatalf("Lookup: %v", err)
	}
	if rec.ExchangeOrderID != "order-abc123" {
		t.Errorf("ExchangeOrderID = %s, want order-abc123", rec.ExchangeOrderID)
	}
	if rec.Status != "confirmed" {
		t.Errorf("Status = %s, want confirmed", rec.Status)
	}
}

func TestReserveDuplicateKey(t *testing.T) {
	store := setupDB(t)
	ctx := context.Background()

	err := store.Reserve(ctx, "bot-1-1716400000000-0002", "bot-1", "ETH-USD")
	if err != nil {
		t.Fatalf("first Reserve: %v", err)
	}

	err = store.Reserve(ctx, "bot-1-1716400000000-0002", "bot-1", "ETH-USD")
	if err == nil {
		t.Error("duplicate Reserve should fail")
	}
}

func TestLookupMissingKey(t *testing.T) {
	store := setupDB(t)
	ctx := context.Background()

	_, err := store.Lookup(ctx, "nonexistent-key")
	if err == nil {
		t.Error("Lookup missing key should fail")
	}
}

func TestConcurrentReserve(t *testing.T) {
	store := setupDB(t)
	ctx := context.Background()

	errCh := make(chan error, 2)
	for range 2 {
		go func() {
			errCh <- store.Reserve(ctx, "concurrent-key", "bot-1", "BTC-USD")
		}()
	}

	err1 := <-errCh
	err2 := <-errCh

	successes := 0
	if err1 == nil {
		successes++
	}
	if err2 == nil {
		successes++
	}

	if successes != 1 {
		t.Errorf("expected exactly 1 successful concurrent Reserve, got %d", successes)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
