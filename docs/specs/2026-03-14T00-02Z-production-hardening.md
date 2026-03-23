# Production Hardening

This spec covers the remaining production-readiness concerns not addressed in dedicated specs: state file locking, structured log levels, graceful force-stop, error message standards, and the contributor guide. It is the implementation reference for ADR-042, ADR-046, and ADR-048.

## Governing ADRs

- ADR-020: Daemon Lifecycle and Process Management
- ADR-042: State File Locking
- ADR-046: Structured Log Levels
- ADR-048: Interactive Session UX

---

## 1. State File Locking

### Lock Mechanism

Advisory file locking using `syscall.Flock` (Unix) or `LockFileEx` (Windows) via a portable Go wrapper.

### Lock File

```
.wolfcastle/system/projects/{namespace}/.lock
```

One lock per engineer namespace. The lock file is created on first acquisition and never deleted (zero-length sentinel file).

### Lock Protocol

```go
// acquireLock opens the lock file and acquires an exclusive flock.
// Returns a cleanup function that releases the lock.
func acquireLock(lockPath string, timeout time.Duration) (func(), error)
```

#### Daemon (RunOnce)

```
acquire lock
  → load state
  → execute iteration
  → save state + propagate
release lock
```

The lock is held for the duration of the iteration. Between iterations (during sleep), the lock is released: allowing CLI commands to execute.

#### CLI Commands (mutating)

```
acquire lock (5s timeout)
  → load state
  → apply mutation
  → save state
release lock
```

If the lock cannot be acquired within the timeout, the command fails:

```
Error: could not acquire state lock: the daemon may be processing.
Try again in a few seconds, or check 'wolfcastle status'.
```

#### CLI Commands (read-only)

No lock acquired. Read-only commands (`status`, `pending`, `spec list`, `follow`) read state files directly. They may see a slightly stale view during a daemon iteration: this is acceptable because:
- Status is advisory, not transactional
- The daemon will finish its iteration within seconds
- No mutations means no conflict risk

### Commands That Acquire the Lock

| Command | Mutates state |
|---------|--------------|
| `task add` | Yes: adds task to node state |
| `task claim` | Yes: transitions task state |
| `task complete` | Yes: transitions task + propagates |
| `task block` | Yes: transitions task state |
| `task unblock` | Yes: transitions task state |
| `project create` | Yes: creates node + updates index |
| `audit approve` | Yes: creates project + updates index |
| `audit reject` | Yes: updates batch |
| `audit gap` | Yes: adds gap to node state |
| `audit fix-gap` | Yes: updates gap status |
| `audit escalate` | Yes: adds escalation |
| `audit resolve` | Yes: updates escalation status |
| `audit breadcrumb` | Yes: adds breadcrumb |
| `audit scope` | Yes: updates audit scope |
| `doctor --fix` | Yes: repairs state |
| `archive add` | No: reads state only |
| `status` | No |
| `pending` | No |
| `history` | No |
| `spec list` | No |
| `follow` | No |

### Lock Timeout Configuration

```json
{
  "daemon": {
    "lock_timeout_seconds": 5
  }
}
```

Default: 5 seconds. CLI commands use this timeout. The daemon uses a longer internal timeout (30 seconds) to handle slow filesystem operations.

### Stale Lock Recovery

If the daemon crashes while holding the lock, the lock is automatically released by the OS when the process exits (flock is process-scoped, not file-scoped). No manual recovery is needed.

---

## 2. Structured Log Levels

### NDJSON Record Format

Every log record gains a `level` field:

```json
{
  "timestamp": "2026-03-14T18:45:32Z",
  "level": "info",
  "type": "stage_start",
  "stage": "execute",
  "model": "heavy",
  "node": "auth-system/token-refresh"
}
```

### Level Definitions

| Level | Meaning | Examples |
|-------|---------|---------|
| `debug` | Verbose operational detail | Stage skip reasons, inbox state checks, iteration context, model output lines |
| `info` | Normal operational events | Stage start/complete, iteration start, daemon start/stop, item counts |
| `warn` | Potential issues, non-fatal errors | Retry attempts, stale PID, validation warnings, non-fatal stage errors |
| `error` | Failures requiring attention | Fatal errors, retry exhaustion, state corruption, invocation failures |

### Level Filtering

- **NDJSON files** always capture all levels (debug through error). The log file is the complete, unfiltered record.
- **Console output** filters by the configured level. Default: `info`.
- **`wolfcastle follow`** shows all levels (it tails the NDJSON file directly).

### Configuration

```json
{
  "daemon": {
    "log_level": "info"
  }
}
```

