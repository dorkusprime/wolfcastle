# Goroutine Architecture and Real-Time I/O

This spec defines the communication architecture between Wolfcastle's concurrent goroutines, the signal chain from filesystem events to task execution, and the real-time I/O pipeline for model invocation.

## Governing ADRs

- ADR-020: Daemon Lifecycle and Process Management
- ADR-039: Clean Daemon Iteration Boundary
- ADR-064: Consolidated Intake Stage and Parallel Inbox Processing

---

## 1. Goroutine Topology

The daemon runs two primary goroutines after startup:

```
                    ┌─────────────────┐
                    │   fsnotify      │
                    │  (inbox.json,   │
                    │   stop file)    │
                    └────────┬────────┘
                             │ file event
                             v
┌────────────────────────────────────────────┐
│           Inbox Goroutine                  │
│                                            │
│  Watches for new inbox items. Runs the     │
│  intake stage (model calls wolfcastle CLI  │
│  to create projects and tasks). Signals    │
│  the execute loop when new work lands.     │
└────────────────────┬───────────────────────┘
                     │ workAvailable
                     v
┌────────────────────────────────────────────┐
│           Execute Loop                     │
│                                            │
│  Navigates the tree depth-first. Claims    │
│  tasks. Invokes models via io.Pipe for     │
│  real-time marker detection. Transitions   │
│  state. Propagates to ancestors.           │
└────────────────────────────────────────────┘
```

Both goroutines share:
- **Context**: cancellation propagates shutdown to both
- **Filesystem**: state files, inbox, logs (coordinated via file locking)
- **Logger**: with trace IDs to distinguish output from each goroutine

Neither goroutine touches the other's in-memory state. All coordination flows through the filesystem and one channel.

---

## 2. Channels

### 2.1 workAvailable

```go
workAvailable chan struct{}
```

Non-blocking signal from the inbox goroutine to the execute loop. Sent after the intake stage creates new projects or tasks. The execute loop selects on this channel alongside its idle sleep timer, waking immediately when new work appears instead of waiting for the poll interval.

The channel is buffered with capacity 1. If the execute loop hasn't consumed the previous signal, the send is dropped (the loop will find the work on its next navigation pass regardless).

### 2.2 Context (existing)

The `context.Context` from `signal.NotifyContext` is the shutdown signal. Both goroutines check `ctx.Done()` and exit cleanly. No additional shutdown channel needed.

### 2.3 Stop file (via fsnotify)

The stop file watch replaces the `os.Stat` check in `RunOnce`. When fsnotify detects the stop file's creation, it cancels the context, which propagates to both goroutines.

---

## 3. Inbox Goroutine

### 3.1 Lifecycle

Started by `Run()` after self-healing and branch verification. Stopped by context cancellation.

```go
func (d *Daemon) runInboxLoop(ctx context.Context, workAvailable chan<- struct{}) {
    watcher := fsnotify.NewWatcher()
    watcher.Add(filepath.Join(d.Resolver.ProjectsDir(), "inbox.json"))

    for {
        select {
        case <-ctx.Done():
            return
        case event := <-watcher.Events:
            if event.Op&(fsnotify.Write|fsnotify.Create) == 0 {
                continue
            }
            d.runIntakeIfNeeded(ctx, workAvailable)
        }
    }
}
```

### 3.2 Intake execution

When inbox.json changes:
1. Load inbox, filter to "new" items
2. If no new items, return
3. Start a log iteration (prefixed "intake-NNNN")
4. Invoke the intake model with the new items as context
5. Model calls wolfcastle CLI to create projects and tasks
6. Check root index for new nodes
7. Mark successfully processed items as "filed"
8. Signal `workAvailable` (non-blocking send)

### 3.3 Fallback polling

