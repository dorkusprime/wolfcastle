# wolfcastle audit resolve

Marks an open escalation as resolved. The escalation stays in the record for traceability, but its status changes from `open` to `resolved` with a timestamp.

## What It Does

Loads the node's `state.json`, finds the escalation by ID, and transitions it from `open` to `resolved`. Records who resolved it and when. Refuses to resolve an escalation that's already resolved.

## Usage

```
wolfcastle audit resolve --node <path> <escalation-id>
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Target node. |
| `--json` | Output as structured JSON. |

## Arguments

| Argument | Description |
|----------|-------------|
| `escalation-id` | **(Required)** The ID of the escalation to resolve (e.g., `escalation-my-project-1`). |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Escalation resolved. |
| 1 | Not initialized, node not found, escalation not found, or already resolved. |

## Consequences

- Mutates the node's `state.json` audit record.

## See Also

- [`wolfcastle audit escalate`](audit-escalate.md) to push a gap upward.
- [`wolfcastle audit show`](audit-show.md) to see all escalations on a node.
- [Gap Escalation](../audits.md#gap-escalation) for how escalation flows through the tree.
