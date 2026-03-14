# wolfcastle audit show

Displays the complete audit record for a node: scope, breadcrumbs, gaps, escalations, status, and result summary. One command, full picture.

## What It Does

Loads the node's `state.json` and renders every field in its `audit` object. In human mode, formats each section with counts and timestamps. In JSON mode, returns the raw audit structure.

## Usage

```
wolfcastle audit show --node <path>
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Target node. |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Audit state displayed. |
| 1 | Not initialized, or node not found. |

## Consequences

- Read-only. No state is modified.

## See Also

- [`wolfcastle audit scope`](audit-scope.md) to set the audit scope.
- [`wolfcastle audit breadcrumb`](audit-breadcrumb.md) to add trail entries.
- [`wolfcastle audit gap`](audit-gap.md) to record gaps.
- [The Audit System](../audits.md#the-audit-system) for the full audit lifecycle.
