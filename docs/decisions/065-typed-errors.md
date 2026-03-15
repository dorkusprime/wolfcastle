# ADR-065: Typed Error Categories

## Status
Accepted

## Date
2026-03-14

## Context
The daemon treats all errors the same: log them, increment the iteration, sleep, retry. A corrupt state file and a transient network timeout both get the same response. This means a bad state.json triggers infinite retries instead of halting before the daemon does more damage.

## Decision

Introduce four error types in `internal/errors/`:

- **ConfigError**: missing model, invalid prompt template, unresolvable config. Prevents startup or stage execution.
- **StateError**: corrupt JSON, invalid state transitions, multiple in-progress tasks. Fatal. The daemon stops.
- **InvocationError**: model process failure, timeout, retry exhaustion. Retryable.
- **NavigationError**: address parsing, root index loading, FindNextTask. Fatal when structural, but distinct from state corruption.

Each type wraps an underlying error and implements `Error()` and `Unwrap()`. Convenience constructors (`werrors.State(err)`, etc.) keep call sites terse.

### Daemon behavior change

The `RunOnce` error handler inspects error types via `errors.As`:
- `StateError` returns `IterationStop` with a fatal error (halts the daemon)
- All other errors return `IterationError` (sleep and retry)

### Package naming

The package is `internal/errors` with import alias `werrors` to avoid shadowing the stdlib `errors` package.

## Consequences
- State corruption halts the daemon instead of compounding the damage
- Invocation failures are retried (same as before, now explicitly typed)
- Config errors surface at the right level (stage assembly, model lookup)
- Future phases can add error-specific metrics, alerting, or recovery strategies by switching on type
