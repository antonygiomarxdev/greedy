package debouncer_test

import (
	"os"
	"testing"
	"time"

	"github.com/antonygiomarxdev/greedy/internal/debouncer"
)

func TestCooldownBlocks(t *testing.T) {
	d := debouncer.New(200*time.Millisecond, 10, 30*time.Second)

	if !d.CanExecute() {
		t.Error("should allow first execution")
	}
	d.RecordExecution()

	if d.CanExecute() {
		t.Error("should block immediately after execution")
	}
}

func TestCooldownExpires(t *testing.T) {
	d := debouncer.New(100*time.Millisecond, 10, 30*time.Second)

	d.RecordExecution()
	time.Sleep(150 * time.Millisecond)

	if !d.CanExecute() {
		t.Error("should allow after cooldown expires")
	}
}

func TestBurstLimit(t *testing.T) {
	d := debouncer.New(0, 3, 500*time.Millisecond)

	for range 3 {
		if !d.CanExecute() {
			t.Fatal("should allow within burst limit")
		}
		d.RecordExecution()
	}

	if d.CanExecute() {
		t.Error("should block when burst limit reached")
	}
}

func TestBurstWindowCleanup(t *testing.T) {
	d := debouncer.New(0, 3, 100*time.Millisecond)

	for range 3 {
		d.RecordExecution()
	}

	time.Sleep(150 * time.Millisecond)

	if !d.CanExecute() {
		t.Error("should allow after burst window expires")
	}
}

func TestReset(t *testing.T) {
	d := debouncer.New(time.Hour, 1, time.Hour)

	d.RecordExecution()
	if d.CanExecute() {
		t.Error("should block after hitting limit")
	}

	d.Reset()
	if !d.CanExecute() {
		t.Error("should allow after reset")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
