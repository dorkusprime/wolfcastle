package clock

import (
	"testing"
	"time"
)

func TestRealClock_ReturnsUTC(t *testing.T) {
	t.Parallel()
	c := RealClock{}
	now := c.Now()
	if now.Location() != time.UTC {
		t.Errorf("expected UTC, got %v", now.Location())
	}
	// Should be very close to time.Now()
	diff := time.Since(now)
	if diff > time.Second {
		t.Errorf("RealClock.Now() is too far from time.Now(): %v", diff)
	}
}

func TestNew_ReturnsRealClock(t *testing.T) {
	t.Parallel()
	c := New()
	if _, ok := c.(RealClock); !ok {
		t.Errorf("expected RealClock, got %T", c)
	}
}

func TestFixedClock_ReturnsSameTime(t *testing.T) {
	t.Parallel()
	fixed := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
	c := NewFixed(fixed)

	for i := 0; i < 3; i++ {
		got := c.Now()
		if !got.Equal(fixed) {
			t.Errorf("call %d: expected %v, got %v", i, fixed, got)
		}
	}
}

func TestMockClock_Now(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := NewMock(start)

	if !c.Now().Equal(start) {
		t.Errorf("expected %v, got %v", start, c.Now())
	}
}

func TestMockClock_Advance(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := NewMock(start)

	c.Advance(5 * time.Minute)
	expected := start.Add(5 * time.Minute)
	if !c.Now().Equal(expected) {
		t.Errorf("after advance: expected %v, got %v", expected, c.Now())
	}

	c.Advance(10 * time.Second)
	expected = expected.Add(10 * time.Second)
	if !c.Now().Equal(expected) {
		t.Errorf("after second advance: expected %v, got %v", expected, c.Now())
	}
}

func TestMockClock_Set(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := NewMock(start)

	newTime := time.Date(2026, 6, 15, 14, 30, 0, 0, time.UTC)
	c.Set(newTime)

	if !c.Now().Equal(newTime) {
		t.Errorf("expected %v, got %v", newTime, c.Now())
	}
}

func TestMockClock_ConcurrentAccess(t *testing.T) {
	t.Parallel()
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	c := NewMock(start)

	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			c.Advance(time.Millisecond)
		}
		close(done)
	}()

	for i := 0; i < 100; i++ {
		_ = c.Now()
	}
	<-done
}

func TestClock_InterfaceSatisfaction(t *testing.T) {
	t.Parallel()
	// Compile-time checks
	var _ Clock = RealClock{}
	var _ Clock = FixedClock{}
	var _ Clock = &MockClock{}
}
