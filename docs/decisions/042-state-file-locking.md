# ADR-042: State File Locking

**Status:** Accepted

**Date:** 2026-03-14

## Context

Wolfcastle uses distributed JSON state files (ADR-024) with atomic write via
temp-then-rename. This prevents corruption from partial writes but not from
concurrent access: two processes (e.g., a running daemon and a manual CLI
command, or two daemons accidentally started) can read stale state and
overwrite each other's mutations. The current PID-file check (ADR-020)
prevents duplicate daemons but doesn't protect against CLI commands mutating
state while the daemon is mid-iteration.

## Decision

Adopt advisory file locking via `flock(2)` (`syscall.Flock` on the state
file's directory) with per-namespace granularity:

1. **Lock granularity.** One lock per engineer namespace, not per-node. The
   lock file lives at `.wolfcastle/system/projects/{namespace}/.lock`.
2. **Daemon lock scope.** The daemon holds the lock for the duration of each
   iteration: acquired in `RunOnce()`, released after state save and
   propagation.
3. **CLI mutation lock.** CLI commands that mutate state (`task complete`,
   `task block`, `audit approve`, etc.) acquire the same lock before reading
   state.
4. **Read-only commands.** Commands that only read state (`status`, `pending`,
   `spec list`) do not acquire the lock.
5. **Lock timeout.** Lock acquisition uses a short timeout (5 seconds default,
   configurable). If the lock cannot be acquired, the command fails with a
   clear message: "daemon is currently processing: try again shortly."
6. **Advisory nature.** The lock prevents well-behaved Wolfcastle processes
   from conflicting but does not prevent manual JSON editing (which is fine).
7. **Platform support.** Darwin and Linux both support `flock`; Windows uses
   `LockFileEx` via the same Go abstraction.

## Consequences

- CLI mutations are safe to run while the daemon is active: they wait for
  the current iteration to finish.
- No state corruption from concurrent access.
- Minimal performance impact: lock contention is rare because iterations are
  seconds-long and CLI commands are instant.
- Does not affect manual JSON editing (advisory lock).
- Lock timeout prevents indefinite hanging if the daemon crashes while
  holding the lock.
