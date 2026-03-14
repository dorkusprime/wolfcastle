# wolfcastle stop

Stops a running daemon.

## What It Does

Finds the PID file at `.wolfcastle/wolfcastle.pid` and sends a signal to the daemon process. Without `--force`, sends SIGTERM and lets the daemon finish its current pipeline stage before shutting down. With `--force`, sends SIGKILL for immediate termination, killing child model processes along with it.

If the PID file exists but the process is gone, removes the stale PID file and reports the stale state. If no PID file exists, reports that no daemon was found.

## Flags

| Flag | Description |
|------|-------------|
| `--force` | SIGKILL instead of SIGTERM. Immediate termination. The current task may be left in an inconsistent state (the [self-healing](../failure-and-recovery.md#self-healing) system will handle it on next start). |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Stopped successfully, or no daemon was running. |
| 1 | Not initialized, or signal delivery failed. |

## Consequences

- Sends a signal to the daemon process. The daemon handles its own cleanup (PID file, worktrees, child processes) during shutdown.
- With `--force`: the in-progress task may need [self-healing](../failure-and-recovery.md#self-healing) on next [`start`](start.md).

## See Also

- [`wolfcastle start`](start.md) to launch the daemon.
- [`wolfcastle status`](status.md) to check if a daemon is running.
