# wolfcastle spec list

Lists specs. Optionally filtered to those linked to a specific node. Read-only.

## What It Does

Scans `.wolfcastle/docs/specs/` for all spec files. If `--node` is provided, filters to specs referenced in that node's `state.json`. Displays filenames, timestamps, and titles.

## Usage

```
wolfcastle spec list
wolfcastle spec list --node backend/auth
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | Filter to specs linked to this node. |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success. |
| 1 | Not initialized. |

## Consequences

None. Read-only.

## See Also

- [`wolfcastle spec create`](spec-create.md) to create specs.
- [`wolfcastle spec link`](spec-link.md) to link specs to nodes.
