# Parallel Sibling Execution

## Status
Accepted

## Date
2026-03-23

## Context

Wolfcastle processes tasks sequentially: one task claimed per iteration, one model invocation at a time, depth-first through the tree. When an orchestrator has multiple children, each child must fully complete before the next begins. This is correct but slow. Independent children that touch different parts of the codebase could run concurrently.

The "Agents Working Together" coordination system (external reference) demonstrates that multiple agents can safely operate in the same worktree when each agent's file scope is declared and enforced. Their approach uses a human orchestrator to decompose work into "lanes" with explicit file ownership and dependency edges. Wolfcastle needs to achieve the same result autonomously.

Three architectural constraints shape the design:

1. **The daemon owns all git operations** (ADR-093 superseded, deterministic-git spec). Models never run git commands. The daemon commits via `commitDirect` after each iteration. This means concurrent model processes cannot corrupt the git index, because they never touch it.

2. **Planning-time scope prediction is unreliable.** The planning model thinks in abstract responsibilities ("build the API layer"), not concrete file lists. Requiring accurate scope locks at planning time would either over-constrain parallelism or produce incorrect scopes.

3. **The agent closest to the problem has the best information.** The executor reads the codebase and understands which files it needs before writing code. Scope declaration belongs in the execution phase, not the planning phase.

## Options Considered

### 1. Planning-declared scope locks
The planning pass emits file lists per child. The daemon identifies disjoint siblings at dispatch time.

Rejected: the planning model doesn't reliably predict file-level scope. Over-narrow scopes block agents; over-broad scopes eliminate parallelism. Scope would need constant amendment, and the planning prompt already carries heavy responsibilities (decomposition, success criteria, dependency ordering).

### 2. Separate worktree per sibling
Each parallel sibling gets its own git worktree. Full isolation. Merge at completion.

Rejected: merge conflicts between siblings are resolved by whoever finishes second, which requires either manual intervention or a reconciliation model call. Worktree proliferation adds filesystem overhead. The user explicitly preferred same-worktree execution.

### 3. Executor-declared scope via streaming markers
The execution prompt adds a phase where the model emits a `WOLFCASTLE_SCOPE` marker listing files. The daemon captures this from the stdout stream and uses it for parallelism decisions.

Rejected: markers are unidirectional (agent to daemon). The agent cannot learn whether its scope conflicts with a running sibling. No feedback loop, no negotiation. The daemon would need to kill agents whose scopes conflict, wasting model invocations.

### 4. Executor-declared scope via CLI commands (chosen)
The execution prompt adds a scoping phase. The agent calls `wolfcastle task scope` CLI commands to acquire file-level locks before writing code. The CLI checks for conflicts against other running tasks and returns success or failure. The agent gets immediate feedback and can yield if its scope is contested.

## Decision

Option 4. Scope locks are acquired through the existing CLI command infrastructure, stored as ephemeral state, and enforced by the daemon at commit time.

The parallelism model:

1. The daemon launches up to N siblings concurrently (configurable worker limit).
2. Each executor acquires scope locks via `wolfcastle task scope add` during an early execution phase, before writing any code.
3. The CLI checks the scope lock table for conflicts with other running tasks. If the requested files are free, the lock is granted. If contested, the CLI returns an error naming the conflicting task.
4. An agent that cannot acquire its needed scope emits `WOLFCASTLE_YIELD` immediately. The daemon re-queues it for after the conflicting sibling completes.
5. On task completion, the daemon commits only the files in the completing task's acquired scope (replacing `git add .` with `git add <scoped files>`).
6. The daemon releases all scope locks for the completed task.
7. Self-heal releases stale scope locks from crashed or killed tasks.

## Consequences

### What changes

- **DaemonConfig** gains `parallel.enabled` (bool, default false) and `parallel.max_workers` (int, default 3). Serial execution is the default; parallelism is opt-in.
- **Execute prompt** gains a scoping phase between Study and Implement (Phase B.5 or a renamed phase). The agent calls `wolfcastle task scope add` with the files it intends to modify.
- **New CLI commands**: `wolfcastle task scope add`, `wolfcastle task scope list`, `wolfcastle task scope release`.
- **New state**: a scope lock table (per-namespace, ephemeral) mapping file paths to running task addresses.
- **Navigation**: `FindNextTask` is supplemented by `FindParallelTasks` which returns multiple actionable siblings under the same orchestrator.
- **Daemon loop**: the run loop changes from claim-execute-wait to a worker pool model. The main loop fills the pool; workers execute and report results.
- **commitDirect**: changes from `git add .` to `git add <files>` based on the completing task's scope. Sequential commit serialization via a git mutex.
- **HasProgress**: checks dirtiness within the task's scope, not globally.
- **State locking**: the existing namespace lock remains the serialization point. Per-node locks are a future optimization if contention becomes measurable.

### What stays the same

- Planning passes are unchanged. No new planning output format.
- The execution protocol phases (A through J) are unchanged in substance, only augmented with a scoping step.
- Serial execution remains the default. Existing projects work identically.
- The four-state model, depth-first navigation, and state propagation semantics are preserved.
- Marker-based completion (COMPLETE, YIELD, BLOCKED, SKIP) is unchanged.
- The inbox goroutine continues to run independently.

### Risks

- **Scope prediction accuracy.** If an agent declares too narrow a scope, it writes outside its locks and the task fails at commit validation. The agent gets retried and can request a broader scope. This wastes one invocation per scope miss.
- **Thundering herd on namespace lock.** Multiple siblings completing simultaneously all propagate through the same namespace lock. Acceptable because propagation is a fast JSON recompute (read ancestors, update, write), but worth monitoring. The lock hold time includes the full ancestor walk (loading and saving each ancestor in the parent chain), so deep trees amplify contention.
- **Semantic dependencies.** Two siblings with disjoint file scopes can still have semantic dependencies (one creates a function, the other calls it). Scope locks don't catch this. Build validation after commit catches some cases; the audit task catches others. For the first version, this is an accepted limitation.
- **Worker starvation.** If all workers are occupied and one task is slow, the daemon can't start new work. The worker limit is configurable to tune this.
- **Yield livelock.** A yielded task re-dispatched immediately can conflict with a newly-launched sibling and yield again, burning invocations. The daemon suppresses re-dispatch of yielded tasks until the conflicting task's locks are released (see spec for yield backoff).

### Migration

No migration needed. `parallel.enabled` defaults to `false`. Existing projects run in serial mode. Parallelism is activated per-project via config.
