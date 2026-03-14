# wolfcastle audit pending

Shows audit findings waiting for your decision. Read-only.

## What It Does

Loads `audit-state.json` and filters to findings with status `pending`. Displays each finding's ID, title, and description preview.

If no review batch exists, reports that there are no pending findings.

## Usage

```
wolfcastle audit pending
```

## Flags

| Flag | Description |
|------|-------------|
| `--json` | Output as structured JSON (includes full batch metadata). |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success. |

## Consequences

None. Read-only.

## See Also

- [`wolfcastle audit`](audit-run.md) to generate findings.
- [`wolfcastle audit approve`](audit-approve.md) and [`wolfcastle audit reject`](audit-reject.md) to act on them.
