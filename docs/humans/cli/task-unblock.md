# wolfcastle task unblock

Clears a blocked task. Resets it to `not_started` with a fresh failure counter. This is [Tier 1](../failure-and-recovery.md#tier-1-status-flip) of the unblock workflow: zero cost, no model involved.

## What It Does

Loads the task's `state.json`, verifies the task is `blocked`, resets it to `not_started`, zeros out the failure counter, clears the block reason, and records an unblock timestamp.

The task goes back to `not_started`, not `in_progress`. It will be re-evaluated from scratch on the next daemon iteration. No blind resumption.

## Usage

```
wolfcastle task unblock --node <path>
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Tree address of the blocked task. |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Task unblocked. |
| 1 | Not initialized. |
| 2 | Node not found. |
| 3 | Not a leaf task. |
| 4 | Task is not in `blocked` state. |

## Consequences

- Mutates task state to `not_started`.
- Resets failure counter to 0.
- Clears block reason.
- The task becomes eligible for [navigation](navigate.md) again.
- Parent state may propagate from `blocked` to `in_progress` as a result.

## See Also

- [`wolfcastle unblock`](unblock.md) for Tier 2 (model-assisted) and Tier 3 (agent context dump) unblocking.
- [`wolfcastle task block`](task-block.md) for how tasks get blocked.
- [The Unblock Workflow](../failure-and-recovery.md#the-unblock-workflow) for all three tiers.
