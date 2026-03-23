# Parallel Sibling Execution

Implements ADR-095. Enables concurrent execution of independent sibling tasks under the same orchestrator, with file-level scope locks acquired by the executor agent via CLI commands.

## Overview

When enabled, the daemon launches multiple sibling tasks concurrently. Each executor acquires file-level scope locks before writing code. Scope conflicts are resolved cooperatively: an agent that cannot acquire its needed files yields immediately and is re-queued. The daemon commits each task's changes using only the files in that task's acquired scope. Serial execution remains the default.

## Config

New section under `daemon`:

```json
{
  "daemon": {
    "parallel": {
      "enabled": false,
      "max_workers": 3
    }
  }
}
```

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `parallel.enabled` | bool | `false` | Enable parallel sibling execution. When false, behavior is identical to the current serial daemon. |
| `parallel.max_workers` | int | `3` | Maximum concurrent task executions. Minimum 1. When 1 with parallel enabled, tasks still acquire scope locks but execute serially (useful for testing scope mechanics without concurrency). |

### Config types

```go
// ParallelConfig controls concurrent sibling execution.
type ParallelConfig struct {
    Enabled    bool `json:"enabled"`
    MaxWorkers int  `json:"max_workers"`
}
```

Add `Parallel ParallelConfig` to `DaemonConfig`. Add defaults in `config.Defaults()`:

```go
Parallel: ParallelConfig{
    Enabled:    false,
    MaxWorkers: 3,
}
```

Validation: `MaxWorkers` must be >= 1.

## Scope Lock Table

### Location

`.wolfcastle/system/scope-locks.json` at the namespace root. This file is ephemeral (not committed to git, added to `.gitignore`). It exists only while the daemon is running and is deleted on clean shutdown.

### Schema

```json
{
  "version": 1,
  "locks": {
    "internal/daemon/iteration.go": {
      "task": "my-project/api-layer/task-0001",
      "node": "my-project/api-layer",
      "acquired_at": "2026-03-23T10:53:00Z",
      "pid": 12345
    },
    "internal/daemon/parallel.go": {
      "task": "my-project/api-layer/task-0001",
      "node": "my-project/api-layer",
      "acquired_at": "2026-03-23T10:53:00Z",
      "pid": 12345
    }
  }
}
```

Each key is a file path relative to the repository root. Each value identifies the holding task, its node, the acquisition timestamp, and the daemon PID (for stale lock detection after crashes).

### Types

```go
// ScopeLockTable is the in-memory representation of scope-locks.json.
type ScopeLockTable struct {
    Version int                  `json:"version"`
    Locks   map[string]ScopeLock `json:"locks"`
}

// ScopeLock is a single file-level scope lock held by a running task.
type ScopeLock struct {
    Task       string    `json:"task"`
    Node       string    `json:"node"`
    AcquiredAt time.Time `json:"acquired_at"`
    PID        int       `json:"pid"`
}
```

### Concurrency

The scope lock table file is protected by the existing namespace file lock. Every CLI command that reads or writes scope locks acquires the namespace lock first. Since scope operations are fast (read JSON, check map, write JSON), the lock is held briefly. This reuses the existing locking infrastructure without adding a new lock mechanism.

### Stale lock cleanup

On daemon startup (in `selfHeal`), scan the scope lock table. For each lock, check if the PID is still alive via `signalProcess(pid, 0)`. Remove locks held by dead processes. If the scope lock file itself has a PID that doesn't match the current daemon, delete the entire file (it's from a previous crashed daemon run).

## CLI Commands

### `wolfcastle task scope add`

Acquires scope locks on one or more files for the currently executing task.

```
wolfcastle task scope add --node <address> [--task <task-id>] <file> [<file>...]
```

**Behavior:**

