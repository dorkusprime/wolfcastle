# wolfcastle task block

Marks a task as `blocked` with an explanation. The task will be skipped by [navigation](navigate.md) until explicitly [unblocked](task-unblock.md).

## What It Does

Loads the task's `state.json`, verifies the task is `in_progress` (only claimed tasks can be blocked), transitions it to `blocked`, and records the block reason and timestamp. The task disappears from the daemon's traversal path until a human intervenes.

## Usage

```
wolfcastle task block --node <path> "<reason>"
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Tree address of the task to block. |
| `--json` | Output as structured JSON. |

## Arguments

| Argument | Description |
|----------|-------------|
| `reason` | **(Required)** Why the task cannot proceed. This is stored and displayed in [`status`](status.md) output and fed to models during [unblock](../failure-and-recovery.md#the-unblock-workflow) sessions. Make it useful. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Task blocked. |
| 1 | Not initialized. |
| 2 | Node not found. |
| 3 | Not a leaf task. |
| 4 | Task is not in `in_progress` state. |
| 5 | Empty reason. |

## Consequences

- Mutates task state to `blocked`.
- The task is skipped during [tree traversal](../how-it-works.md#the-project-tree).
- If all non-complete tasks in a subtree are blocked, the parent's state propagates to `blocked` as well.
- The block reason is preserved for diagnostic use in [`wolfcastle unblock`](../failure-and-recovery.md#the-unblock-workflow).

## See Also

- [`wolfcastle task unblock`](task-unblock.md) to clear the block.
- [`wolfcastle unblock`](unblock.md) for model-assisted or agent-assisted unblocking.
- [Failure and Recovery](../failure-and-recovery.md) for the full picture.
