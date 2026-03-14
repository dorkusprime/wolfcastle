# wolfcastle audit fix-gap

Marks an open audit gap as fixed. The gap stays in the record (nothing gets erased), but its status changes from `open` to `fixed` with a timestamp.

## What It Does

Loads the node's `state.json`, finds the gap by ID, and transitions it from `open` to `fixed`. Records who fixed it and when. Refuses to fix a gap that's already fixed.

## Usage

```
wolfcastle audit fix-gap --node <path> <gap-id>
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Target node. |
| `--json` | Output as structured JSON. |

## Arguments

| Argument | Description |
|----------|-------------|
| `gap-id` | **(Required)** The ID of the gap to fix (e.g., `gap-my-project-1`). |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Gap marked as fixed. |
| 1 | Not initialized, node not found, gap not found, or gap already fixed. |

## Consequences

- Mutates the node's `state.json` audit record.
- Once all gaps are fixed, the [audit task](../audits.md#the-audit-system) can pass.

## See Also

- [`wolfcastle audit gap`](audit-gap.md) to record a new gap.
- [`wolfcastle audit show`](audit-show.md) to see all gaps on a node.
