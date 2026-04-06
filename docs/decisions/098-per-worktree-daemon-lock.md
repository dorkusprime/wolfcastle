# Per-worktree daemon lock

## Status
Accepted

## Date
2026-04-06

## Context

Wolfcastle uses a global daemon lock in `~/.wolfcastle/daemon.lock` to enforce single-instance execution. This prevents two daemons from running simultaneously, even in separate git worktrees with completely independent `.wolfcastle/` directories and state files. Developers using worktree-heavy workflows (one worktree per branch) cannot run daemons concurrently.

The global lock was introduced when wolfcastle supported only one working directory. The assumption was sound at the time: two daemons writing to the same state files would corrupt data. But git worktrees give each checkout its own `.wolfcastle/`, so two daemons in two worktrees have no shared mutable state. The only shared resource is git itself, which handles its own locking for commits and pushes.

## Options Considered

1. **Keep global lock.** Simple, prevents all concurrency issues by preventing all concurrency. Blocks worktree workflows.

2. **User-level lock with queue.** One daemon runs at a time, others wait. Preserves safety but serializes all work across worktrees, defeating the point.

3. **Per-worktree lock.** The lock file lives inside `.wolfcastle/`. Two worktrees, two locks, two daemons. Each daemon only contends with other attempts to start in the same worktree.

## Decision

Per-worktree lock. The daemon lock moves from `~/.wolfcastle/daemon.lock` to inside the worktree's `.wolfcastle/` directory. The global lock file and its associated code (`AcquireGlobalLock`, `ReleaseGlobalLock`, `ReadGlobalLock`) are removed.

The `WOLFCASTLE_LOCK_DIR` environment variable remains for test isolation.

## Consequences

- Multiple daemons can run concurrently in separate worktrees.
- The "already running" check becomes per-worktree: starting a daemon in a worktree that already has one fails, but starting in a different worktree succeeds.
- Discovery of running instances requires a separate mechanism (the instance registry in `~/.wolfcastle/instances/`), since there is no single lock file to check.
- Git push contention between concurrent daemons on different branches is handled by git, not by wolfcastle. A failed push is a recoverable error.