1. Acquire namespace lock.
2. Read scope lock table (create if absent).
3. For each requested file:
   a. If unlocked, grant it to the requesting task.
   b. If locked by the same task, no-op (idempotent).
   c. If locked by a different task, reject the entire request. Return an error naming the conflicting file and holding task. No partial acquisition: either all files are granted or none are.
4. Write updated scope lock table.
5. Release namespace lock.

**All-or-nothing semantics** prevent deadlocks. If task A holds file X and wants file Y, while task B holds file Y and wants file X, neither can make progress. By rejecting the entire batch on any conflict, the requesting agent knows immediately and can yield.

**Directory scope:** If a file argument ends with `/`, it is treated as a directory prefix. The lock covers all files under that directory. A directory lock conflicts with any file lock that shares the prefix, and vice versa. For example, locking `internal/daemon/` conflicts with an existing lock on `internal/daemon/iteration.go`.

**Exit codes:**
- 0: all locks acquired.
- 1: conflict detected. Stderr prints the conflicting file and holding task address.

**JSON output:**

```json
{
  "ok": true,
  "action": "task.scope.add",
  "data": {
    "acquired": ["internal/daemon/iteration.go", "internal/daemon/parallel.go"],
    "node": "my-project/api-layer",
    "task": "task-0001"
  }
}
```

On conflict:

```json
{
  "ok": false,
  "action": "task.scope.add",
  "error": "scope conflict: internal/daemon/iteration.go held by my-project/other-node/task-0002",
  "code": 1,
  "data": {
    "conflicts": [
      {
        "file": "internal/daemon/iteration.go",
        "held_by_task": "my-project/other-node/task-0002",
        "held_by_node": "my-project/other-node"
      }
    ]
  }
}
```

### `wolfcastle task scope list`

Lists current scope locks.

```
wolfcastle task scope list [--node <address>] [--task <task-id>]
```

Without flags, lists all locks. With `--node` or `--task`, filters to that scope. Used for debugging and by `wolfcastle status`.

### `wolfcastle task scope release`

Releases scope locks. Normally called by the daemon, not the agent.

```
wolfcastle task scope release --node <address> --task <task-id> [<file>...]
```

Without file arguments, releases all locks held by the specified task. With file arguments, releases only those files. The daemon calls this after committing a task's changes.

## Execute Prompt Changes

The execution protocol gains a scoping phase. The current phases are:

```
A. Claim (daemon-owned)
B. Study
C. Implement
D. Validate
E. Record (AAR)
F. Document (ADRs, Specs)
G. Capture Knowledge
H. Signal Completion
I. Pre-block downstream
J. Create follow-ups
```

The new phase is inserted between Study and Implement:

```
A. Claim (daemon-owned)
B. Study
B2. Acquire Scope (new)
C. Implement
...
```

### Phase B2: Acquire Scope

Added to `execute.md` only when `parallel.enabled` is true (via conditional prompt fragment or a separate fragment included when parallel is active):

```markdown
### B2. Acquire Scope

Before writing any code, declare the files you intend to modify. Run:

    wolfcastle task scope add --node <your-node> file1.go file2.go pkg/foo/

List every file you plan to create or modify. Include directories (with trailing slash)
when you expect to modify multiple files under a path. Be honest and thorough: if you
write to a file you did not scope, the daemon will reject your work.

If the command fails with a scope conflict, another task is working on those files.
Do not attempt to work around the conflict. Emit WOLFCASTLE_YIELD immediately so the
daemon can re-queue your task for later.

You may call `wolfcastle task scope add` again later if you discover additional files
you need. If that second call fails, yield. Do not write to files you have not scoped.
```

When `parallel.enabled` is false, this phase is omitted. The execute prompt is unchanged for serial execution.

### Prompt fragment approach

Rather than conditionally modifying `execute.md`, create a new prompt fragment:

```
prompts/fragments/parallel-scope.md
```

The daemon includes this fragment when `parallel.enabled` is true. This avoids modifying the core execute prompt and keeps the parallel behavior modular.

## Daemon Changes

