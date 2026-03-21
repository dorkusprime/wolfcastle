# ADR-068: Unified Store for File-Backed State

## Status
Accepted

## Date
2026-03-15

## Context

The same read-modify-write race keeps surfacing in different corners of the codebase, and each time we patch it with a slightly different mechanism. The inbox got `InboxMutate` (lock, read, callback, write, unlock) after a write-race between the daemon's intake goroutine and CLI append commands. The root index got a "re-read from disk" workaround in `propagateState` after the daemon clobbered nodes that the intake model had created mid-iteration via CLI. Node state gets reloaded after model invocation to pick up breadcrumbs and scope changes written by CLI commands the model called.

The building blocks are already in place: `FileLock` (ADR-042) provides per-namespace advisory locking, and `atomicWriteJSON` guarantees readers never see a partial write. But these primitives aren't composed consistently. `InboxMutate` does the full lock-read-callback-write cycle; root index and node state mutations do not. The result is a patchwork of ad-hoc fixes that each solve the same structural problem in isolation.

## Decision

Introduce a `Store` type in `internal/state/` that provides `Mutate*` methods for all three state file types: node state, root index, and inbox. Every mutation follows the same sequence: acquire lock, read current state from disk, invoke a caller-supplied callback to modify it, atomically write the result, release lock.

Read operations (`ReadNode`, `ReadIndex`, `ReadInbox`) do not acquire the lock. Because `atomicWriteJSON` uses temp-file-then-rename, readers are guaranteed to see either the previous complete file or the new complete file, never a torn write.

The store is parameterized by namespace directory (the projects dir from the resolver) and holds a single `FileLock` for the namespace. All mutations, regardless of which file they target, serialize through this one lock. This matches the existing granularity from ADR-042 and prevents cross-file inconsistencies within a namespace.

Raw `LoadNodeState`/`SaveNodeState`, `LoadRootIndex`/`SaveRootIndex`, and `LoadInbox`/`SaveInbox` remain available for tests and backward compatibility but are no longer the primary API for production code paths. `InboxMutate` and `InboxAppend` become thin wrappers that delegate to the store.

## Consequences

- The read-modify-write race is eliminated structurally rather than patched case by case. Every state mutation, whether from the daemon, a CLI command, or the intake model calling CLI commands, flows through the same lock-read-callback-write sequence.
- The `propagateState` re-read workaround becomes unnecessary. Instead of reading the index once and holding it across an iteration (then defensively re-reading before writing), each propagation step calls `store.MutateIndex`, which reads the current state from disk inside the lock.
- The daemon, CLI commands, and intake model all share a single entry point for state mutations. New callers get correct locking by default.
- A small runtime cost: each mutation reads state from disk inside the lock rather than operating on a cached copy. For Wolfcastle's write volume (a handful of state transitions per iteration), this cost is negligible.
- No architectural changes to the locking model itself. The store composes the existing `FileLock` and `atomicWriteJSON` primitives into a consistent API.
