# wolfcastle task claim

Marks a task as `in_progress`. This is how a model takes ownership of a task during [execution](../how-it-works.md#execution-protocol).

## What It Does

Loads the task's `state.json`, verifies the task is `not_started`, transitions it to `in_progress`, and records a claim timestamp. Rejects the call if the task is already claimed, completed, or blocked.

This is typically called by the daemon during the execute stage, not by humans directly. But you can use it if you're driving things manually.

## Usage

```
wolfcastle task claim <task-address>
wolfcastle task claim --node <task-address>
```

The task address can be given as a positional argument or via `--node`. Both forms are equivalent. Providing both is an error.

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | Tree address of the task to claim. Alias for the positional argument. |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Task claimed. |
| 1 | Not initialized. |
| 2 | Node not found. |
| 3 | Not a leaf task. |
| 4 | Task is not in `not_started` state. |

## Consequences

- Mutates task state to `in_progress`.
- Records claim timestamp.
- The [daemon](../how-it-works.md#the-daemon) will not attempt to claim a task that is already in progress (unless recovering from a [crash](../failure-and-recovery.md#self-healing)).

## See Also

- [`wolfcastle task complete`](task-complete.md) when the work is done.
- [`wolfcastle task block`](task-block.md) when the work cannot proceed.
- [Execution Protocol](../how-it-works.md#execution-protocol) for where this fits in the model's workflow.
