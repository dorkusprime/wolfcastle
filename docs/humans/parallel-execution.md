# Parallel Execution

By default, Wolfcastle executes one task at a time. Parallel mode lets the daemon run multiple tasks concurrently, each in its own worker slot, with file-level scope locks preventing two workers from editing the same files.

## Enabling Parallel Mode

Add this to your `config.json` at [whatever tier you want](configuration.md#three-tier-directory-structure])

```json
{
  "daemon": {
    "parallel": {
      "enabled": true,
      "max_workers": 3
    }
  }
}
```

`max_workers` controls how many tasks can run simultaneously. Start with 2 or 3; going higher increases the chance of scope conflicts without proportional throughput gains.

## How It Works

The parallel dispatcher replaces the serial "find one task, run it, repeat" loop with a worker pool. On each tick, the daemon:

1. Drains completed workers, committing their scoped changes and releasing scope locks.
2. Fills open worker slots by finding eligible tasks via the normal navigation algorithm.
3. Falls through to planning only when the entire pool is empty (no workers active, none dispatched). This prevents plan-while-executing races.

Each worker runs the same pipeline as serial mode: claim the task, run the execute stage, parse terminal markers, handle failures. The difference is that multiple workers run concurrently, and git operations (commits) are serialized through a mutex.

## Scope Locks

When a task declares its scope (the files it intends to modify), those paths are recorded in `.wolfcastle/system/projects/<namespace>/scope-locks.json`. Before a second task can start, the daemon checks for overlapping scope. If two tasks would touch the same files, the later one yields with a `scope_conflict` and waits for the first to finish.

A yielded task is reset to `not_started` and becomes eligible for re-dispatch once its blocker completes. The dispatcher tracks yield counts and durations so you can see which tasks are contending. If all workers yield simultaneously (a circular dependency), the stale-entry cleanup in `isBlocked` breaks the cycle on the next dispatch pass by detecting that no blocker is actually active.

## Worker Status

The [TUI dashboard](the-tui.md#the-dashboard) shows the current target and progress for all active workers in real time. From the command line, run `wolfcastle status` to see the worker pool while the daemon is running. The parallel section appears below the daemon line and shows active workers, their scope locks, and any yielded tasks waiting on scope. See [status](cli/status.md) for the full output format.

## Failure Handling

Workers are isolated. A panic in one worker is recovered, logged with a stack trace, and reported as an error result. The worker's scope locks are released and its failure count incremented, just like a serial failure.

Context cancellation (from a branch change or shutdown signal) cancels all active workers simultaneously. The daemon waits for them to drain before exiting, so no goroutines are orphaned.

On daemon startup, `selfHeal` cleans up stale scope locks left by a previous crash. Any lock held by a PID that is no longer running gets removed.

## When to Use Parallel Mode

Parallel mode is most effective when your project tree has multiple independent leaf nodes with non-overlapping scope. A project with three modules that each touch different directories will see near-linear speedup at `max_workers: 3`.

It is less useful when tasks frequently overlap in scope, since the yield-and-retry overhead can approach serial execution time. The yield tracking in the TUI dashboard and `wolfcastle status` shows you this directly: if the same tasks keep yielding repeatedly, consider restructuring the task tree to reduce file contention.
