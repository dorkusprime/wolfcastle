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
            └── RunOnce (three-step iteration)
                 ├── check shutdown/stop-file/iteration-cap
                 ├── Step 1 (Execute): navigate via FindNextTask, run pipeline if found
                 ├── Step 2 (Plan): if no task, find childless orchestrator, plan it
                 └── Step 3 (Idle): report status, wait for inbox or poll timeout
```

Each `RunOnce` call follows this three-step sequence. Step 1 always runs first: find a task via navigation and execute it. Only when navigation returns nothing does the daemon fall through to Step 2, which looks for an orchestrator with no children and runs the planning stage against it. If neither step finds work, Step 3 reports the idle reason and blocks on either an inbox signal or the poll interval. Planning is lazy; it fires on demand, right before a subtree needs work.

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

`selfHeal()` runs at the start of every `Run()` call. It walks all nodes in the root index and does two things:

1. **Reset stale in_progress tasks.** If a task is in_progress and has children, derive the correct status via `DeriveParentStatus`. If it has no children, reset to not_started (it will be re-claimed next iteration).
2. **Derive parent status from children.** For any task that has children, if the current state disagrees with what the children say, overwrite it with the derived status. This catches parents stuck in not_started or blocked when their children tell a different story. The check applies to all parent tasks, not just in_progress ones.

### PreStartSelfHeal

`PreStartSelfHeal(resolver, wolfcastleDir)` is a standalone function that runs the same two-pass logic (reset stale in_progress, derive parents) without needing a Daemon instance. It can be called before the Daemon is constructed to ensure stale state is corrected before validation gates the startup.

### Pre-start FixWithVerification

Before validation, the start command runs `validate.FixWithVerification` in multi-pass mode (up to 5 passes per ADR-051). Each pass validates the tree, applies deterministic fixes, and re-validates until no fixable issues remain. The call omits `wolfcastleDir` so the engine skips daemon artifact checks (PID file, stop file); those are intentional at startup, not stale leftovers.

The sequence in `cmd/daemon/start.go` is: recover stale PID → `FixWithVerification` → startup validation gate → construct Daemon → `RunWithSupervisor`. The Daemon's own `selfHeal` runs inside `Run()` after construction.

## Key Invariants

- **Serial execution.** Only one task is in_progress at a time (ADR-014).
- **State reloaded after invocation.** The daemon reloads state.json from disk after each model invocation to pick up mutations made by the model's CLI subprocesses (ADR-067).
- **Propagation after every state change.** `propagateState()` re-reads the root index from disk, applies the state change, and saves.
- **Summary via CLI or marker.** The model can call `wolfcastle audit summary` (ADR-067) or emit `WOLFCASTLE_SUMMARY:` inline (ADR-036). The invoke package detects the marker and stores the text on `Result.Summary`.

## Pipeline Stages

Stages are defined in `config.json` under `pipeline.stages`. The default pipeline has two stages:

| Stage | Model | Purpose |
|-------|-------|---------|
| `intake` | mid | Reads inbox items, calls wolfcastle CLI to create projects/tasks |
| `execute` | heavy | Does the actual work on a single task per iteration |

The intake stage runs in a parallel goroutine and is skipped in the main iteration pipeline. The execute stage runs in the main loop.

## Terminal Markers (ADR-067)

The model signals iteration completion via stdout markers. These are the only markers the daemon parses:

| Marker | Effect |
|--------|--------|
| `WOLFCASTLE_COMPLETE` | Marks task complete |
| `WOLFCASTLE_BLOCKED` | Blocks task |
| `WOLFCASTLE_YIELD` | Ends iteration, task stays in_progress |

All other state mutations (breadcrumbs, gaps, scope, summary, escalations) are made by the model calling wolfcastle CLI commands during execution. The daemon reloads state from disk after invocation to pick up these changes.

## State I/O (ADR-068)

All state file mutations should go through the `StateStore` (lock, read, callback, atomic write, unlock). This prevents the read-modify-write races that occur when the daemon and model CLI subprocesses write to the same files concurrently. See the spec at `docs/specs/2026-03-15T00-01Z-state-store.md`.

## Model Invocation

`internal/invoke` handles subprocess execution:

- `Invoke()`: buffered capture, returns `Result{Stdout, Stderr, ExitCode}`
- `InvokeStreaming()`: streams stdout to a log writer while capturing
- Child processes run in their own process group (`Setpgid: true`) for clean signal propagation
- Scanner buffer is 1MB to handle large model output lines

## Retry & Failure

- **Invocation retries:** exponential backoff, configured in `retries.*`
- **Task failures:** tracked in `task.FailureCount`, thresholds trigger decomposition or auto-block
- **Non-fatal stage errors:** intake stage errors are logged but don't halt the daemon

## Concurrency Notes

- The `sync.Once` in Daemon is reset between supervisor restarts. This is safe because all goroutines from the previous `Run()` have exited before reset occurs. The reset is documented in code.
- `d.branch` is written in `Run()` and read in `RunOnce()`. Safe because `RunOnce` is only called from within `Run`'s serial loop.
- The inbox goroutine and the main loop both access `inbox.json` and the project tree. Safety is provided by the StateStore's lock-read-mutate-write pattern (ADR-068). The model's CLI subprocesses also write state files during execution; the daemon reloads from disk after invocation returns.