The `--verbose` / `-v` flag on `wolfcastle start` overrides `log_level` to `debug`.

### Logger API Changes

```go
// Log writes a record at info level (backward compatible).
func (l *Logger) Log(record map[string]any) error

// LogAt writes a record at the specified level.
func (l *Logger) LogAt(level string, record map[string]any) error
```

Existing `Log()` calls continue to work without modification: they default to `info`.

### Console Output Filtering

The daemon's output.PrintHuman calls are associated with log levels:

```go
// Only print to console if level >= configured threshold
func (d *Daemon) consoleLog(level, format string, args ...any) {
    d.Logger.LogAt(level, map[string]any{"type": "console", "message": fmt.Sprintf(format, args...)})
    if levelOrd(level) >= levelOrd(d.Config.Daemon.LogLevel) {
        output.PrintHuman(format, args...)
    }
}
```

---

## 3. Force Stop

ADR-020 specifies `wolfcastle stop --force`, which is fully implemented. The implementation:

### Graceful Stop (existing)

```
wolfcastle stop
  → write .wolfcastle/stop file
  → daemon detects stop file on next iteration boundary
  → daemon finishes current stage, then exits
```

### Force Stop (new)

```
wolfcastle stop --force
  → read PID from .wolfcastle/wolfcastle.pid
  → send SIGKILL to the process group (kills daemon + child model process)
  → remove PID file
  → remove stop file (if present)
  → output "Wolfcastle force-stopped (PID {pid})"
```

Force stop uses the process group (negative PID) to kill both the daemon and any child process (model CLI). This is necessary because SIGKILL cannot be caught: the daemon can't propagate it to the child.

### Safety

Force stop is destructive: the model's in-flight work is lost, and uncommitted changes remain in the working directory. The command warns:

```
Force-stopping Wolfcastle (PID 12345)...
Warning: in-flight work may be lost. Run 'wolfcastle doctor' to check state consistency.
Wolfcastle force-stopped.
```

---

## 4. Error Message Standards

### Format

All error messages follow these conventions:

- **Lowercase start**: `"loading config: file not found"` not `"Loading config: ..."`
- **No trailing period**: `"invalid node address"` not `"invalid node address."`
- **Context chain with colons**: `"task complete: loading node state: read /path: permission denied"`
- **Actionable when possible**: include what the user can do: `"identity not configured: run 'wolfcastle init' first"`
- **No stack traces**: error wrapping provides the chain; stack traces are for panics only

### Required Context

Every error message should answer: *what failed, and what can the user do about it?*

| Bad | Good |
|-----|------|
| `"error"` | `"loading config: .wolfcastle/system/base/config.json not found: run 'wolfcastle init'"` |
| `"invalid input"` | `"--node must be a task address (e.g. my-project/task-1)"` |
| `"not found"` | `"task task-3 not found in my-project: use 'wolfcastle status --node my-project' to list tasks"` |

### Flag Validation

Required flags that are empty produce errors in this format:

```
--node is required: specify the task address (e.g. my-project/task-1)
```

Not:

```
--node is required
```

---

## 5. Interactive Session UX

### readline Integration

Replace the raw `bufio.Scanner` loop in `cmd/unblock.go` with `github.com/chzyer/readline`:

```go
rl, err := readline.NewEx(&readline.Config{
    Prompt:          "wolfcastle> ",
    InterruptPrompt: "^C",
    EOFPrompt:       "exit",
})
defer rl.Close()

for {
    line, err := rl.Readline()
    if err != nil { // io.EOF or interrupt
        break
    }
    // process line...
}
```

### Behavior

| Input | Effect |
|-------|--------|
| Arrow keys | Navigate within the line / recall history |
| Enter | Submit the current line |
| Ctrl+C | Cancel current line (not the session) |
| Ctrl+D | Exit the session |
| `quit` / `exit` | Exit the session |
| Empty line | Exit the session |
| Up/Down | Navigate session history |

### History

In-memory only. Not persisted across sessions. Unblock sessions are short-lived: persistent history adds complexity without value.

### Prompt

```
wolfcastle>
```

Consistent, branded, no ambiguity about what's accepting input.

---

## 6. Contributing Guide

A `CONTRIBUTING.md` at the project root covering:

1. **Development setup**. Go version, `make build`, `make test`
2. **Code standards**: link to `docs/agents/code-standards.md`
3. **Testing**: how to run each test tier, how to write new tests
4. **Pull request process**: branch naming, commit messages, CI expectations
5. **ADR process**: when to write one, format, numbering
6. **Issue reporting**: what to include, labels

This is a living document: it grows as the contributor community grows.
