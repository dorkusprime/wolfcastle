# wolfcastle audit list

Lists available audit scopes. Equivalent to `wolfcastle audit run --list`.

## What It Does

Discovers scopes from `base/audits/`, `custom/audits/`, and `local/audits/` (all [three tiers](../configuration.md#three-tier-directory-structure)). Prints each scope's ID and description.

## Usage

```
wolfcastle audit list
wolfcastle audit list --json
```

## Flags

| Flag | Description |
|------|-------------|
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Scopes listed. |
| 1 | Not initialized. |

## See Also

- [`wolfcastle audit run`](audit-run.md) to run audits against scopes.
- [Scopes](../audits.md#scopes) for how to add custom audit scopes.
