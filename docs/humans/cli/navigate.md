# wolfcastle navigate

Finds the next actionable task. Read-only. Does not claim it.

## What It Does

Performs a depth-first traversal of the [project tree](../how-it-works.md#the-project-tree), loading each node's `state.json` as it goes. Returns the first task it finds that needs attention:

- A task left `in_progress` from a previous crash (the [self-healing](../failure-and-recovery.md#self-healing) case).
- A `not_started` task ready for [claiming](task-claim.md).
- An audit task whose leaf has all other tasks complete.

Skips `complete` and `blocked` tasks. If nothing is actionable, returns "no work available" with a reason (everything complete, everything blocked, etc.).

## Usage

```
wolfcastle navigate
wolfcastle navigate --node backend/auth
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | Scope traversal to a subtree. Only looks for work under this node. |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success (whether or not a task was found). |
| 1 | Not initialized. |
| 2 | Invalid node path. |

## Consequences

None. This command is strictly read-only. Finding a task and claiming it are separate operations. The [daemon](../how-it-works.md#the-daemon) calls `navigate` then [`task claim`](task-claim.md) as two distinct steps.

## See Also

- [`wolfcastle task claim`](task-claim.md) to take ownership of the found task.
- [The Project Tree](../how-it-works.md#the-project-tree) for traversal order.
