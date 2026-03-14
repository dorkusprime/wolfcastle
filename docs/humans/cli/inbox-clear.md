# wolfcastle inbox clear

Removes processed items from the inbox. Keeps things tidy.

## What It Does

Loads `inbox.json` and removes items that have already been handled. Without `--all`, only removes items with status `filed` or `expanded` (items the daemon has already processed). With `--all`, clears everything including `new` items.

## Usage

```
wolfcastle inbox clear
wolfcastle inbox clear --all
```

## Flags

| Flag | Description |
|------|-------------|
| `--all` | Remove everything, including unprocessed `new` items. |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Inbox cleared. |
| 1 | Not initialized. |

## Consequences

- Mutates `inbox.json`. Removed items are gone.

## See Also

- [`wolfcastle inbox list`](inbox-list.md) to see what's there before clearing.
- [`wolfcastle inbox add`](inbox-add.md) to add new items.
