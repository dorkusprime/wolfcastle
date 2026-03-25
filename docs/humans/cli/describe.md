# wolfcastle describe

Shows everything about a single node: type, status, tasks, audit state, breadcrumbs, gaps, escalations, AARs, specs, and planning history.

## What It Does

Loads the node's `state.json` and its index entry, then renders a comprehensive view of the node's current state. Sections are shown conditionally: a leaf node won't display Planning, an orchestrator won't list Tasks, and empty collections (no specs, no AARs) are omitted entirely.

In `--json` mode, the full `NodeState` and `IndexEntry` are included in the envelope, along with the contents of any `description.md` file in the node directory.

## Usage

```
wolfcastle describe api/health
wolfcastle describe --node api/health
wolfcastle describe api/health --json
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <address>` | Node address (alternative to positional argument). |
| `--json` | Output as structured JSON envelope. |

The address can be provided as a positional argument or via `--node`, but not both.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success. |
| 1 | Node not found, not initialized, or identity not configured. |

## Human Output Sections

- **Header**: address, type (leaf/orchestrator/project), status, and scope description.
- **Tasks**: each task with glyph, ID, title, status, deliverables, class, references.
- **Children**: each child node with status glyph and address (orchestrators only).
- **Audit**: status, scope description, gap/escalation counts, result summary.
- **Breadcrumbs**: timestamped log of changes made during task execution.
- **Gaps**: detailed list of audit gaps with status.
- **Escalations**: detailed list with source node and status.
- **Specs**: linked specification filenames.
- **AARs**: after action review summaries by task ID.
- **Planning**: child count, replan count, planning history, success criteria (orchestrators only).

## JSON Envelope

```json
{
  "ok": true,
  "action": "describe",
  "data": {
    "node_state": { ... },
    "index_entry": {
      "name": "...",
      "type": "leaf",
      "state": "in_progress",
      "address": "api/health",
      "decomposition_depth": 0,
      "parent": "api",
      "children": [],
      "archived": false
    },
    "description_md": "# Health Check\n..."
  }
}
```

## Consequences

None. This command is strictly read-only.

## See Also

- [`wolfcastle status`](status.md) for a tree-wide overview.
- [`wolfcastle navigate`](navigate.md) to find the next actionable task.
- [`wolfcastle audit show`](audit-show.md) for audit-specific detail.