If fsnotify is unavailable (e.g., the filesystem doesn't support inotify), fall back to polling at `InboxPollIntervalSeconds` (default 5). The same `runIntakeIfNeeded` function runs on each poll tick.

```go
// Fallback when fsnotify is not available
ticker := time.NewTicker(time.Duration(d.Config.Daemon.InboxPollIntervalSeconds) * time.Second)
for {
    select {
    case <-ctx.Done():
        return
    case <-ticker.C:
        d.runIntakeIfNeeded(ctx, workAvailable)
    }
}
```

---

## 4. Execute Loop

### 4.1 Idle wake

The execute loop currently sleeps for `BlockedPollIntervalSeconds` when no work is found. With channels, it selects on both the timer and `workAvailable`:

```go
case IterationNoWork:
    select {
    case <-ctx.Done():
        return nil
    case <-workAvailable:
        // New work arrived from inbox goroutine, loop immediately
    case <-time.After(time.Duration(d.Config.Daemon.BlockedPollIntervalSeconds) * time.Second):
        // Poll timeout, check again
    }
```

This means new inbox items wake the execute loop instantly instead of waiting up to 5 seconds.

### 4.2 No sleep after work

After `IterationDidWork`, the loop continues immediately (already implemented). Combined with the channel wake, the daemon processes new work with zero latency: inbox item arrives, intake stage creates tasks, signals channel, execute loop wakes and starts the task.

---

## 5. Filesystem Watching (fsnotify)

### 5.1 Watched paths

| Path | Event | Action |
|------|-------|--------|
| `projects/{ns}/inbox.json` | Write, Create | Run intake stage |
| `.wolfcastle/stop` | Create | Cancel context (graceful shutdown) |

### 5.2 Dependency

`github.com/fsnotify/fsnotify` (BSD license, no transitive deps, used by Viper/Hugo/Kubernetes). Third external dependency after Cobra and readline.

### 5.3 Platform support

fsnotify uses kqueue (macOS), inotify (Linux), ReadDirectoryChangesW (Windows). All five build targets are supported. If the watcher fails to initialize, the daemon falls back to polling with a log warning.

---

## 6. Real-Time Marker Detection (io.Pipe)

### 6.1 Current behavior

Model stdout is captured into a buffer. After the process exits, `scanTerminalMarker` scans the buffer for WOLFCASTLE_COMPLETE/YIELD/BLOCKED. State transitions happen after the model is done.

### 6.2 New behavior

Model stdout flows through an `io.Pipe`. A scanner goroutine reads lines from the pipe, detects markers in real time, and signals the daemon. State transitions can begin before the model process exits.

```go
pr, pw := io.Pipe()
cmd.Stdout = pw

go func() {
    scanner := bufio.NewScanner(pr)
    for scanner.Scan() {
        line := scanner.Text()
        captured.WriteString(line + "\n")

        // Real-time marker detection
        text := extractAssistantText(line)
        if marker := checkMarker(text); marker != "" {
            markerChan <- marker
        }

        // Stream to log writer
        if logWriter != nil {
            fmt.Fprintln(logWriter, line)
        }
    }
    close(markerChan)
}()
```

### 6.3 Benefits

- Faster state transitions: WOLFCASTLE_COMPLETE is detected mid-stream, not after process exit
- Streaming log output is unified with marker detection (one scanner, not two passes)
- The `result.Stdout` buffer is still fully captured for post-processing (breadcrumbs, gaps, etc.)

### 6.4 Marker priority

Real-time detection sees markers as they arrive. The priority system (COMPLETE > BLOCKED > YIELD) still applies, but now it's evaluated across the full output after the process exits. Real-time detection provides early awareness, not early commitment. The final marker determination uses the same `scanTerminalMarker` function on the complete output.

---

## 7. Trace IDs

### 7.1 Purpose

With two goroutines producing log output, trace IDs distinguish which goroutine and which iteration produced each log line.

### 7.2 Format

```
{iteration_type}-{counter}
```

Examples: `exec-0042`, `intake-0007`

### 7.3 Implementation

The trace ID is set on the logger when starting an iteration:

```go
d.Logger.StartIterationWithPrefix("exec")
// or
d.Logger.StartIterationWithPrefix("intake")
```

Every log record includes a `trace` field:

```json
{"trace": "exec-0042", "type": "stage_start", "stage": "execute", ...}
```

### 7.4 Context propagation

The trace ID is stored in the context via `context.WithValue`. Functions that receive a context can extract the trace ID for logging without needing a reference to the logger.

---

## 8. Custom Error Types

### 8.1 Types

```go
// internal/errors/errors.go (new package)

type ConfigError struct{ Err error }
type StateError struct{ Err error }
type InvocationError struct{ Err error }
type NavigationError struct{ Err error }
```

Each implements `Error() string` and `Unwrap() error`.

### 8.2 Usage in the daemon

```go
case IterationError:
    var invErr *errors.InvocationError
    if errors.As(err, &invErr) {
        // Retryable: model failed, will try again
        d.Logger.Log(map[string]any{"type": "retry", "error": err.Error()})
    }
    var stateErr *errors.StateError
    if errors.As(err, &stateErr) {
        // Fatal: state is corrupt, stop the daemon
        return err
    }
```

### 8.3 Scope

Error types are defined in a new `internal/errors` package to avoid circular imports. Packages that produce errors wrap them in the appropriate type. The daemon inspects error types to decide retry vs abort.

---

## 9. Implementation Phases

### Phase 1: Foundation (complete)

- Custom error types in `internal/errors/` (ADR-065): ConfigError, StateError, InvocationError, NavigationError
- Daemon halts on StateError, retries on InvocationError
- Templates already embedded via go:embed (ADR-033)

### Phase 2: Communication infrastructure (complete)

- `workAvailable` channel (buffered cap 1) on Daemon struct
- Execute loop idle sleep selects on channel, context, and timer
- Inbox goroutine signals workAvailable after successful intake
- Trace IDs in Logger: StartIterationWithPrefix("exec"/"intake") produces "exec-0042", "intake-0007"
- Every log record includes a `trace` field when set

### Phase 3: Real-time I/O (complete)

- fsnotify dependency (github.com/fsnotify/fsnotify v1.9.0)
- Inbox watcher uses fsnotify with polling fallback when unavailable
- Stop file polling remains via os.Stat (fsnotify for stop file deferred; the existing check is cheap and reliable)
- ProcessInvoker already streams stdout via StdoutPipe + bufio.Scanner with real-time marker detection
- Streaming is unified with marker detection in the scanner goroutine

### Phase 4: Optimization (deferred)

Only after profiling shows need.

- sync.Pool for JSON encode/decode buffers
- Benchmark daemon iteration overhead

---

## 10. Concurrency Safety

### 10.1 Shared state

The only shared mutable state between goroutines is the filesystem. In-memory state (node states, root index, inbox data) is loaded fresh on each iteration. No goroutine caches state across iterations.

### 10.2 File locking

The existing `FileLock` (per-namespace advisory locking via flock) provides mutual exclusion for state file writes. Both goroutines acquire the lock before writing state files.

### 10.3 Race conditions

The channel is the only cross-goroutine communication in memory. It's buffered with capacity 1 and uses non-blocking sends, so there's no blocking or deadlock risk. The context is read-only after creation (cancellation is the only mutation, which is thread-safe).

### 10.4 Testing

- Race detector (`go test -race`) must pass with both goroutines active
- Integration tests exercise the parallel path: add inbox item while execute loop is running, verify both complete
- Property: no state file should ever contain partially written JSON (atomic writes via temp+rename guarantee this)
