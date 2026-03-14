# Testability and Decoupling Improvements

This spec details six architectural improvements focused on test infrastructure, decoupling, and determinism. Each section describes the current limitation, the target design, and the implementation approach.

## Governing ADRs

- ADR-050: Integration Test Harness via CLI Dispatch
- ADR-051: Multi-Pass Doctor with Fix Verification
- ADR-052: Time Injection for Deterministic Testing
- ADR-053: Centralized Configuration Defaults
- ADR-054: Callback-Based Marker Parsing
- ADR-055: Property-Based Propagation Tests
- ADR-056: Cobra Dependency Evaluation

---

## 1. Integration Test Suite

### Current Limitation

Unit tests exercise `internal/` packages in isolation but cannot detect integration failures — wrong flag names, missing `RequireResolver()` calls, JSON envelope mismatches, or state propagation errors that only manifest when commands execute in sequence.

### Target Design

An integration test infrastructure in `test/integration/` that exercises complete workflows through Cobra's `rootCmd.Execute()`:

```go
//go:build integration

func run(t *testing.T, dir string, args ...string) string {
    // Reset Cobra state, set args, capture stdout, execute
}

func loadNode(t *testing.T, dir, addr string) *state.NodeState {
    // Read state.json from the expected path
}
```

### Test Pattern

Sequential commands with direct disk verification:

```go
func TestProjectLifecycle(t *testing.T) {
    dir := t.TempDir()
    run(t, dir, "init")
    run(t, dir, "project", "create", "my-feature")
    run(t, dir, "task", "add", "--node", "my-feature", "implement API")
    run(t, dir, "task", "claim", "--node", "my-feature/task-1")
    run(t, dir, "task", "complete", "--node", "my-feature/task-1")

    idx := loadRootIndex(t, dir)
    if idx.Nodes["my-feature"].State != state.StatusComplete {
        t.Errorf("expected complete, got %s", idx.Nodes["my-feature"].State)
    }
}
```

### Critical Paths to Cover

| Test | Commands Exercised |
|------|--------------------|
| Project lifecycle | create → add → claim → complete → status |
| Decomposition depth | Nested project creation, verify depth tracking |
| State recomputation | Mixed blocked/not_started children, verify parent state |
| Navigation priority | Multiple actionable tasks, verify DFS order |
| Doctor fix | Corrupt state on disk → doctor --fix → verify repair |
| Archive generation | Complete a node → archive add → verify markdown |
| Inbox lifecycle | inbox add → inbox list → inbox clear |
| Audit workflow | audit run → pending → approve/reject → history |

---

## 2. Multi-Pass Doctor

### Current Limitation

`ApplyDeterministicFixes` runs a single pass — some fixes create new issues (e.g., relinking an orphan creates a propagation mismatch).

### Target Design

A new `FixWithVerification()` method wraps validate + fix in a multi-pass loop:

```go
const maxFixPasses = 5

func (e *Engine) FixWithVerification(categories []Category) (*Report, error) {
    for pass := 0; pass < maxFixPasses; pass++ {
        idx, err := state.LoadRootIndex(e.rootIndexPath)
        report := e.Validate(idx, categories)
        if !report.HasAutoFixable() { return report, nil }
        fixes := e.ApplyDeterministicFixes(idx, report)
        if len(fixes) == 0 { break }
    }
    // Final validation-only pass
    idx, _ := state.LoadRootIndex(e.rootIndexPath)
    return e.Validate(idx, categories), nil
}
```

The `doctor --fix` command calls `FixWithVerification()` instead of the single-pass `ApplyDeterministicFixes()`.

### Cascading Fix Examples

| Pass 1 Fix | Cascading Issue | Pass 2 Fix |
|-----------|----------------|-----------|
| Add missing audit task | Audit status mismatch | Sync audit lifecycle |
| Relink orphaned node | Parent child-ref stale | Propagation recomputation |
| Remove dangling index entry | Root state stale | Root state recomputation |

---

## 3. Time Injection

### Current Limitation

Direct `time.Now()` calls in mutation functions make time-dependent tests non-deterministic.

### Target Design

A `Clock` interface injected through `App` and `Daemon`:

```go
type Clock interface {
    Now() time.Time
}

type realClock struct{}
func (realClock) Now() time.Time { return time.Now().UTC() }

type fixedClock struct{ t time.Time }
func (c fixedClock) Now() time.Time { return c.t }
```

