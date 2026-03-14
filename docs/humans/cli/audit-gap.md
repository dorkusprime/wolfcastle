# wolfcastle audit gap

Records a gap in a node's audit record. Gaps are issues found during audit that need resolution before the audit can pass.

## What It Does

Loads the node's `state.json` and appends a new gap to its `audit.gaps` array. Each gap gets a deterministic ID (e.g., `gap-my-project-1`), a timestamp, and an `open` status. Gaps accumulate until they're fixed with [`audit fix-gap`](audit-fix-gap.md) or escalated with [`audit escalate`](audit-escalate.md).

## Usage

```
wolfcastle audit gap --node <path> "<description>"
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Target node. |
| `--json` | Output as structured JSON. |

## Arguments

| Argument | Description |
|----------|-------------|
| `description` | **(Required)** What the gap is. Cannot be empty. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Gap recorded. |
| 1 | Not initialized, node not found, or empty description. |

## Consequences

- Mutates the node's `state.json` audit record.
- Open gaps prevent the audit from passing. Fix them or escalate them.

## See Also

- [`wolfcastle audit fix-gap`](audit-fix-gap.md) to mark a gap as fixed.
- [`wolfcastle audit escalate`](audit-escalate.md) to push a gap to the parent node.
- [`wolfcastle audit show`](audit-show.md) to see all gaps on a node.
