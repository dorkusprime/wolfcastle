# wolfcastle audit enrich

Adds enrichment context to a node's audit record. Extra intelligence for the auditor to consider when evaluating the node.

## What It Does

Loads the node's `state.json` and appends the provided text to its `audit_enrichment` list. Duplicates are silently ignored. The enrichment text surfaces during [audit execution](../audits.md#audit-execution), giving the auditor additional angles of attack when verifying the work.

Think of enrichment as pre-audit instructions. "Check error handling in the auth module." "Verify backward compatibility with v2 clients." The auditor sees these and adjusts its scrutiny accordingly.

## Usage

```
wolfcastle audit enrich --node <path> "<text>"
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Target node address. |
| `--json` | Output as structured JSON. |

## Arguments

| Argument | Description |
|----------|-------------|
| `text` | **(Required)** The enrichment context to add. |

## Examples

```
wolfcastle audit enrich --node my-project "check error handling in auth module"
wolfcastle audit enrich --node my-project "verify backward compatibility"
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Enrichment added. |
| 1 | Not initialized. |
| 2 | Node not found. |
| 3 | Empty text. |

## Consequences

- Mutates the node's `state.json` enrichment list.
- Enrichment entries accumulate. They are permanent and feed into [audit execution](../audits.md#audit-execution).
- Duplicate entries are ignored. No harm in calling this twice with the same text.

## See Also

- [`wolfcastle audit breadcrumb`](audit-breadcrumb.md) for recording what was done.
- [`wolfcastle audit summary`](audit-summary.md) for the final result summary.
