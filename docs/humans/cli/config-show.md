# wolfcastle config show

Displays the resolved Wolfcastle configuration as indented JSON.

## What It Does

Merges hardcoded defaults with three tier files (base < custom < local) and prints the result. An optional positional argument filters output to a single top-level section.

By default, the output includes the hardcoded defaults layer. Pass `--raw` to suppress defaults and see only what the tier files contribute. Pass `--tier` to isolate a single tier's overlay rather than the merged result.

## Usage

```
wolfcastle config show
wolfcastle config show daemon
wolfcastle config show --tier local
wolfcastle config show --raw
```

## Flags

| Flag | Description |
|------|-------------|
| `--tier <name>` | Display a single tier instead of the merged result. One of: `base`, `custom`, `local`. |
| `--raw` | Suppress the hardcoded defaults layer. Shows only values contributed by tier files. |
| `--json` | Output as structured JSON envelope. |

## Arguments

| Argument | Description |
|----------|-------------|
| `[section]` | Optional. A top-level key to filter output (e.g., `daemon`, `audit`, `pipeline`). |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Configuration displayed. |
| 1 | Not initialized or section not found. |

## Consequences

None. This command is strictly read-only.

## See Also

- [`wolfcastle config set`](config-set.md) to write a value into a tier overlay.
- [Configuration](../configuration.md) for how tiers merge and what keys are available.
- [Config Reference](../config-reference.md) for every field, its type, and default value.
