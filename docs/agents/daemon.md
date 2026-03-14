# Daemon & Pipeline

## Daemon Loop

The daemon (`internal/daemon/daemon.go`) is the heart of Wolfcastle. It runs a supervisor loop (`RunWithSupervisor`) that wraps `Run()` with crash recovery:

```
RunWithSupervisor
  └── Run (main loop)
       ├── selfHeal (ADR-020: find interrupted tasks)
       ├── branch check (ADR-015)
       └── for each iteration:
            └── RunOnce
                 ├── check shutdown/stop-file/iteration-cap
                 ├── navigate to find work (state.FindNextTask)
                 └── runIteration
                      ├── claim task
                      ├── check inbox state
                      ├── run pipeline stages (expand → file → execute)
                      ├── parse WOLFCASTLE_* markers from output
                      ├── sync audit lifecycle
                      └── handle failure thresholds
```

## Key Invariants

- **Serial execution.** Only one task is in_progress at a time (ADR-014).
- **All state mutations are persisted immediately** after marker parsing (line ~394).
- **Propagation after every state change.** `propagateState()` walks ancestors and updates root index.
- **Summary is inline (ADR-036).** No separate summary stage — the executing model emits `WOLFCASTLE_SUMMARY:` alongside `WOLFCASTLE_COMPLETE`.

## Pipeline Stages

Stages are defined in `config.json` under `pipeline.stages`. The daemon processes them sequentially:

| Stage | Model | Purpose | Skip condition |
|-------|-------|---------|----------------|
| `expand` | fast | Break inbox items into tasks | No `"new"` inbox items |
| `file` | mid | Organize/scope tasks | No `"expanded"` inbox items |
| `execute` | heavy | Do the actual work | Expanded items pending filing |

After `expand` runs, inbox state is re-checked so `file` sees freshly expanded items.

## Marker Protocol

The model communicates state mutations via `WOLFCASTLE_*` prefixed lines in stdout:

| Marker | Effect |
|--------|--------|
| `WOLFCASTLE_COMPLETE` | Marks task complete |
| `WOLFCASTLE_BLOCKED: reason` | Blocks task with reason |
| `WOLFCASTLE_YIELD` | Ends iteration without state change |
| `WOLFCASTLE_BREADCRUMB: text` | Adds audit breadcrumb |
| `WOLFCASTLE_GAP: description` | Records audit gap |
| `WOLFCASTLE_FIX_GAP: gap-id` | Marks gap as fixed |
| `WOLFCASTLE_SCOPE: description` | Sets audit scope |
| `WOLFCASTLE_SCOPE_FILES: a\|b\|c` | Sets scope files (pipe-delimited) |
| `WOLFCASTLE_SCOPE_SYSTEMS: a\|b` | Sets scope systems |
| `WOLFCASTLE_SCOPE_CRITERIA: a\|b` | Sets scope criteria |
| `WOLFCASTLE_SUMMARY: text` | Sets result summary (ADR-036) |
| `WOLFCASTLE_RESOLVE_ESCALATION: id` | Resolves an escalation |

## Model Invocation

`internal/invoke` handles subprocess execution:

- `Invoke()` — buffered capture, returns `Result{Stdout, Stderr, ExitCode}`
- `InvokeStreaming()` — streams stdout to a log writer while capturing
- Child processes run in their own process group (`Setpgid: true`) for clean signal propagation
- Scanner buffer is 1MB to handle large model output lines

## Retry & Failure

- **Invocation retries:** exponential backoff, configured in `retries.*`
- **Task failures:** tracked in `task.FailureCount`, thresholds trigger decomposition or auto-block
- **Non-fatal stage errors:** expand/file stage errors are logged but don't halt the daemon

## Concurrency Notes

- The `sync.Once` in Daemon is reset between supervisor restarts — this is safe because all goroutines from the previous `Run()` have exited before reset occurs. The reset is documented in code.
- `d.branch` is written in `Run()` and read in `RunOnce()` — safe because `RunOnce` is only called from within `Run`'s serial loop.
