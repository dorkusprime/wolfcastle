# wolfcastle start

Launches the daemon. This is where work gets done.

## What It Does

Loads and [deep-merges configuration](../configuration.md#three-tiers) from all three tiers. Checks for an existing daemon via PID file; refuses to start if one is already running. Records the current git branch name for [branch safety checks](../collaboration.md#default-behavior).

Runs a startup validation subset (orphaned files, missing indices, stale in-progress states) and blocks on errors. If a task was left `in_progress` from a previous crash, the [self-healing](../failure-and-recovery.md#self-healing) system picks it up first.

Then the loop begins: [navigate](navigate.md) to the next task, run the [pipeline stages](../how-it-works.md#the-pipeline), stream output to NDJSON [logs](../collaboration.md#logging), parse script calls, check for stop signals, repeat. One task at a time. Depth-first. Until the tree is conquered or you tell it to stop.

## Flags

| Flag | Description |
|------|-------------|
| `-d` | Run as a background daemon. Forks, writes PID to `.wolfcastle/wolfcastle.pid`, returns immediately. |
| `--node <path>` | Scope execution to a specific subtree. Only tasks under this node will be worked. |
| `--worktree <branch>` | Run in an isolated [git worktree](../collaboration.md#worktree-isolation). Creates or checks out the specified branch in `.wolfcastle/worktrees/`. Your working directory is never touched. |
| `-v`, `--verbose` | Set daemon console log level to debug. Overrides `daemon.log_level` in config. |
| `--json` | Output as structured JSON. |

`--node` and `--worktree` compose: `wolfcastle start --worktree feature/auth --node backend/auth` gives you an isolated branch scoped to a single subtree.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Work complete or graceful shutdown. |
| 1 | Not initialized or no identity configured. |
| 2 | Another daemon is already running. |
| 3 | Invalid `--node` path. |
| 4 | Git branch changed during execution. |
| 5 | Worktree creation failed. |

## Consequences

- Writes PID file (background mode only).
- Creates NDJSON log files in `.wolfcastle/logs/`.
- Mutates [state files](../how-it-works.md#distributed-state) as tasks progress.
- May create git commits via the execute stage.
- May create worktree directories.

## See Also

- [`wolfcastle stop`](stop.md) to shut it down.
- [`wolfcastle log`](follow.md) to watch it work (`follow` still works as an alias).
- [`wolfcastle status`](status.md) to check progress.
- [The Daemon](../how-it-works.md#the-daemon) for the full execution model.
