# ADR-020: Daemon Lifecycle and Process Management

## Status
Accepted

## Date
2026-03-12

## Context
Ralph ran as a foreground bash process with a stop file pattern for graceful shutdown. This was simple but had issues: the foreground Claude process swallowed signals, making Ctrl+C unreliable and requiring PID-based kills. Wolfcastle is written in Go, which has significantly better process management capabilities.

## Decision

### Foreground and Background Modes
- `wolfcastle start` — foreground (default). Good for watching, debugging, Ctrl+C.
- `wolfcastle start -d` — background daemon. Forks, returns control to the terminal, writes a PID file.

### Process Management in Go
The Go daemon owns the child process (Claude CLI invocation) lifecycle:
- Child processes are spawned in their own process group
- The daemon intercepts signals (SIGTERM, SIGINT) and propagates them to the child
- Context cancellation coordinates graceful shutdown internally
- No signal swallowing — Go handles signals first, then manages the child

### PID File
When running in background mode (`-d`), Wolfcastle writes `.wolfcastle/wolfcastle.pid`. This is used by `wolfcastle stop` to locate the daemon. Not written in foreground mode (not needed — the process is right there).

### Stop Modes
- `wolfcastle stop` — graceful. Signals the daemon, which finishes the current iteration and exits cleanly.
- `wolfcastle stop --force` — hard kill via PID. For when the daemon or its child is unresponsive.

### Self-Healing on Restart
If Wolfcastle starts and finds a task in `In Progress` state (from a previous crash or hard kill), it navigates to that task and lets the model decide what to do with any uncommitted changes in the working directory. No special recovery logic — the commit-together strategy (state committed alongside code) ensures consistent state.

### Stale PID Detection
On start, if a PID file exists, Wolfcastle checks whether the process is actually running. If the PID is stale (process died without cleanup), the PID file is removed and startup proceeds normally.

### Monitoring
`wolfcastle follow` tails logs regardless of foreground/background mode. `wolfcastle status` shows the current tree state. Both work identically in either mode.

## Consequences
- Foreground mode is the simple default for interactive use
- Background mode (`-d`) enables long-running autonomous execution
- Go's process management eliminates the signal-swallowing problems from Ralph's bash approach
- Graceful stop allows clean iteration completion; force stop is available as escape hatch
- Self-healing restart means crashes don't corrupt state or require manual intervention
- PID file is only written when needed (background mode)
