// Package clock provides a minimal time abstraction for deterministic testing.
// Production code uses RealClock; tests inject FixedClock or MockClock.
package clock

import (
	"sync"
	"time"
)

// Clock abstracts time.Now so that test code can supply deterministic timestamps.
type Clock interface {
	Now() time.Time
}

// RealClock delegates to the system clock, always returning UTC.
type RealClock struct{}

// Now returns the current time in UTC.
func (RealClock) Now() time.Time { return time.Now().UTC() }

// New returns the default production clock.
func New() Clock { return RealClock{} }

// FixedClock returns the same instant every time Now is called.
type FixedClock struct {
	T time.Time
}

// Now returns the fixed time.
func (c FixedClock) Now() time.Time { return c.T }

// NewFixed creates a FixedClock pinned to the given instant.
func NewFixed(t time.Time) FixedClock { return FixedClock{T: t} }

// MockClock is a thread-safe, advanceable clock for test scenarios that
// need multiple distinct timestamps within a single test case.
type MockClock struct {
	mu sync.Mutex
	t  time.Time
}

// NewMock creates a MockClock starting at the given time.
func NewMock(t time.Time) *MockClock { return &MockClock{t: t} }

// Now returns the current mock time.
func (c *MockClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

// Advance moves the mock clock forward by the given duration.
func (c *MockClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// Set replaces the mock clock's current time.
func (c *MockClock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = t
}
