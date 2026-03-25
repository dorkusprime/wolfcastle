# Daemon & Pipeline

## Daemon Loop

The daemon (`internal/daemon/daemon.go`) is the heart of Wolfcastle. It runs a supervisor loop (`RunWithSupervisor`) that wraps `Run()` with crash recovery:

```
RunWithSupervisor
  └── Run (main loop)
       ├── selfHeal (ADR-020: find interrupted tasks, derive parents)
       ├── branch check (ADR-015)
       ├── snapshotDeliverables (baseline hashes for change detection)
       ├── start inbox goroutine (ADR-064: parallel intake)
       ├── start spinner goroutine
       └── for each iteration:
            └── RunOnce (four-step iteration)
                 ├── check shutdown/stop-file/iteration-cap
                 ├── Step 1 (Execute): navigate via FindNextTask, run pipeline if found
                 ├── Step 2 (Plan): if no task, find childless orchestrator, plan it
                 ├── Step 3 (Auto-archive): if no task and no planning, archive one eligible node
                 └── Step 4 (Idle): report status, wait for inbox or poll timeout
```

Each `RunOnce` call follows this four-step sequence. Step 1 always runs first: find a task via navigation and execute it. Only when navigation returns nothing does the daemon fall through to Step 2, which looks for an orchestrator with no children and runs the planning stage against it. If neither step finds work, Step 3 checks for completed nodes eligible for archival (see Auto-Archive below). If nothing remains, Step 4 reports the idle reason and blocks on either an inbox signal or the poll interval. Planning is lazy; it fires on demand, right before a subtree needs work.

When Step 1 does find a task, `runIteration` handles it:

```
runIteration
  ├── claim task
  ├── PauseSpinner
  ├── run execute stage
  ├── ResumeSpinner
  ├── reclaim foreground process group
  ├── reload state from disk (model CLI calls may have mutated it)
  ├── scan for terminal markers (COMPLETE/YIELD/BLOCKED)
  ├── checkDeliverablesChanged
  └── handle failure thresholds
```

## Spinner Coordination

