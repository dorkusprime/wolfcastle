package state

import (
	"fmt"
	"testing"
	"time"
)

func TestWithLock_AcquireFailure(t *testing.T) {
	dir := t.TempDir()

	// Hold the lock so the second WithLock attempt times out
	holder := NewFileLock(dir, DefaultLockTimeout)
	if err := holder.Acquire(); err != nil {
		t.Fatalf("holder Acquire: %v", err)
	}
	defer holder.Release()

	// Attempt WithLock with a very short timeout
	fl := NewFileLock(dir, 100*time.Millisecond)
	err := fl.WithLock(func() error {
		t.Error("fn should not be called when Acquire fails")
		return nil
	})
	if err != ErrLockTimeout {
		t.Errorf("expected ErrLockTimeout, got %v", err)
	}
}

func TestWithLock_FnError(t *testing.T) {
	dir := t.TempDir()
	fl := NewFileLock(dir, DefaultLockTimeout)

	expected := fmt.Errorf("inner error")
	err := fl.WithLock(func() error {
		return expected
	})
	if err != expected {
		t.Errorf("expected inner error, got %v", err)
	}

	// Lock should be released even on error
	fl2 := NewFileLock(dir, 200*time.Millisecond)
	if err := fl2.Acquire(); err != nil {
		t.Fatalf("could not acquire after WithLock error: %v", err)
	}
	fl2.Release()
}
