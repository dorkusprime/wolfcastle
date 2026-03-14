# wolfcastle inbox list

Shows everything in your inbox, grouped by status. Read-only.

## What It Does

Loads `inbox.json` from your [namespace](../collaboration.md#engineer-namespacing) and displays items grouped by status (`new`, `expanded`, `filed`), with timestamp and text for each.

## Usage

```
wolfcastle inbox list
```

## Flags

| Flag | Description |
|------|-------------|
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success. |
| 1 | Not initialized. |

## Consequences

None. Read-only.

## See Also

- [`wolfcastle inbox add`](inbox-add.md) to add items.
- [`wolfcastle inbox clear`](inbox-clear.md) to remove processed items.