### Worker pool

Replace the serial claim-execute-wait loop with a worker pool.

```go
// ParallelDispatcher manages concurrent task execution.
type ParallelDispatcher struct {
    daemon    *Daemon
    workers   int
    active    map[string]*WorkerSlot  // task address -> slot
    mu        sync.Mutex
    gitMu     sync.Mutex  // serializes git operations
    results   chan WorkerResult
}

// WorkerSlot tracks a running task execution.
type WorkerSlot struct {
    Node    string
    Task    string
    Cancel  context.CancelFunc
}

// WorkerResult is the outcome of a single task execution.
type WorkerResult struct {
    Node     string
    Task     string
    Result   IterationResult
    Error    error
}
```

### Dispatch flow

When `parallel.enabled` is true, `RunOnce` changes:

1. Load root index.
2. Reconcile orchestrator states (as today).
3. Find actionable tasks via `FindParallelTasks` (see Navigation below).
4. For each actionable task, if a worker slot is available:
   a. Claim the task (transition to `in_progress`).
   b. Launch a goroutine that runs the iteration (study, invoke, handle marker).
   c. Register the goroutine in the active map.
5. Wait for any worker to complete (read from results channel).
6. Handle the result (propagate state, commit changes, release scope locks).
7. Check if new siblings became available (a completed task may unblock the next one).
8. Return `IterationDidWork` if any work was done, `IterationNoWork` if the pool is empty and no tasks are available.

When `parallel.enabled` is false, the existing serial `RunOnce` is unchanged.

### Git commit serialization

All git operations go through `gitMu`. When a task completes:

1. Acquire `gitMu`.
2. Read the task's acquired scope from the scope lock table.
3. Run `git add <file1> <file2> ...` (only scoped files, replacing `git add .`).
4. If `commit_state` is enabled, also `git add .wolfcastle/`.
5. Run `git commit` with the task-specific message.
6. Release `gitMu`.

If another task completes while `gitMu` is held, it waits. Git commits are fast (no network), so contention is brief.

### HasProgress (scope-aware)

`HasProgress` currently checks:
- Did HEAD move?
- Is the working tree dirty outside `.wolfcastle/`?

With parallel execution, the working tree may be dirty from other running agents. The check changes to:

- Did HEAD move? (still global, still valid: if any task committed since this one started, HEAD moved)
- Are there uncommitted changes within this task's acquired scope?

The second check uses `git diff --name-only` filtered to the task's scoped files. If any scoped file is modified or untracked, there is progress.

### Scope lock release

After committing a task's changes, the daemon releases all scope locks for that task:

```go
func (d *ParallelDispatcher) releaseScope(taskAddr string) error {
    return d.daemon.store.MutateScopeLocks(func(table *ScopeLockTable) {
        for file, lock := range table.Locks {
            if lock.Task == taskAddr {
                delete(table.Locks, file)
            }
        }
    })
}
```

### Failure handling

When a parallel task fails:

1. The daemon increments the failure count (as today).
2. Scope locks for the failed task are released immediately.
3. If decomposition is triggered, the new subtasks are created under the failed task's node. Since scope locks are released, sibling tasks are unaffected.
4. Uncommitted changes from the failed task are committed as partial work (if `commit_on_failure` is true), scoped to the task's acquired files.
5. Other running siblings continue uninterrupted.

When a parallel task yields (emits `WOLFCASTLE_YIELD`):

1. Scope locks are released.
2. The task transitions to `not_started` (or appropriate state for re-execution).
3. Partial changes are committed if present.
4. The task is eligible for re-dispatch in a future iteration (after conflicting siblings finish).

### Task cancellation

If a running task needs to be cancelled (e.g., daemon shutdown, branch verification failure):

1. The daemon cancels the task's context.
2. The model process receives SIGTERM (existing `ProcessInvoker` behavior via context cancellation).
3. Scope locks are released on cancellation cleanup.
4. Partial changes are either committed (if `commit_on_failure`) or left uncommitted.