The daemon runs a spinner animation in a background goroutine to signal liveness while waiting between iterations. Before invoking the model subprocess, the daemon calls `PauseSpinner()` to stop the animation (the model's own output takes the terminal). After invocation returns, `ResumeSpinner()` restarts it. Both calls are synchronous; the spinner goroutine blocks on a channel until told to resume.

## Deliverable Baseline Hashes

At startup, `snapshotDeliverables()` walks the project tree and records a hash of each deliverable file. After each iteration, `checkDeliverablesChanged()` re-hashes and compares. This detects when the model has produced or modified deliverables during execution, even if no state marker was emitted. Changed files are logged.

## Foreground Process Group Reclaim

Model invocations run in their own process group (`Setpgid: true`). When the subprocess exits, the daemon reclaims the foreground process group so that terminal signals (SIGINT, SIGTSTP) route back to the daemon rather than being swallowed by the defunct child group.

## Discovery-First Intake Pipeline

Inbox processing uses a discovery-first approach: new inbox items are scanned and decomposed into project/task structures before any task execution begins. The intake model creates the nodes and marks new tasks as `not_started` with a pre-block so the daemon does not pick them up mid-decomposition. Once intake completes, the pre-block is removed and the tasks become eligible for execution during normal navigation.

## Parallel Inbox Processing (ADR-064)

Inbox processing runs in a background goroutine started by `Run()`. The goroutine polls `inbox.json` at the configured interval and runs the intake stage when new items are found. This decouples inbox processing from task execution so neither blocks the other.

## Self-Healing

`selfHeal()` runs at the start of every `Run()` call. It walks all nodes in the root index and does three things:

1. **Reset stale in_progress tasks.** If a task is in_progress and has children, derive the correct status via `DeriveParentStatus`. If it has no children, reset to not_started (it will be re-claimed next iteration).
2. **Derive parent status from children.** For any task that has children, if the current state disagrees with what the children say, overwrite it with the derived status. This catches parents stuck in not_started or blocked when their children tell a different story. The check applies to all parent tasks, not just in_progress ones.
3. **Blocked audit remediation.** If an audit task is blocked with open gaps but no remediation subtasks exist, selfHeal creates them. This catches daemon crashes that occurred after an audit blocked but before subtask creation completed. The audit task is reset to not_started so it re-runs to verify fixes once the remediation subtasks finish.

### PreStartSelfHeal

`PreStartSelfHeal(resolver, wolfcastleDir)` is a standalone function that runs the same two-pass logic (reset stale in_progress, derive parents) without needing a Daemon instance. It can be called before the Daemon is constructed to ensure stale state is corrected before validation gates the startup.

### Pre-start FixWithVerification

Before validation, the start command runs `validate.FixWithVerification` in multi-pass mode (up to 5 passes per ADR-051). Each pass validates the tree, applies deterministic fixes, and re-validates until no fixable issues remain. The call omits `wolfcastleDir` so the engine skips daemon artifact checks (PID file, stop file); those are intentional at startup, not stale leftovers.

The sequence in `cmd/daemon/start.go` is: recover stale PID → `FixWithVerification` → startup validation gate → construct Daemon → `RunWithSupervisor`. The Daemon's own `selfHeal` runs inside `Run()` after construction.

## Key Invariants

- **Serial execution by default.** Only one task is in_progress at a time (ADR-014) unless `parallel.enabled` is true, in which case up to `parallel.max_workers` tasks run concurrently with file-level scope locks (ADR-095).
- **State reloaded after invocation.** The daemon reloads state.json from disk after each model invocation to pick up mutations made by the model's CLI subprocesses (ADR-067).
- **Propagation after every state change.** `propagateState()` re-reads the root index from disk, applies the state change, and saves.
- **Summary via CLI or marker.** The model can call `wolfcastle audit summary` (ADR-067) or emit `WOLFCASTLE_SUMMARY:` inline (ADR-036). The invoke package detects the marker and stores the text on `Result.Summary`.

## Pipeline Stages

Stages are stored in `PipelineConfig` as a `map[string]PipelineStage` dict keyed by stage name, paired with a `StageOrder []string` slice that controls execution order. The daemon iterates `StageOrder` and looks up each stage's configuration from the map. The default pipeline has two stages:

| Stage | Model | Purpose |
|-------|-------|---------|
| `intake` | mid | Reads inbox items, calls wolfcastle CLI to create projects/tasks |
| `execute` | heavy | Does the actual work on a single task per iteration |

The intake stage runs in a parallel goroutine (ADR-064) and is skipped during the main iteration pipeline. Custom stages and execute run in `StageOrder` sequence during each iteration.

## Spec Review

When a spec-type task completes, `checkSpecReviewNeeded()` in `internal/daemon/spec_review.go` automatically creates a sibling review task. The review task ID is deterministic (`{specTaskID}-review`), making the operation idempotent. The review task carries `TaskType: "spec-review"`, references the same spec files as the original, and uses a dedicated `spec-review.md` prompt template. It is inserted before the audit task in task ordering so the spec gets reviewed before the node's audit runs.

If the review task blocks, `handleSpecReviewBlocked()` feeds the blocked reason back into the original spec task's body as a `## Review Feedback (Revision Required)` section and resets the spec task to not_started, sending it through another revision cycle.

## After Action Reviews (AARs)

Each completed task records an AAR on its parent node's state (`NodeState.AARs`, keyed by task ID). The AAR struct captures objective, what happened, what went well, improvements, and action items. AARs replace terse breadcrumbs as the primary audit input for gap detection, giving auditors a structured narrative of what each task accomplished and what doubts remain. The `AddAAR` mutation in `internal/state/mutations.go` handles storage.

## Codebase Knowledge Injection

The `ContextBuilder` reads the codebase knowledge file for the current engineer namespace (`.wolfcastle/docs/knowledge/<namespace>.md`) and injects it into the iteration context under a `## Codebase Knowledge` heading. It appears after class guidance and before AARs. The file is read fresh every iteration (not cached), so entries added by one task are immediately visible to the next.

Knowledge files accumulate informal codebase observations that don't belong in ADRs (design decisions) or specs (contracts): build environment quirks, undocumented conventions, intentional-looking oddities, and cross-module dependencies that aren't obvious from the code. If no knowledge file exists for the namespace, the section is omitted from the context. See the `knowledge` commands in `cmd/knowledge/` and the `internal/knowledge/` package for the implementation.

## Terminal Markers (ADR-067)

The model signals iteration completion via stdout markers. These are the only markers the daemon parses:

| Marker | Effect |
|--------|--------|
| `WOLFCASTLE_COMPLETE` | Marks task complete |
| `WOLFCASTLE_BLOCKED` | Blocks task |
| `WOLFCASTLE_YIELD` | Ends iteration, task stays in_progress |

All other state mutations (breadcrumbs, gaps, scope, summary, escalations) are made by the model calling wolfcastle CLI commands during execution. The daemon reloads state from disk after invocation to pick up these changes.

## State I/O (ADR-068)

All state file mutations should go through the `Store` (lock, read, callback, atomic write, unlock). This prevents the read-modify-write races that occur when the daemon and model CLI subprocesses write to the same files concurrently. See the spec at `docs/specs/2026-03-15T00-01Z-state-store.md`.

## Model Invocation

`internal/invoke` handles subprocess execution:

- `Invoke()`: buffered capture, returns `Result{Stdout, Stderr, ExitCode}`
- `InvokeStreaming()`: streams stdout to a log writer while capturing
- Child processes run in their own process group (`Setpgid: true`) for clean signal propagation
- Scanner buffer is 1MB to handle large model output lines

### Stall Detector

`ProcessInvoker` carries a `StallTimeout` duration field. When set and streaming mode is active (logWriter or onLine callback provided), a timer starts at process launch and resets on every output line. If no output arrives within the timeout, the invoker cancels the process context, killing the entire process group. The result carries `ErrStallTimeout` with whatever partial output was captured before the stall. This prevents hung model processes (API instability, network partitions) from blocking the daemon indefinitely.

## Retry & Failure

- **Invocation retries:** exponential backoff, configured in `retries.*`
- **Task failures:** tracked in `task.FailureCount`, thresholds trigger decomposition or auto-block
- **Non-fatal stage errors:** intake stage errors are logged but don't halt the daemon

## Concurrency Notes

- The `sync.Once` in Daemon is reset between supervisor restarts. This is safe because all goroutines from the previous `Run()` have exited before reset occurs. The reset is documented in code.
- `d.branch` is written in `Run()` and read in `RunOnce()`. Safe because `RunOnce` is only called from within `Run`'s serial loop.
- The inbox goroutine and the main loop both access `inbox.json` and the project tree. Safety is provided by the Store's lock-read-mutate-write pattern (ADR-068). The model's CLI subprocesses also write state files during execution; the daemon reloads from disk after invocation returns.

## Auto-Archive

Completed nodes are archived inline during RunOnce, not in a separate goroutine (per ADR). Archive checking runs as Step 3 in the iteration sequence, after task execution and planning but before the idle report.

A node is eligible for archival when: it lives in the active root (not already archived), its state is complete, and its `Audit.CompletedAt` timestamp is older than the configured `AutoArchiveDelayHours`. The `findArchiveEligible()` function in `internal/daemon/archive.go` evaluates these criteria.

`tryAutoArchive()` enforces two bounds: a `lastArchiveCheck` timestamp on the Daemon struct throttles checks to one per poll interval, and at most one node is archived per RunOnce call. The archive operation itself generates a markdown rollup via `GenerateEntry()`, moves the node's state directory into `.archive/`, and atomically updates the root index (removes from Root, adds to ArchivedRoot, sets `Archived` and `ArchivedAt`).

The tradeoff is that archive checks only fire when the daemon is idle. A constantly-busy daemon won't archive until it runs out of work, which is acceptable since archival is not time-sensitive.
