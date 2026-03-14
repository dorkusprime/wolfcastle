# wolfcastle audit approve

Approves a pending audit finding. Creates a leaf project from it.

## What It Does

Loads `audit-review.json`, finds the targeted finding (or all pending findings with `--all`), creates a leaf project node from each approved finding, marks the finding as `approved` with a timestamp and node address, and updates the root index.

When all findings in a batch have been decided (approved or rejected), the batch is archived to `audit-review-history.json` (retention: 100 entries, 90 days) and the pending file is removed.

## Usage

```
wolfcastle audit approve <finding-id>
wolfcastle audit approve --all
```

## Flags

| Flag | Description |
|------|-------------|
| `--all` | Approve all pending findings at once. |
| `--json` | Output as structured JSON. |

## Arguments

| Argument | Description |
|----------|-------------|
| `finding-id` | **(Required unless `--all`)** ID of the finding to approve. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Finding(s) approved. |
| 1 | Not initialized. |

## Consequences

- Creates new project node(s) in the [tree](../how-it-works.md#the-project-tree).
- Mutates `audit-review.json`.
- Archives the batch when all findings are decided.

## See Also

- [`wolfcastle audit reject`](audit-reject.md) to discard findings.
- [`wolfcastle audit pending`](audit-pending.md) to review what's waiting.
- [The Approval Gate](../audits.md#the-approval-gate) for how this fits into the audit workflow.
