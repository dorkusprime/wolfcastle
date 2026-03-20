# wolfcastle audit summary

Records the final result summary on a node's audit record. The last word before the audit begins.

## What It Does

Loads the node's `state.json` and sets the `audit.result_summary` field to the provided text. This overwrites any previous summary. Call this before signaling `WOLFCASTLE_COMPLETE` on the final task so the auditor knows what was accomplished.

The summary is the executive briefing. Breadcrumbs tell the full story; the summary tells the ending.

## Usage

```
wolfcastle audit summary --node <path> "<text>"
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Target node address. |
| `--json` | Output as structured JSON. |

## Arguments

| Argument | Description |
|----------|-------------|
| `text` | **(Required)** The result summary. |

## Examples

```
wolfcastle audit summary --node my-project "Implemented JWT auth with full test coverage"
wolfcastle audit summary --node auth/login "Refactored login flow to use OAuth2"
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Summary recorded. |
| 1 | Not initialized. |
| 2 | Node not found. |
| 3 | Empty text. |

## Consequences

- Mutates `audit.result_summary` in the node's `state.json`.
- Overwrites any previous summary. Only the last call wins.
- The auditor reads this during [audit execution](../audits.md#audit-execution) to understand the claimed outcome.

## See Also

- [`wolfcastle audit breadcrumb`](audit-breadcrumb.md) for the detailed trail of what happened.
- [`wolfcastle audit enrich`](audit-enrich.md) for adding context the auditor should consider.
