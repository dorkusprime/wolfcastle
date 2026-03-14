# wolfcastle archive add

Generates a Markdown archive entry from a completed node. The work's permanent record.

## What It Does

Validates that the target node is `complete` (all children complete, audit done). Gathers archive data from the node and its children: summary (if the [summary stage](../how-it-works.md#the-pipeline) ran), chronological [breadcrumbs](../audits.md#breadcrumbs), [audit results](../audits.md#the-audit-system) (scopes, gaps, escalations), and metadata (node path, completion timestamp, engineer identity, current branch).

Generates a timestamped filename (`{timestamp}-{slug}.md`) and writes the archive entry to `.wolfcastle/archive/`. The content is assembled deterministically from state data; no model is invoked at archive time.

## Usage

```
wolfcastle archive add --node backend/auth
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Path of the completed node to archive. |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Archive entry created. |
| 1 | Not initialized. |
| 2 | Node not found. |
| 3 | Node is not `complete`. |

## Consequences

- Creates a new Markdown file in `.wolfcastle/archive/`.
- Archive files are append-only. Unique filenames by construction. Merge-conflict-proof.

## See Also

- [Archive](../collaboration.md#archive) for how archive entries are used.
- [`wolfcastle status`](status.md) to check if a node is complete.
