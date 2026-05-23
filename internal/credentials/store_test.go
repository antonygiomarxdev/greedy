package credentials_test

import (
	"context"
	"os"
	"testing"

	"github.com/antonygiomarxdev/greedy/internal/credentials"
	"github.com/antonygiomarxdev/greedy/internal/crypto"
	"github.com/antonygiomarxdev/greedy/internal/infrastructure/db"
	"github.com/antonygiomarxdev/greedy/internal/shared"
)

const testPassword = "test-master-password-for-unit-tests"

func setupStore(t *testing.T) credentials.Store {
	t.Helper()
	database, err := db.Open(t.TempDir())
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.RunMigrations(database); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	return credentials.NewSQLiteStore(database)
}

func masterKey() *[crypto.KeySize]byte {
	k := crypto.DeriveKey(testPassword, nil)
	return &k
}

func TestSetAndGet(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	key := masterKey()

	err := store.Set(ctx, credentials.Credential{
		Exchange: shared.ProviderCoinbase, Label: "default", APIKey: "my-api-key", APISecret: "my-api-secret",
	}, key)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	cred, err := store.Get(ctx, shared.ProviderCoinbase, "default", key)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if cred.APIKey != "my-api-key" {
		t.Errorf("APIKey = %s, want my-api-key", cred.APIKey)
	}
	if cred.APISecret != "my-api-secret" {
		t.Errorf("APISecret = %s, want my-api-secret", cred.APISecret)
	}
}

func TestSetDuplicateFails(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	key := masterKey()

	err := store.Set(ctx, credentials.Credential{
		Exchange: shared.ProviderBinance, Label: "default", APIKey: "k1", APISecret: "s1",
	}, key)
	if err != nil {
		t.Fatalf("first Set: %v", err)
	}

	err = store.Set(ctx, credentials.Credential{
		Exchange: shared.ProviderBinance, Label: "default", APIKey: "k2", APISecret: "s2",
	}, key)
	if err != shared.ErrCredentialExists {
		t.Errorf("expected ErrCredentialExists, got %v", err)
	}
}

func TestGetMissingFails(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	key := masterKey()

	_, err := store.Get(ctx, "nonexistent", "default", key)
	if err != shared.ErrCredentialNotFound {
		t.Errorf("expected ErrCredentialNotFound, got %v", err)
	}
}

func TestWrongKeyFails(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	key := masterKey()

	err := store.Set(ctx, credentials.Credential{
		Exchange: shared.ProviderCoinbase, Label: "default", APIKey: "secured", APISecret: "secret",
	}, key)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	wrongKey := crypto.DeriveKey("wrong-password", nil)
	_, err = store.Get(ctx, shared.ProviderCoinbase, "default", &wrongKey)
	if err == nil {
		t.Error("expected decryption failure with wrong key")
	}
}

func TestListAndDelete(t *testing.T) {
	store := setupStore(t)
	ctx := context.Background()
	key := masterKey()

	for _, exch := range []shared.ExchangeProvider{shared.ProviderCoinbase, shared.ProviderBinance} {
		err := store.Set(ctx, credentials.Credential{
			Exchange: exch, Label: "default", APIKey: "key-" + string(exch), APISecret: "secret-" + string(exch),
		}, key)
		if err != nil {
			t.Fatalf("Set %s: %v", exch, err)
		}
	}

	metas, err := store.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(metas) != 2 {
		t.Errorf("expected 2 credentials, got %d", len(metas))
	}

	err = store.Delete(ctx, shared.ProviderCoinbase, "default")
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	metas, err = store.List(ctx)
	if err != nil {
		t.Fatalf("List after delete: %v", err)
	}
	if len(metas) != 1 || metas[0].Exchange != shared.ProviderBinance {
		t.Errorf("expected only binance, got %v", metas)
	}

	err = store.Delete(ctx, shared.ProviderCoinbase, "default")
	if err != shared.ErrCredentialNotFound {
		t.Errorf("expected ErrCredentialNotFound on double delete, got %v", err)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
