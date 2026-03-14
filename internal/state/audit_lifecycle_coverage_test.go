package state

import (
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/clock"
)

func TestResolveOptionalClock_WithNilClock(t *testing.T) {
	t.Parallel()
	clk := resolveOptionalClock([]clock.Clock{nil})
	if clk == nil {
		t.Error("expected non-nil clock when passed nil clock")
	}
}

func TestResolveOptionalClock_WithNoClocksProvided(t *testing.T) {
	t.Parallel()
	clk := resolveOptionalClock(nil)
	if clk == nil {
		t.Error("expected non-nil default clock")
	}
	// Should return a real clock
	now := clk.Now()
	if now.IsZero() {
		t.Error("real clock should return non-zero time")
	}
}

func TestResolveOptionalClock_WithEmptySlice(t *testing.T) {
	t.Parallel()
	clk := resolveOptionalClock([]clock.Clock{})
	if clk == nil {
		t.Error("expected non-nil default clock")
	}
}

func TestResolveOptionalClock_WithValidClock(t *testing.T) {
	t.Parallel()
	fixed := clock.NewFixed(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	clk := resolveOptionalClock([]clock.Clock{fixed})
	if clk != fixed {
		t.Error("expected the provided clock to be returned")
	}
}
