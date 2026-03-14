# wolfcastle audit

Runs a read-only codebase audit against composable scopes. Produces a Markdown report. Does not touch your code.

## What It Does

Discovers available [scopes](../audits.md#scopes) from `base/audits/`, `custom/audits/`, and `local/audits/` (all [three tiers](../configuration.md#three-tiers)). For each requested scope, invokes a model to analyze your codebase and collect findings. Saves the findings as a pending batch in `audit-review.json`.

The audit is strictly read-only. The model reads your code and produces a report. It does not modify files, create branches, or write code.

Findings do not become tasks automatically. They go through an [approval gate](../audits.md#the-approval-gate): use [`wolfcastle audit approve`](audit-approve.md) or [`wolfcastle audit reject`](audit-reject.md) to decide their fate.

## Usage

```
wolfcastle audit                              # all scopes
wolfcastle audit --scope dry,modularity       # specific scopes
wolfcastle audit --list                       # show available scopes
```

## Flags

| Flag | Description |
|------|-------------|
| `--scope <scopes>` | Comma-separated scope IDs. Defaults to all discovered scopes. |
| `--list` | List available scopes and exit. |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Audit complete. |
| 1 | Not initialized. |

## Consequences

- Creates or updates `audit-review.json` with pending findings.
- Model invocation costs apply.
- No code modifications.

## See Also

- [`wolfcastle audit approve`](audit-approve.md) and [`wolfcastle audit reject`](audit-reject.md) to act on findings.
- [`wolfcastle audit pending`](audit-pending.md) to review what's waiting.
- [Scopes](../audits.md#scopes) for how to add custom audit scopes.
