# wolfcastle audit report

Displays the latest audit report for a node. If no report file exists yet, generates a preview from current state.

## What It Does

Checks the node's directory for saved audit report files (Markdown). If one exists, prints it. If not, loads the node's `state.json`, builds a report preview from the current audit state (scope, breadcrumbs, gaps, status), and prints that instead.

With `--path`, prints only the file path to the report (or nothing if no report exists on disk). Useful for piping into other tools.

## Usage

```
wolfcastle audit report --node <path>
wolfcastle audit report --node <path> --path
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Target node address. |
| `--path` | Print only the report file path, not its contents. |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Report displayed (or path printed). |
| 1 | Not initialized, identity not configured, or node not found. |

## Consequences

- Read-only. No state is modified.

## See Also

- [`wolfcastle audit show`](audit-show.md) for raw audit state (scope, breadcrumbs, gaps).
- [`wolfcastle audit aar`](audit-aar.md) to record After Action Reviews that feed into reports.
- [Audit Reports](../audits.md#audit-reports) for what reports contain and when they are generated.
