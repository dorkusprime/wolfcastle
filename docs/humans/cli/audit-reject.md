# wolfcastle audit reject

Rejects a pending audit finding. No project is created. The finding disappears.

## What It Does

Loads `audit-state.json`, marks the targeted finding (or all pending findings with `--all`) as `rejected` with a timestamp. No nodes are created, no state changes beyond the batch file.

When all findings in a batch have been decided, the batch is archived and the pending file is removed.

## Usage

```
wolfcastle audit reject <finding-id>
wolfcastle audit reject --all
```

## Flags

| Flag | Description |
|------|-------------|
| `--all` | Reject all pending findings at once. |
| `--json` | Output as structured JSON. |

## Arguments

| Argument | Description |
|----------|-------------|
| `finding-id` | **(Required unless `--all`)** ID of the finding to reject. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Finding(s) rejected. |
| 1 | Not initialized. |

## Consequences

- Mutates `audit-state.json`.
- Archives the batch when all findings are decided.
- No project nodes created.

## See Also

- [`wolfcastle audit approve`](audit-approve.md) to accept findings instead.
- [`wolfcastle audit pending`](audit-pending.md) to review what's waiting.