During daemon shutdown (SIGINT/SIGTERM), all active workers are cancelled. The daemon waits for all workers to exit (via `runWg.Wait()`) before cleanup.

## Navigation Changes

### FindParallelTasks

New function alongside `FindNextTask`:

```go
// FindParallelTasks returns up to maxWorkers actionable tasks that are siblings
// under the same orchestrator and eligible for parallel execution. Tasks are
// returned in creation order. If no parallel-safe tasks exist, it falls back
// to returning a single task (equivalent to FindNextTask).
func FindParallelTasks(idx *RootIndex, store *Store, maxWorkers int) ([]TaskRef, error)
```

**Algorithm:**

1. Run the existing DFS to find the first actionable task (same as `FindNextTask`).
2. If the task's parent is an orchestrator, scan the parent's other children for additional actionable tasks.
3. A sibling is eligible if:
   a. It is a leaf node (not an unplanned orchestrator).
   b. Its state is `not_started` (no in-progress or blocked siblings are launched).
   c. It has an actionable task (via `findActionableTask`).
4. Return up to `maxWorkers` eligible tasks, in creation order.

**Key constraint:** `FindParallelTasks` does NOT check scope disjointness. That's the executor's job via CLI scope commands. The daemon launches siblings optimistically; agents that discover scope conflicts yield immediately.

**Unplanned orchestrator rule:** The existing rule ("stop at unplanned orchestrators") is preserved. If sibling B is an unplanned orchestrator, siblings C, D, E after it are not eligible for parallel launch. Planning for B must happen first. Siblings before B in creation order are eligible.

### TaskRef

```go
// TaskRef identifies a specific task within the tree, returned by navigation.
type TaskRef struct {
    NodeAddress string
    TaskID      string
}
```

## State Mutation Concurrency

### Current model

A single namespace file lock serializes all state mutations. `MutateNode` acquires the lock, reads, applies, writes, propagates up, saves root index, releases.

### Parallel model

The namespace lock remains the serialization point. Parallel tasks mutate state (via CLI commands like `wolfcastle task complete`) through the same lock. Since CLI mutations are fast (read JSON, apply, write JSON), lock contention is acceptable even with multiple concurrent tasks.