State mutation functions that need timestamps accept the clock as a parameter. Production code passes `realClock{}`, test code passes `fixedClock{t}`.

### Affected Functions

| Function | Current | After |
|----------|---------|-------|
| `state.AddBreadcrumb` | `time.Now().UTC()` | Clock parameter |
| `state.AddEscalation` | `time.Now().UTC()` | Clock parameter |
| `daemon.applyModelMarkers` | `time.Now().UTC()` | `d.Clock.Now()` |
| `archive.Rollup` | `time.Now().UTC()` | Clock parameter |

---

## 4. Centralized Config Defaults

### Current Limitation

Default values are scattered across `config.Load()`, `config.Validate()`, embedded templates, and daemon runtime initialization.

### Target Design

A single `DefaultConfig()` function that returns a complete, valid config:

```go
func DefaultConfig() *Config {
    return &Config{
        Models: map[string]ModelDef{
            "fast":  {Command: "claude", Args: [...]},
            "mid":   {Command: "claude", Args: [...]},
            "heavy": {Command: "claude", Args: [...]},
        },
        Pipeline: PipelineConfig{
            Stages: []PipelineStage{...},
        },
        Daemon: DaemonConfig{
            PollIntervalSeconds: 10,
            BlockedPollIntervalSeconds: 60,
            // ... all defaults
        },
        // ... complete config
    }
}
```

The `Load()` function changes to: `DefaultConfig()` → overlay `config.json` → overlay `config.local.json` → `Validate()`.

---

## 5. Callback-Based Marker Parsing

### Current Limitation

`applyModelMarkers` in daemon.go directly mutates `NodeState` and logs to the daemon logger, making marker parsing untestable without a full `Daemon` struct.

### Target Design

A `MarkerCallbacks` struct with optional function fields, and a standalone `ParseMarkers` function:

```go
type MarkerCallbacks struct {
    OnComplete     func()
    OnBlocked      func(reason string)
    OnBreadcrumb   func(text string)
    OnGap          func(description string)
    OnFixGap       func(gapID string)
    OnSummary      func(text string)
    // ... one per marker type
}

func ParseMarkers(output string, cb MarkerCallbacks) {
    for _, line := range strings.Split(output, "\n") {
        line = strings.TrimSpace(line)
        switch {
        case cb.OnBreadcrumb != nil && strings.HasPrefix(line, "WOLFCASTLE_BREADCRUMB:"):
            cb.OnBreadcrumb(strings.TrimSpace(strings.TrimPrefix(line, "WOLFCASTLE_BREADCRUMB:")))
        // ...
        }
    }
}
```

The daemon wires callbacks that close over `NodeState`, `NavigationResult`, and `Logger`. Tests construct callbacks that capture values for assertion.

---

## 6. Property-Based Propagation Tests

### Current Limitation

Propagation tests use hand-crafted scenarios covering a finite set of tree shapes and mutation sequences.

### Target Design

Use `testing/quick` with an in-memory tree to generate random mutations and verify invariants:

```go
func TestPropagationInvariants(t *testing.T) {
    f := func(seed int64) bool {
        tree := generateRandomTree(rand.New(rand.NewSource(seed)))
        for i := 0; i < numMutations; i++ {
            applyRandomMutation(tree)
            propagateAll(tree)
        }
        return verifyConsistency(tree)
    }
    quick.Check(f, &quick.Config{MaxCount: 500})
}
```

Invariants verified: parent-child state consistency, root index accuracy, depth tracking, idempotent propagation.

---

## Implementation Priority

| Item | Priority | Effort | Impact |
|------|----------|--------|--------|
| Integration test suite | **P0** — before beta | Large (50+ tests) | Closes the biggest quality gap |
| Multi-pass doctor | **P1** — before beta | Small (wrapper function) | Fixes cascading repair issues |
| Callback marker parsing | **P1** — with daemon decomposition | Medium (extract + refactor) | Enables marker testing without daemon |
| Centralized defaults | **P2** — before 1.0 | Small (consolidation) | Improves config discoverability |
| Time injection | **P2** — before 1.0 | Medium (incremental) | Enables deterministic time tests |
| Property-based tests | **P2** — before 1.0 | Medium (test infrastructure) | Catches edge cases in propagation |
| Cobra evaluation | **P3** — review trigger | None (decision recorded) | Risk documentation |
