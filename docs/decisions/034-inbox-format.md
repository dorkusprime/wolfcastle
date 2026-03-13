# ADR-034: Inbox Format and Lifecycle

## Status
Accepted

## Date
2026-03-13

## Context
Wolfcastle's inbox accepts rough ideas that flow through the expand→file pipeline stages (ADR-006) before becoming projects in the work tree. The inbox format needs to support status tracking through this lifecycle and be manageable by both the CLI and the daemon.

## Decision

### JSON Format
The inbox is a single JSON file at `{projects}/{identity}/inbox.json`:

```json
{
  "items": [
    {
      "timestamp": "2026-03-13T19:36:49Z",
      "text": "Add rate limiting to the API",
      "status": "new",
      "expanded": ""
    }
  ]
}
```

### Fields

| Field | Type | Description |
|-------|------|-------------|
| `timestamp` | string (RFC 3339) | When the item was added |
| `text` | string | The original idea/description |
| `status` | string | Lifecycle state (see below) |
| `expanded` | string | Structured expansion from the expand stage (populated by model) |

### Status Lifecycle

```
new → expanded → filed
```

| Status | Meaning |
|--------|---------|
| `new` | Just added, not yet processed by any pipeline stage |
| `expanded` | The expand stage has fleshed out the idea into a structured description (stored in `expanded` field) |
| `filed` | The file stage has created projects/tasks from this item. Terminal state. |

### Shared Package
Inbox types and I/O live in `internal/inbox/` so they're accessible from both `cmd/` (CLI commands) and `internal/daemon/` (pipeline stages).

### CLI Commands
- `wolfcastle inbox add "idea"` — creates a new item with status `new`
- `wolfcastle inbox list` — shows all items with status and timestamp
- `wolfcastle inbox clear` — removes `filed` and `expanded` items (keeps `new`); `--all` removes everything

## Consequences
- Simple, flat JSON file — no complex data structures
- Status lifecycle enables the daemon to process items incrementally
- `expanded` field preserves the model's structured output for the file stage
- CLI commands give users full visibility and control over the inbox
- The daemon's expand and file stages are the primary consumers of this format