Propagation walks up the parent chain under the same lock hold. Two siblings completing simultaneously serialize on the namespace lock. The second propagation re-reads the parent (which now reflects the first sibling's completion) and recomputes correctly.

No change to the locking model is needed for the first version. Per-node locks are a future optimization if contention becomes measurable.

### Invariant preservation

- **Parent state derivation.** Propagation always re-reads the parent before recomputing. Two concurrent propagations serialize on the lock, so each sees the other's changes. The final parent state is correct.
- **Root index consistency.** The root index is updated inside the namespace lock, after propagation. No concurrent writer can see a stale index.
- **Scope lock table consistency.** The scope lock table is protected by the same namespace lock. Scope acquisitions and releases serialize correctly.

## Scope Validation at Commit Time

After a task completes and the daemon prepares to commit:

1. Read the task's acquired scope from the scope lock table.
2. Run `git status --porcelain` to get all modified/untracked files.
3. Partition the dirty files into "in scope" and "out of scope" relative to the task's locks.
4. If any code files (outside `.wolfcastle/`) are dirty and out of scope:
   a. Log a warning identifying the out-of-scope files.
   b. Do NOT stage or commit the out-of-scope files. They belong to another running task or are unexpected.
   c. The task's commit includes only its scoped files (plus `.wolfcastle/` state if `commit_state` is true).
5. If the task has no in-scope dirty files and no HEAD movement, the `HasProgress` check fails and the task is treated as a no-progress failure (same as today).

Out-of-scope writes are not automatically a task failure. The agent may have read a file outside its scope (fine) or written to an unexpected location (the commit simply won't include it). The primary enforcement is positive: only scoped files are committed. Unscoped writes are orphaned in the working tree and will be attributed to the next task that scopes them, or cleaned up by the daemon.

## Status and Observability

### `wolfcastle status` changes

When parallel execution is active, `wolfcastle status` shows:

- The number of active workers and their tasks.
- Scope locks held by each running task.
- Worker pool capacity (active/max).

Example output:

```
Workers: 2/3 active

  my-project/api-layer/task-0001 [in_progress]
    scope: internal/api/handler.go, internal/api/routes.go

  my-project/database/task-0001 [in_progress]
    scope: internal/db/
```

### Logging

Each worker logs with a task-scoped prefix so log lines from concurrent executions can be distinguished:

```
[api-layer/task-0001] invoking model (execute stage)
[database/task-0001] invoking model (execute stage)
[api-layer/task-0001] marker detected: WOLFCASTLE_COMPLETE
```

The NDJSON log format already includes node and task fields. No structural change needed.

## Interaction with Existing Features

### Inbox processing

The inbox goroutine runs independently and is unaffected by parallel execution. Inbox intake acquires its own lock and does not conflict with parallel task execution.

### Planning passes

Planning runs when no actionable tasks exist (or when an orchestrator needs planning). With parallel execution, planning runs when the worker pool is empty and no tasks are available. If workers are active, the daemon waits for at least one to complete before checking for planning opportunities. Planning does not run concurrently with task execution.

### Auto-archive

Auto-archive runs when the pool is empty and no tasks or planning are needed (same trigger as today). No change.

### Branch verification

Branch verification runs at the start of each `RunOnce` call, before dispatching workers. If the branch changed, all active workers are cancelled and the daemon stops. No change to the verification logic itself.

### Commit state flush

`commitStateFlush` runs when the daemon goes idle (no active workers, no tasks, no planning). It acquires `gitMu` and commits any pending `.wolfcastle/` state. No change to the trigger or behavior.

## Rollout

### Phase 1: Scope lock infrastructure
- Add `ScopeLockTable` type and file I/O.
- Implement `wolfcastle task scope add/list/release` CLI commands.
- Add stale lock cleanup to `selfHeal`.
- Add scope lock display to `wolfcastle status`.
- No daemon dispatch changes. Parallel is not yet enabled.
- All tests remain serial.

### Phase 2: Scope-aware git
- Modify `commitDirect` to accept a file list (scope) instead of using `git add .`.
- Modify `HasProgress` to check scope-filtered dirtiness.
- Add `gitMu` for commit serialization.
- When `parallel.enabled` is false, `commitDirect` receives `nil` scope and falls back to `git add .` (current behavior).

### Phase 3: Parallel dispatch
- Add `ParallelDispatcher` and worker pool.
- Add `FindParallelTasks` to navigation.
- Modify `RunOnce` to use the dispatcher when `parallel.enabled` is true.
- Add parallel-scope prompt fragment.
- Integration tests with concurrent model invocations.

### Phase 4: Observability and hardening
- Worker status in `wolfcastle status`.
- Scope conflict logging and metrics.
- Failure mode testing (scope violations, concurrent propagation, worker cancellation).
- Documentation updates (human docs, agent docs).

## What This Does Not Cover

- **Dependency edges between siblings.** Siblings are assumed independent when their file scopes are disjoint. Semantic dependencies (one creates a function, another calls it) are caught by build validation after commit, not by the scope system. Explicit dependency declarations are a future enhancement.
- **Cross-orchestrator parallelism.** Only siblings under the same orchestrator are parallelized. Independent subtrees under different orchestrators execute serially (depth-first tree traversal is preserved).
- **Dynamic worker scaling.** The worker count is static per config. Adaptive scaling based on API rate limits or system load is out of scope.
- **Distributed execution.** All workers run on the same machine, in the same worktree, managed by the same daemon process. Multi-machine parallelism is a different architecture.
