# wolfcastle task complete

Marks a task as `complete`. Terminal state. It never comes back.

## What It Does

Loads the task's `state.json`, verifies the task is `in_progress`, transitions it to `complete`, and records a completion timestamp. Returns whether the parent leaf is now ready for its [audit task](../audits.md#the-audit-system) (i.e., all non-audit tasks are complete).

## Usage

```
wolfcastle task complete --node <path>
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Tree address of the task to complete. |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Task completed. |
| 1 | Not initialized. |
| 2 | Node not found. |
| 3 | Not a leaf task. |
| 4 | Task is not in `in_progress` state. |

## Consequences

- Mutates task state to `complete`. This is irreversible.
- Triggers [state propagation](../how-it-works.md#state-propagation) readiness: the parent leaf recomputes, then its parent, up to the root.
- If this was the last non-audit task in the leaf, the audit task becomes eligible for execution.

## See Also

- [`wolfcastle task claim`](task-claim.md) to take ownership first.
- [`wolfcastle audit breadcrumb`](audit-breadcrumb.md) to record what was done before completing.
- [State Propagation](../how-it-works.md#state-propagation) for how completion ripples upward.
