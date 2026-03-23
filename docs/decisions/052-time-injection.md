# ADR-052: Time Injection for Deterministic Testing

## Status
Accepted

## Date
2026-03-14

## Context
The codebase calls `time.Now()` directly in state mutation functions (`AddBreadcrumb`, `AddEscalation`, `TaskBlock`), daemon lifecycle methods, and archive rollup. This makes time-dependent behavior non-deterministic in tests: assertions on timestamps require fuzzy matching or ignoring the field entirely, and tests that depend on time ordering become fragile and difficult to reason about.

Injecting time through an interface allows test code to supply a fixed or controllable clock, enabling deterministic assertions on `CompletedAt`, `StartedAt`, breadcrumb timestamps, and escalation timestamps without sacrificing production behavior.

## Decision

Introduce a `Clock` interface and inject it through the `App` and `Daemon` structs:

```go
// internal/clock.go (or within an appropriate package)
type Clock interface {
    Now() time.Time
}

type realClock struct{}
func (realClock) Now() time.Time { return time.Now().UTC() }

type fixedClock struct{ t time.Time }
func (c fixedClock) Now() time.Time { return c.t }
```

### Injection Points

- **`cmdutil.App`** gains a `Clock` field, defaulting to `realClock{}` in production
- **`daemon.Daemon`** gains a `Clock` field, passed from `App` at construction
- **State mutation functions** that currently call `time.Now().UTC()` accept the clock as a parameter or use the one from their receiver

### Affected Functions

| Function | Current | After |
|----------|---------|-------|
| `state.AddBreadcrumb` | `time.Now().UTC()` | Clock parameter |
| `state.AddEscalation` | `time.Now().UTC()` | Clock parameter |
| `daemon.applyModelMarkers` (gap timestamps) | `time.Now().UTC()` | `d.Clock.Now()` |
| `archive.Rollup` (archived timestamp) | `time.Now().UTC()` | Clock parameter |
| `logging.Logger.Log` (record timestamp) | `time.Now().UTC()` | Clock parameter |

### Test Usage

```go
func TestBreadcrumbTimestamp(t *testing.T) {
    fixed := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
    state.AddBreadcrumb(ns, "task-1", "did something", fixedClock{fixed})
    if ns.Audit.Breadcrumbs[0].Timestamp != fixed {
        t.Errorf("expected %v, got %v", fixed, ns.Audit.Breadcrumbs[0].Timestamp)
    }
}
```

### Scope

This is an incremental change: each function is updated individually as its tests are written or improved. The `Clock` field on `App` defaults to `realClock{}`, so production behavior is unchanged without any caller modifications.

## Consequences
- All time-dependent tests become deterministic: no more fuzzy timestamp matching
- The `Clock` interface is minimal (one method) with no over-abstraction
- Production code is unchanged: `realClock{}` is the default
- Test code can advance time, freeze time, or inject specific timestamps
- Uses Go interfaces for clean dependency injection while keeping the surface area small
