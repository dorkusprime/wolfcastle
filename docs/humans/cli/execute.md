# wolfcastle execute

Runs the execution loop in the foreground with live interleaved output.

## What It Does

Same execution loop as [`wolfcastle start`](start.md), but in the foreground. A background goroutine tails the NDJSON log files as they're written and renders them through the same interleaved renderer used by `wolfcastle log --interleaved --follow`. You see the work happening in real time, one terminal, no separate log session.

Refuses to run if the daemon is already alive. Stop it first or watch the existing session with [`wolfcastle log -i -f`](log.md).

## Usage

```
wolfcastle execute
wolfcastle execute --node auth-module
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | Scope execution to a subtree. Only works on tasks under this node. |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Execution completed (tree conquered or no work found). |
| 1 | Not initialized, daemon already running, or runtime error. |

## Consequences

Identical to [`wolfcastle start`](start.md): claims tasks, invokes models, commits results, propagates state. The only difference is output routing: `execute` streams formatted output to your terminal instead of writing to log files in silence.

## See Also

- [`wolfcastle start`](start.md) to run the daemon in the background.
- [`wolfcastle log`](log.md) to tail daemon output from a separate terminal.
- [`wolfcastle stop`](stop.md) to shut down a running daemon.
