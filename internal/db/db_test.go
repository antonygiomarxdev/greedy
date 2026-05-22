package db

import (
	"testing"
	"time"
)

func TestOpenClose(t *testing.T) {
	dir := t.TempDir()
	database, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := RunMigrations(database); err != nil {
		t.Fatal(err)
	}

	Close(database)
}

func TestOpen_InvalidDir(t *testing.T) {
	_, err := Open("/nonexistent/dir/should/fail")
	if err == nil {
		t.Fatal("expected error for invalid directory")
	}
}

func TestRunMigrations_EmptyDB(t *testing.T) {
	dir := t.TempDir()
	database, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer Close(database)

	if err := RunMigrations(database); err != nil {
		t.Fatal(err)
	}
	if err := RunMigrations(database); err != nil {
		t.Fatal(err)
	}
}

func TestBotRepository_InsertGet(t *testing.T) {
	dir := t.TempDir()
	database, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer Close(database)

	if err := RunMigrations(database); err != nil {
		t.Fatal(err)
	}

	repo := NewBotRepository(database)
	now := time.Now().UTC()

	bot := BotRecord{
		ID:         "bot-1",
		Name:       "Test Bot",
		Strategy:   "dca",
		Symbol:     "BTC-USD",
		ConfigYAML: "strategy: dca",
		Status:     "running",
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := repo.Insert(bot); err != nil {
		t.Fatal(err)
	}

	// Get
	found, err := repo.Get("bot-1")
	if err != nil {
		t.Fatal(err)
	}
	if found == nil {
		t.Fatal("expected bot to exist")
	}
	if found.Status != "running" {
		t.Fatalf("expected running, got %s", found.Status)
	}

	// Update status
	if err := repo.UpdateStatus("bot-1", "stopped"); err != nil {
		t.Fatal(err)
	}

	found, _ = repo.Get("bot-1")
	if found.Status != "stopped" {
		t.Fatalf("expected stopped, got %s", found.Status)
	}
}

func TestBotRepository_Get_NotFound(t *testing.T) {
	dir := t.TempDir()
	database, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer Close(database)

	if err := RunMigrations(database); err != nil {
		t.Fatal(err)
	}

	repo := NewBotRepository(database)
	found, err := repo.Get("nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if found != nil {
		t.Fatal("expected nil for nonexistent bot")
	}
}

func TestBotRepository_List(t *testing.T) {
	dir := t.TempDir()
	database, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer Close(database)

	if err := RunMigrations(database); err != nil {
		t.Fatal(err)
	}

	repo := NewBotRepository(database)
	now := time.Now().UTC()

	repo.Insert(BotRecord{ID: "bot-a", Name: "A", Strategy: "dca", Symbol: "BTC-USD", Status: "running", CreatedAt: now, UpdatedAt: now})
	repo.Insert(BotRecord{ID: "bot-b", Name: "B", Strategy: "grid", Symbol: "ETH-USD", Status: "stopped", CreatedAt: now, UpdatedAt: now})

	bots, err := repo.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(bots) != 2 {
		t.Fatalf("expected 2 bots, got %d", len(bots))
	}
}

func TestBotRepository_Delete(t *testing.T) {
	dir := t.TempDir()
	database, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer Close(database)

	if err := RunMigrations(database); err != nil {
		t.Fatal(err)
	}

	repo := NewBotRepository(database)
	now := time.Now().UTC()

	repo.Insert(BotRecord{ID: "del-me", Name: "D", Strategy: "signal", Symbol: "SOL-USD", Status: "paused", CreatedAt: now, UpdatedAt: now})

	if err := repo.Delete("del-me"); err != nil {
		t.Fatal(err)
	}

	found, _ := repo.Get("del-me")
	if found != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestConfigRepository_SetGetDelete(t *testing.T) {
	dir := t.TempDir()
	database, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer Close(database)

	if err := RunMigrations(database); err != nil {
		t.Fatal(err)
	}

	repo := NewConfigRepository(database)

	if err := repo.Set("api_key", []byte("secret-value")); err != nil {
		t.Fatal(err)
	}

	val, err := repo.Get("api_key")
	if err != nil {
		t.Fatal(err)
	}
	if val == nil {
		t.Fatal("expected value")
	}
	if string(val) != "secret-value" {
		t.Fatalf("expected secret-value, got %s", string(val))
	}

	if err := repo.Delete("api_key"); err != nil {
		t.Fatal(err)
	}

	val, _ = repo.Get("api_key")
	if val != nil {
		t.Fatal("expected nil after delete")
	}
}

func TestConfigRepository_Upsert(t *testing.T) {
	dir := t.TempDir()
	database, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer Close(database)

	if err := RunMigrations(database); err != nil {
		t.Fatal(err)
	}

	repo := NewConfigRepository(database)

	repo.Set("key", []byte("v1"))
	repo.Set("key", []byte("v2"))

	val, _ := repo.Get("key")
	if string(val) != "v2" {
		t.Fatalf("expected v2 after upsert, got %s", string(val))
	}
}

func TestSQLiteWAL(t *testing.T) {
	dir := t.TempDir()
	database, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer Close(database)

	var journalMode string
	err = database.QueryRow("PRAGMA journal_mode").Scan(&journalMode)
	if err != nil {
		t.Fatal(err)
	}
	if journalMode != "wal" {
		t.Fatalf("expected WAL mode, got %s", journalMode)
	}
}

func TestDB_ConcurrentAccess(t *testing.T) {
	dir := t.TempDir()
	database, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer Close(database)

	if err := RunMigrations(database); err != nil {
		t.Fatal(err)
	}

	repo := NewBotRepository(database)
	now := time.Now().UTC()
	repo.Insert(BotRecord{ID: "concurrent-bot", Name: "C", Strategy: "dca", Symbol: "BTC-USD", Status: "running", CreatedAt: now, UpdatedAt: now})

	done := make(chan bool, 5)
	for i := 0; i < 5; i++ {
		go func() {
			time.Sleep(10 * time.Millisecond)
			_ = repo.UpdateStatus("concurrent-bot", "running")
			done <- true
		}()
	}

	for i := 0; i < 5; i++ {
		<-done
	}
}
