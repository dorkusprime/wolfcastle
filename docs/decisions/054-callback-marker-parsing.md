# ADR-054: Callback-Based Marker Parsing

## Status
Accepted

## Date
2026-03-14

## Context
The `applyModelMarkers` method (in `internal/daemon/daemon.go`) is a 90-line switch statement that directly mutates `NodeState` and logs to the daemon's logger. This tightly couples marker parsing to the daemon — the parsing logic cannot be tested without constructing a full `Daemon` struct, and it cannot be reused outside the daemon context (e.g., in integration tests or a future replay tool).

Functional callbacks are an idiomatic Go pattern for decoupling parsing from side effects. A struct of optional callback functions — one per marker type — lets the parser invoke callbacks when markers are found, while callers wire those callbacks to whatever state, logging, or side-effect system they need. Each callback can be nil (marker is ignored) or a function (marker is processed). This enables:
- Testing marker parsing without a daemon
- Wiring markers to mock state for assertion
- Adding new markers without modifying the parser

## Decision

Extract marker parsing into a standalone function in a dedicated file (`internal/daemon/markers.go`, per ADR-045), using a callback struct to decouple parsing from daemon internals:

### MarkerCallbacks Struct

```go
type MarkerCallbacks struct {
    OnBreadcrumb     func(text string)
    OnGap            func(description string)
    OnFixGap         func(gapID string)
    OnScope          func(description string)
    OnScopeFiles     func(raw string)
    OnScopeSystems   func(raw string)
    OnScopeCriteria  func(raw string)
    OnSummary        func(text string)
    OnResolveEsc     func(escalationID string)
    OnComplete       func()
    OnBlocked        func(reason string)
    OnYield          func()
}
```

### Parser Function

```go
func ParseMarkers(output string, cb MarkerCallbacks) {
    for _, line := range strings.Split(output, "\n") {
        line = strings.TrimSpace(line)
        switch {
        case cb.OnBreadcrumb != nil && strings.HasPrefix(line, "WOLFCASTLE_BREADCRUMB:"):
            cb.OnBreadcrumb(strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_BREADCRUMB:")))
        case cb.OnComplete != nil && strings.Contains(line, "WOLFCASTLE_COMPLETE"):
            cb.OnComplete()
        // ... etc
        }
    }
}
```

### Daemon Wiring

The daemon's `runIteration` method constructs callbacks that close over `ns`, `nav`, and the logger:

```go
ParseMarkers(result.Stdout, MarkerCallbacks{
    OnBreadcrumb: func(text string) {
        state.AddBreadcrumb(ns, nav.NodeAddress+"/"+nav.TaskID, text)
        d.Logger.Log(map[string]any{"type": "marker_breadcrumb", "text": text})
    },
    OnGap: func(desc string) {
        gapID := fmt.Sprintf("gap-%s-%d", ns.ID, len(ns.Audit.Gaps)+1)
        ns.Audit.Gaps = append(ns.Audit.Gaps, state.Gap{...})
        d.Logger.Log(map[string]any{"type": "marker_gap", "gap_id": gapID})
    },
    // ...
})
```

### Testing

Marker parsing can now be tested in isolation:

```go
func TestParseMarkersComplete(t *testing.T) {
    called := false
    ParseMarkers("some output\nWOLFCASTLE_COMPLETE\n", MarkerCallbacks{
        OnComplete: func() { called = true },
    })
    if !called { t.Error("expected OnComplete callback") }
}
```

## Consequences
- Marker parsing is testable without a daemon
- New markers are added by extending `MarkerCallbacks` and adding a case — no coupling to daemon internals
- Nil callbacks are safe — markers without handlers are silently skipped
- The daemon wiring is explicit and readable — each callback is a closure with clear dependencies
- Future tools (replay, dry-run, analysis) can reuse the parser with different callbacks
