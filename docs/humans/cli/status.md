# wolfcastle status

Shows the current state of your work. Read-only.

## What It Does

Loads the root [state index](../how-it-works.md#distributed-state) and walks the tree, computing summary statistics: total tasks, completed, in-progress, blocked, pending. Identifies the currently active task (if any). Lists blocked tasks with their reasons. Checks whether a daemon is running via PID file and process lookup.

With `--all`, scans every engineer's [namespace](../collaboration.md#engineer-namespacing) under `projects/` and aggregates results grouped by engineer.

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | Show status for a specific subtree only. |
| `--all` | Aggregate status across all engineer namespaces. |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success. |
| 1 | Not initialized or identity not configured. |

## Consequences

None. This command is strictly read-only.

## See Also

- [`wolfcastle log`](follow.md) to watch daemon output in real time (`follow` still works as an alias).
- [The Project Tree](../how-it-works.md#the-project-tree) for how the tree is structured.
