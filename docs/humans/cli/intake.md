# wolfcastle intake

Processes inbox items in the foreground with live interleaved output.

## What It Does

Watches `inbox.json` for new items and runs the intake stage for each one, the same way the daemon would in the background. Output is streamed through the interleaved renderer so you see model output in real time.

Refuses to run if the daemon is already alive. Stop it first.

## Usage

```
wolfcastle intake
wolfcastle intake --node auth-module
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | Scope intake to a subtree. |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Intake loop exited cleanly (signal or context cancellation). |
| 1 | Not initialized, daemon already running, or runtime error. |

## Consequences

Processes inbox items into the project tree. Each item gets triaged by a model invocation that can create projects, orchestrators, and task trees. Commits results to disk.

## See Also

- [`wolfcastle inbox add`](inbox-add.md) to queue items for intake.
- [`wolfcastle inbox list`](inbox-list.md) to see pending items.
- [`wolfcastle execute`](execute.md) for the execution counterpart.
