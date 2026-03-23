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

Add to `ValidateStructure` in `validate.go`: `MaxWorkers` must be >= 1.

The config merge system (`DeepMerge`) handles nested structs correctly via JSON round-trip. A tier overlay like `{"daemon": {"parallel": {"max_workers": 5}}}` deep-merges as expected.

## Scope Lock Table

### Location

`.wolfcastle/system/projects/{namespace}/scope-locks.json`, alongside the existing `state.json` (root index) and `.lock` file. This placement puts the scope lock table inside the namespace directory, where it is protected by the existing namespace file lock.

The file is ephemeral, deleted on clean daemon shutdown. It exists only while the daemon is running. The `.wolfcastle/.gitignore` uses explicit excludes (not a deny-all pattern), so files under `system/projects/` are tracked by default. Add `scope-locks.json` to `.wolfcastle/.gitignore` alongside the existing runtime artifact rules (`system/wolfcastle.pid`, `system/stop`, `*.lock`) to prevent it from being committed.

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
    }
  }
}
```

Each key is a file path relative to the repository root. Each value identifies the holding task, its node, the acquisition timestamp, and the daemon PID (for stale lock detection after crashes).

Scope locks are namespace-wide, not scoped to an orchestrator's children. Any running task in the namespace can see any other task's locks. This is intentional: tasks at different depths in the tree share the same working tree, so file conflicts are real regardless of tree position.

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

The scope lock table is protected by the existing namespace file lock. Every CLI command that reads or writes scope locks acquires the namespace lock first. The lock is held briefly (read JSON, check map, write JSON).

The namespace lock is also the serialization point for all state mutations (`MutateNode`, `MutateIndex`, `MutateInbox`). With parallel workers, lock hold time includes the full ancestor propagation walk (loading and saving each ancestor in the parent chain). For a node at depth 5, that is approximately 10 file I/O operations under a single lock hold. This is fast in absolute terms (local filesystem) but represents more work than a bare "read, apply, write." The existing 5-second lock timeout (`DefaultLockTimeout`) is generous enough for this workload even with 3 concurrent workers.

A new `MutateScopeLocks` method on `Store` follows the pattern of `MutateInbox`: acquire namespace lock, read JSON from fixed path, apply callback, write atomically, release lock.

### Stale lock cleanup

On daemon startup (in `selfHeal`), read the scope lock table. For each lock, check if the holding PID is still alive. The `isProcessRunning` function in `internal/state/filelock.go` provides PID liveness checking; it is unexported, so either export it or add an equivalent to the daemon package. Remove locks held by dead processes. If the entire scope lock file has a PID that doesn't match the current daemon, delete the file (leftover from a previous crashed run).

## CLI Commands

### `wolfcastle task scope add`

Acquires scope locks on one or more files for the currently executing task.

```
wolfcastle task scope add --node <address> [--task <task-id>] <file> [<file>...]
```

**Behavior:**

1. Acquire namespace lock.
2. Read scope lock table (create if absent).
3. For each requested file, check for conflicts using the bidirectional prefix rule (see below).
   a. If unlocked, grant it to the requesting task.
   b. If locked by the same task, no-op (idempotent).
   c. If locked by a different task, reject the entire request. Return an error naming the conflicting file and holding task. No partial acquisition: either all files are granted or none are.
4. Write updated scope lock table.
5. Release namespace lock.

**All-or-nothing semantics** prevent deadlocks on initial acquisition. For incremental scope expansion (calling `scope add` again after already holding locks), the all-or-nothing applies only to the new request. Already-held locks are not released on failure. If the agent cannot expand its scope, it should yield. Yielding releases all held locks (see Yield Handling below), so the hold-and-wait condition is broken.

**Directory scope:** If a file argument ends with `/`, it is treated as a directory prefix. Two scope entries conflict if either is a prefix of the other (bidirectional containment):
- `internal/daemon/` conflicts with `internal/daemon/iteration.go` (directory contains file)
- `internal/daemon/iteration.go` conflicts with `internal/daemon/` (file is inside directory)
- `internal/` conflicts with `internal/daemon/` (parent contains child directory)
- `internal/daemon/` conflicts with `internal/` (child is inside parent)

A path without a trailing slash is always treated as a file, even if a directory with that name exists.

**Exit codes:**
- 0: all locks acquired.
- 1: conflict detected. Stderr prints the conflicting file and holding task address.

**JSON output (`output.Ok`/`output.Err` envelope):**

```json
{
  "ok": true,
  "action": "task_scope_add",
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
  "action": "task_scope_add",
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

Action names use underscores (`task_scope_add`) to match the existing convention (`task_add`, `task_complete`).

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

### CLI registration

The `scope` subcommand is a new command group under `task` in `cmd/task/register.go`. This is the first nested subcommand group under `task`; existing subcommands (`add`, `claim`, `complete`, `block`, `unblock`, `deliverable`, `amend`) are direct children. The `scope` parent command has no action of its own; it serves as a grouping node for `add`, `list`, `release`.

## Execute Prompt Changes

### How scope instructions reach the agent

The existing prompt system has no conditional fragment inclusion based on config values. Rule fragments (in `rules/`) are included unconditionally via `PromptsConfig.Fragments`. Stage prompts (`stages/execute.md`) are loaded as raw markdown, not templates with conditional blocks.

Rather than adding a conditional mechanism, scope instructions are injected through the **iteration context**. The daemon's `ContextBuilder.Build()` already composes per-task context that varies each iteration (task details, node state, AARs, etc.). When `parallel.enabled` is true, the context builder appends a scope acquisition section to the iteration context:

```markdown
## Parallel Execution: Scope Acquisition Required

Before writing any code, declare the files you intend to modify:

    wolfcastle task scope add --node {node} file1.go file2.go pkg/foo/

List every file you plan to create or modify. Include directories (with trailing
slash) when you expect to modify multiple files under a path. Be thorough: if you
write to a file you did not scope, the daemon rejects your commit and the task fails.

If the command fails with a scope conflict, another task is working on those files.
Emit WOLFCASTLE_YIELD immediately. Do not attempt to work around the conflict.

You may call `wolfcastle task scope add` again later if you discover additional files.
If that second call fails, emit WOLFCASTLE_YIELD.
```

This approach requires no changes to `execute.md`, no new fragment mechanism, and no template conditionals. The context builder checks `d.Config.Daemon.Parallel.Enabled` and appends the section. When parallel is disabled, the section is absent and execution proceeds as today.

### Phase ordering

The existing execute.md phases are:

```
A. Claim (daemon-owned)
[Audit Tasks block - unlettered, replaces B-J for audit tasks]
B. Study
C. Implement
D. Validate
E. Record (AAR)
F. Document WHY (ADRs) and WHAT/HOW (Specs)
G. Capture Codebase Knowledge
H. Signal completion
I. Pre-block downstream tasks
J. Create follow-up tasks
```

Scope acquisition happens between B (Study) and C (Implement). The agent reads the codebase in Study, then acquires scope based on what it learned, then implements. No change to `execute.md` is needed; the iteration context tells the agent to acquire scope after studying and before implementing.

Audit tasks skip scope acquisition entirely. Audits write only to `.wolfcastle/` via CLI commands, never to code files, so they have no scope conflicts.

## Daemon Changes

### Shared mutable state

The `Daemon` struct has fields that are written during iteration without synchronization. With concurrent workers, these need protection:

| Field | Current usage | Parallel fix |
|-------|---------------|-------------|
| `d.iteration` | Counter incremented each iteration | Use `atomic.Int64` |
| `d.lastNoWorkMsg` | Dedup for "no work" log messages | Protect with `d.mu sync.Mutex`, or move to dispatcher |
| `d.lastArchiveCheck` | Timestamp for archive throttling | Protect with `d.mu` |
| `d.Logger` | Shared logger with per-iteration prefix | Each worker creates its own logger via `d.Logger.StartIterationWithPrefix(taskAddr)`. Workers do not share a logger instance. The parent logger must support concurrent `StartIterationWithPrefix` calls (create independent child loggers, not mutate the parent). |

### Worker pool

```go
// ParallelDispatcher manages concurrent task execution.
type ParallelDispatcher struct {
    daemon     *Daemon
    maxWorkers int
    active     map[string]*WorkerSlot  // task address -> slot
    mu         sync.Mutex
    gitMu      sync.Mutex              // serializes all git operations
    results    chan WorkerResult
    blocked    map[string]string       // task address -> conflicting task address (yield backoff)
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

The `blocked` map tracks which yielded tasks are waiting on which conflicting tasks. A yielded task is not eligible for re-dispatch until its blocking task completes and releases locks (see Yield Handling).

### Dispatch flow

When `parallel.enabled` is true, `RunOnce` changes to:

1. Check shutdown channel. *(preserved from serial)*
2. Check stop file. *(preserved)*
3. Check max iterations. *(preserved)*
4. Verify branch hasn't changed. If changed, cancel all active workers and return `IterationStop`. *(preserved, extended)*
5. Load root index. *(preserved)*
6. `deliverPendingScope(idx)`. *(preserved)*
7. `reconcileOrchestratorStates(idx)`. *(preserved)*
8. **Drain completed workers.** Read all available results from the results channel (non-blocking). For each completed worker:
   a. Handle the result (propagate state, commit scoped changes, release scope locks).
   b. Remove from active map.
   c. Clear any `blocked` entries that reference this task (unblock yielded siblings).
9. **Fill worker slots.** Call `FindParallelTasks` to get actionable siblings (up to available slot count). For each:
   a. Skip if the task is in the `blocked` map and its blocker is still active.
   b. Claim the task (transition to `in_progress`).
   c. Create a per-worker context via `context.WithCancel(ctx)`.
   d. Launch a goroutine that runs the iteration (study, scope acquisition, invoke, handle marker).
   e. Register in the active map and `runWg`.
10. If no active workers and no tasks found:
    a. Try `findPlanningTarget` and run planning. *(preserved)*
    b. Try `tryAutoArchive`. *(preserved)*
    c. Run `commitStateFlush`. *(preserved)*
    d. Return `IterationNoWork`.
11. If workers are active but no new tasks dispatched, return `IterationDidWork` (keep the Run loop alive to drain results next iteration).

Planning runs only when the worker pool is empty AND no actionable tasks exist. The daemon never plans while workers are active, because in-progress tasks may change the tree structure (creating subtasks, blocking, completing) and planning against a moving target produces stale plans.

### IterationResult semantics

The existing four values (`DidWork`, `NoWork`, `Stop`, `Error`) remain. The `Run` loop's handling:

- `IterationDidWork`: call `RunOnce` again immediately (no sleep). This covers both "dispatched new workers" and "just draining active workers."
- `IterationNoWork`: sleep on poll timeout or `workAvailable` channel. Only returned when the pool is empty AND no tasks/planning/archive work exists.
- `IterationStop`: cancel all active workers, wait for `runWg`, return.
- `IterationError`: cancel all active workers, wait, sleep, retry.

If a single worker errors while others succeed, the dispatcher handles it per-worker (logs error, increments failure count, releases scope). The overall `RunOnce` returns `IterationDidWork` because work happened. `IterationError` is reserved for daemon-level failures (root index corruption, lock timeout on dispatch).

### Git commit serialization

All git operations go through `gitMu`. When a task completes:

1. Acquire `gitMu`.
2. Read the task's acquired scope from the scope lock table.
3. Build the file list for `git add`:
   - Start with the task's scoped files.
   - If `commit_state` is true, add `.wolfcastle/` to the list.
   - If `commit_state` is false, do not include `.wolfcastle/`.
4. Run `git add <file1> <file2> ...` (only the files from step 3).
5. Check `git status --porcelain` filtered to the staged files. If nothing is staged, skip the commit.
6. Run `git commit` with the task-specific message.
7. Release `gitMu`.

This replaces both the `git add .` and the `git reset HEAD -- .wolfcastle/` logic in the current `commitDirect`. The `CommitState` flag is handled at file-list construction time rather than as a post-add reset. When `parallel.enabled` is false, `commitDirect` receives `nil` as the scope and falls back to the current `git add .` behavior (with the existing `CommitState` reset logic unchanged).

The `commitAfterIteration` function's pre-commit `git status` check must also be scope-aware: check for changes within the task's scoped files, not globally. Otherwise it sees other workers' changes and never triggers the "no changes" early return.

### Scope validation at commit time

After a task completes and the daemon prepares to commit:

1. Read the task's acquired scope from the scope lock table.
2. Run `git status --porcelain` to get all modified/untracked files.
3. Partition the dirty files into "in scope" and "out of scope."
4. If any code files (outside `.wolfcastle/`) are dirty and out of scope:
   a. **The task fails.** The daemon logs the out-of-scope files, increments the failure count, and does not commit.
   b. The scope locks are released.
   c. The out-of-scope files remain in the working tree. They will be cleaned up by `git checkout -- <files>` to restore the working tree to a clean state for those files.
   d. In-scope files are committed normally (the task did produce valid scoped work, just also wrote outside its scope).
5. If no in-scope dirty files exist and HEAD has not moved from this task's commit, the `HasProgress` check fails (same as today).

Out-of-scope writes are a task failure because silent exclusion produces commits that look complete but are missing files. The agent gets retried and can request a broader scope on the next attempt.

### HasProgress (scope-aware)

The `git.Provider` interface gains a new method:

```go
type Provider interface {
    // existing methods...
    HasProgress(sinceCommit string) bool
    HasProgressScoped(sinceCommit string, scopeFiles []string) bool
}
```

`HasProgressScoped` checks whether any of the `scopeFiles` are modified or untracked. It does not check HEAD movement, because in parallel mode, HEAD can move from a sibling's commit (false positive). The check is purely: "did this task produce changes within its declared scope?"

When `parallel.enabled` is false, the daemon continues calling `HasProgress` (unchanged). When true, it calls `HasProgressScoped` with the task's scope. Test stubs implementing `Provider` need the new method added (returns `true` by default for backwards compatibility).

### Scope lock release

After committing (or failing) a task's changes:

```go
func (pd *ParallelDispatcher) releaseScope(taskAddr string) {
    // MutateScopeLocks acquires namespace lock, reads/writes scope-locks.json
    pd.daemon.store.MutateScopeLocks(func(table *ScopeLockTable) {
        for file, lock := range table.Locks {
            if lock.Task == taskAddr {
                delete(table.Locks, file)
            }
        }
    })
}
```

### Yield handling

Two kinds of yield exist in parallel mode:

1. **Scope-conflict yield.** The agent called `wolfcastle task scope add`, got a conflict error, and emitted `WOLFCASTLE_YIELD`. The agent may or may not have done implementation work (it might have partially implemented before discovering it needs an additional file).

2. **Normal yield.** The agent created subtasks and yielded, or made progress but needs another iteration. Same as today.

Both types commit partial work (if `commit_on_failure` or `commit_on_success` is true, depending on whether a marker was emitted). Both release all scope locks.

The distinction matters for re-dispatch:

- **Scope-conflict yield**: the daemon records the conflicting task address in the `blocked` map. The yielded task is not eligible for re-dispatch until the blocking task completes and its locks are released. This prevents the hot-loop where a yielded task is immediately re-dispatched into the same conflict.
- **Normal yield**: re-dispatch is immediate (next iteration). No backoff needed.

A scope-conflict yield does NOT increment the task's failure count. The task did not fail; it was blocked by a sibling. The failure count is reserved for actual execution failures (no marker, build failures, etc.).

When a yielded task is re-dispatched, it is a fresh model invocation. The agent sees the current codebase state, including its own partial work from the previous attempt (committed before yield) and any completed sibling work.

### Task cancellation

If a running task needs to be cancelled (daemon shutdown, branch verification failure):

1. Cancel the task's per-worker context (derived via `context.WithCancel` from the parent context).
2. The model process receives SIGTERM (existing `ProcessInvoker` behavior via context cancellation).
3. Scope locks are released in the worker's cleanup path.
4. Partial changes are committed if `commit_on_failure` is true.
5. The worker sends its result to the results channel and calls `runWg.Done()`.

During daemon shutdown (SIGINT/SIGTERM), the parent context is cancelled, which cascades to all per-worker contexts. The `Run` function returns; `RunWithSupervisor` calls `runWg.Wait()` to ensure all workers exit before proceeding.

## Navigation Changes

### FindParallelTasks

```go
// FindParallelTasks returns up to maxCount actionable tasks that are siblings
// under the same orchestrator and eligible for parallel execution. Tasks are
// returned in creation order. If no parallel-safe tasks exist, it falls back
// to returning a single task (equivalent to FindNextTask).
func FindParallelTasks(
    idx *RootIndex,
    scopeAddr string,
    loadNode func(addr string) (*NodeState, error),
    maxCount int,
) ([]*NavigationResult, error)
```

The signature matches `FindNextTask`'s conventions: `loadNode` callback (not `*Store`), `scopeAddr` for subtree-scoped searches, and `*NavigationResult` return type (preserving `Description` and `Reason` fields used by the daemon for logging).

### Algorithm

`FindParallelTasks` cannot reuse the existing DFS directly. The serial DFS (`dfs` in `navigation.go`) stops at the first incomplete child that yields no actionable task (line 115: `return nil, nil`). When a sibling is `in_progress` (claimed by another worker), the serial DFS sees it as incomplete-with-no-actionable-task and stops, preventing later siblings from being considered.

The parallel variant needs a modified traversal:

1. **Find the first actionable task** using the existing `FindNextTask` logic. If nothing is found, return empty (the daemon will try planning).
2. **Identify the parent orchestrator.** Look up the task's node in the root index; find its `Parent` field.
3. **If no parent** (root-level node), return just the single task. Cross-orchestrator parallelism is out of scope.
4. **Load the parent's node state** and iterate its `Children` array.
5. **For each sibling** (other children of the same orchestrator):
   a. Skip if the sibling's state in the index is `complete` or `blocked`.
   b. Skip if the sibling is an orchestrator that needs planning (has no children). The unplanned-orchestrator stopping rule is preserved: siblings created after an unplanned orchestrator are not eligible.
   c. Skip if the sibling is already `in_progress` (claimed by another worker).
   d. Load the sibling's node state and call `findActionableTask` on it.
   e. If an actionable task is found, add it to the result set.
   f. Stop when the result set reaches `maxCount`.
6. **Return the results** in creation order (Children array order).

The key difference from serial DFS: step 5c **skips** in-progress siblings instead of **stopping** at them. Serial execution stops because it assumes ordered dependencies. Parallel execution skips because it assumes independence (enforced later by scope locks, not by traversal order).

The unplanned-orchestrator rule (step 5b) is preserved. If sibling B is an unplanned orchestrator, siblings C, D, E after it in the Children array are not eligible. Planning for B must happen first. Siblings A (before B) that are `not_started` are eligible.

## State Mutation Concurrency

### Locking model

The existing single namespace file lock remains the serialization point. Every `MutateNode`, `MutateIndex`, `MutateInbox`, and `MutateScopeLocks` call acquires the same lock. No per-node locks are introduced in this version.

With parallel workers, lock contention increases but remains bounded. Each mutation acquires the lock, performs I/O (read node, apply, write node, read ancestors, recompute, write ancestors, update root index), and releases. The `Propagate` function walks the ancestor chain twice (once to propagate state upward via `PropagateUp`, once to update index entries), roughly doubling the file I/O under a single lock hold compared to a bare mutation. For a node at depth 3 (typical), this is approximately 12 file operations. At local filesystem speeds, this completes in under 50ms.

With 3 workers completing simultaneously, the worst case is 3 sequential lock holds totaling approximately 150ms. Well within the 5-second lock timeout.

### Invariant preservation

- **Parent state derivation.** Propagation re-reads the parent from disk inside the lock. Two concurrent propagations serialize on the namespace lock, so each sees the other's changes. The final parent state is correct.
- **Root index consistency.** Updated inside the namespace lock, after propagation.
- **Scope lock table consistency.** Protected by the same namespace lock.

## Status and Observability

### `wolfcastle status` changes

When parallel execution is active:

```
Workers: 2/3 active

  my-project/api-layer/task-0001 [in_progress]
    scope: internal/api/handler.go, internal/api/routes.go

  my-project/database/task-0001 [in_progress]
    scope: internal/db/

Yielded (waiting on scope):
  my-project/auth/task-0001 -> blocked by my-project/api-layer/task-0001
```

### Logging

Each worker logs with a task-scoped prefix. The NDJSON log format already includes node and task fields; no structural change needed. The parent `Logger` must support concurrent child logger creation (each worker calls `StartIterationWithPrefix` independently to get its own child logger).

### Commit ordering

Commit order may not match task creation order. Sibling B may commit before sibling A if B finishes first. This is expected. Audit tasks and state propagation use task metadata (state files), not git commit ordering, so correctness is unaffected. The commit message includes the task address and ID for traceability.

## Interaction with Existing Features

### Inbox processing

The inbox goroutine runs independently and is unaffected. Inbox intake acquires its own lock and does not conflict with parallel task execution.

### Planning passes

Planning runs only when the worker pool is empty AND no actionable tasks exist. The trigger is explicitly gated: if `len(dispatcher.active) > 0`, skip planning. This prevents planning against a tree that is actively being modified by running workers.

When planning creates new children, those children become available for dispatch in the next `RunOnce` call. The pool fills in step 9 of the dispatch flow.

### Auto-archive

Runs when the pool is empty and no tasks or planning are needed (same trigger as today).

### Branch verification

Runs at the start of each `RunOnce`, before draining results or dispatching workers. If the branch changed, all active workers are cancelled (via context cancellation) and the daemon returns `IterationStop`.

### Commit state flush

`commitStateFlush` runs when the daemon goes idle (no active workers, no tasks, no planning). It acquires `gitMu` and commits any pending `.wolfcastle/` state changes. No change to the trigger or behavior.

## Rollout

### Phase 1: Scope lock infrastructure
- Add `ScopeLockTable` type and `MutateScopeLocks` to `Store`.
- Implement `wolfcastle task scope add/list/release` CLI commands in `cmd/task/`.
- Add `ParallelConfig` to `DaemonConfig` and `config.Defaults()`. Add validation.
- Add stale lock cleanup to `selfHeal` (export or duplicate PID liveness check from `internal/state/filelock.go`).
- Add scope lock display to `wolfcastle status`.
- No daemon dispatch changes. Parallel is not yet enabled.
- All tests remain serial.

### Phase 2: Scope-aware git
- Add `HasProgressScoped` to `git.Provider` interface and `git.Service` implementation.
- Modify `commitDirect` to accept an optional file list (scope). When non-nil, use `git add <files>` and skip the `CommitState` reset logic (handled at file-list construction time). When nil, fall back to current `git add .` behavior.
- Modify `commitAfterIteration` to pass scope-filtered status checks.
- Add `gitMu` to daemon for commit serialization.
- Update test stubs implementing `git.Provider`.

### Phase 3: Parallel dispatch
- Add `ParallelDispatcher` with worker pool, active map, blocked map, results channel.
- Protect shared `Daemon` fields: `atomic.Int64` for `d.iteration`, `sync.Mutex` for `d.lastNoWorkMsg` and `d.lastArchiveCheck`.
- Add `FindParallelTasks` to navigation with the relaxed sibling scanning algorithm.
- Modify `RunOnce` to use the dispatcher when `parallel.enabled` is true.
- Add scope acquisition instructions to `ContextBuilder.Build()` (conditional on config).
- Ensure `Logger.StartIterationWithPrefix` supports concurrent child creation.
- Integration tests with concurrent model invocations.

### Phase 4: Observability and hardening
- Worker status in `wolfcastle status`.
- Scope conflict logging and yield backoff tracking.
- Failure mode testing: scope violations, concurrent propagation, worker cancellation, yield livelock prevention.
- Documentation updates (human docs, agent docs, AGENTS.md).

## What This Does Not Cover

- **Dependency edges between siblings.** Siblings are assumed independent when their file scopes are disjoint. Semantic dependencies (one creates a function, another calls it) are caught by build validation after commit and by the audit task. Explicit dependency declarations are a future enhancement.
- **Cross-orchestrator parallelism.** Only siblings under the same orchestrator are parallelized. Independent subtrees under different orchestrators execute serially (depth-first tree traversal is preserved across orchestrator boundaries).
- **Dynamic worker scaling.** The worker count is static per config. Adaptive scaling based on API rate limits or system load is out of scope.
- **Distributed execution.** All workers run on the same machine, in the same worktree, managed by the same daemon process.
- **Working tree cleanup.** Out-of-scope writes from failed tasks are reverted via `git checkout -- <files>`. Untracked stray files from other sources are not cleaned up automatically. Working tree hygiene beyond scope validation is a future concern.
