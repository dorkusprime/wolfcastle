# wolfcastle audit escalate

Escalates a gap to the parent node's audit scope. When a leaf audit finds something it cannot resolve locally, this pushes the problem upward.

## What It Does

Loads both the source node's and parent node's `state.json` files. Appends the gap description to the parent's `escalations` array with a timestamp and the source node address. Also records the escalation on the source node for traceability.

Escalation can chain. A parent's audit can escalate to its own parent, all the way to the root if necessary. See [Gap Escalation](../audits.md#gap-escalation).

Cannot be called on the root node (nothing to escalate to).

## Usage

```
wolfcastle audit escalate --node <path> "<gap>"
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Source node where the gap was found. |
| `--json` | Output as structured JSON. |

## Arguments

| Argument | Description |
|----------|-------------|
| `gap` | **(Required)** Description of the gap. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Gap escalated. |
| 1 | Not initialized. |
| 2 | Node not found. |
| 3 | Node is the root (cannot escalate). |
| 4 | Empty gap description. |

## Consequences

- Mutates both the source node's and parent node's `state.json`.
- The parent's [audit task](../audits.md#the-audit-system) will now include cross-cutting verification of this gap.

## See Also

- [`wolfcastle audit breadcrumb`](audit-breadcrumb.md) for recording what was done.
- [Gap Escalation](../audits.md#gap-escalation) for how escalation flows through the tree.
