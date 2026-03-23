# ADR-039: Clean Daemon Iteration Boundary

**Status:** Accepted

**Date:** 2026-03-14

## Context

ADR-020 established the daemon lifecycle with `Run()` as the main loop and
`runIteration()` for pipeline execution. The loop body: shutdown checks,
stop file detection, iteration cap, branch verification, navigation, and
sleep timing: lived inline within `Run()`, interleaved with one-time setup
(signal handling, self-healing, branch recording).

While `runIteration()` provided a reasonable boundary for the pipeline logic,
the loop control was not independently testable or composable. Testing whether
the stop file works required standing up signal handlers. The supervisor
restarted the entire `Run()` function rather than individual iterations,
making crash recovery semantics implicit.

## Decision

Extract the loop body into a public `RunOnce()` method that performs a single
daemon iteration and returns a typed result:

```go
type IterationResult int

const (
    IterationDidWork IterationResult = iota
    IterationNoWork
    IterationStop
    IterationError
)

func (d *Daemon) RunOnce(ctx context.Context) (IterationResult, error)
```

`Run()` becomes a thin loop: one-time setup (signals, self-healing, branch
recording), then repeated calls to `RunOnce()` with sleep timing determined
by the result type. Fatal errors (returned as non-nil error from `RunOnce`)
halt the daemon; recoverable errors return `IterationError` with a nil error.

The result enum lets `Run()` choose the appropriate sleep interval without
`RunOnce()` knowing about timing. `IterationNoWork` sleeps longer (blocked
poll interval), while `IterationDidWork` and `IterationError` sleep at the
normal poll interval.

The iteration counter moves from a local variable in `Run()` to a field on
the Daemon struct, since `RunOnce()` needs to read and increment it.

## Consequences

- **Testability.** `RunOnce()` can be called in isolation: pass a context,
  get a result. Loop control, shutdown, and navigation are testable without
  running the full daemon lifecycle.
- **Clarity.** The boundary between "should I keep going?" (`Run`) and "do
  one unit of work" (`RunOnce`) is explicit and compiler-enforced.
- **Composability.** Future features like dry-run mode, single-step
  debugging, or metrics-per-iteration can call `RunOnce()` directly.
- **No behavioral change.** The refactor preserves all existing behavior:
  same shutdown semantics, same sleep timing, same supervisor interaction.
