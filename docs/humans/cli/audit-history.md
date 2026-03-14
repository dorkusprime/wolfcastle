# wolfcastle audit history

Shows the history of completed audit review batches. Read-only.

## What It Does

Loads `audit-review-history.json` and displays entries in reverse chronological order: batch ID, completion time, scopes covered, and a summary of how many findings were approved vs. rejected.

## Usage

```
wolfcastle audit history
```

## Flags

| Flag | Description |
|------|-------------|
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success (even if no history exists). |

## Consequences

None. Read-only.

## See Also

- [`wolfcastle audit`](audit-run.md) to run a new audit.
- [`wolfcastle audit pending`](audit-pending.md) for the current batch.
