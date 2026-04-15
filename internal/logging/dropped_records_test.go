package logging

import (
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// resetDroppedRecords is a helper that zeros the package counter so
// tests can make precise claims about a specific scenario rather than
// fighting with noise from earlier-running tests.
func resetDroppedRecords() {
	droppedRecords.Store(0)
}

func TestDroppedRecords_ZeroOnHappyPath(t *testing.T) {
	// Not t.Parallel(): we mutate and read a package-level atomic.
	resetDroppedRecords()

	dir := t.TempDir()
	lg, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := lg.StartIterationWithPrefix("happy"); err != nil {
		t.Fatal(err)
	}
	if err := lg.Log(map[string]any{"type": "hello"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lg.Close()

	if got := DroppedRecords(); got != 0 {
		t.Errorf("happy path should drop nothing, got %d", got)
	}
}

func TestDroppedRecords_NilReceiverIsCountedNotFatal(t *testing.T) {
	resetDroppedRecords()
	// Suppress the stderr canary line so verbose test runs stay quiet.
	devnull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = devnull.Close() }()
	oldStderr := os.Stderr
	os.Stderr = devnull
	defer func() { os.Stderr = oldStderr }()

	var lg *Logger
	if err := lg.Log(map[string]any{"type": "nil-receiver"}); err == nil {
		t.Error("expected an error from a nil-receiver log call")
	}
	if got := DroppedRecords(); got != 1 {
		t.Errorf("expected 1 drop from nil receiver, got %d", got)
	}
}

func TestDroppedRecords_NoActiveIterationIsCounted(t *testing.T) {
	resetDroppedRecords()
	devnull, _ := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	defer func() { _ = devnull.Close() }()
	oldStderr := os.Stderr
	os.Stderr = devnull
	defer func() { os.Stderr = oldStderr }()

	dir := t.TempDir()
	lg, err := NewLogger(dir)
	if err != nil {
		t.Fatal(err)
	}
	// Never call StartIteration. Log should drop and count.
	_ = lg.Log(map[string]any{"type": "no-file"})
	if got := DroppedRecords(); got != 1 {
		t.Errorf("expected 1 drop without StartIteration, got %d", got)
	}

	// Start an iteration, write one record, close, write again: the
	// post-close write should be dropped and counted.
	_ = lg.StartIterationWithPrefix("late")
	if err := lg.Log(map[string]any{"type": "ok"}); err != nil {
		t.Fatal(err)
	}
	lg.Close()
	_ = lg.Log(map[string]any{"type": "after-close"})

	if got := DroppedRecords(); got != 2 {
		t.Errorf("expected 2 drops total, got %d", got)
	}

	// Sanity: the happy record did write a file.
	entries, _ := os.ReadDir(dir)
	if len(entries) == 0 {
		t.Error("expected the one successful log to produce a file")
	}
	_ = filepath.Join // keep path/filepath import alive without a matcher
	_ = atomic.Uint64{}
}
