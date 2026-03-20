# wolfcastle audit aar

Records a structured After Action Review for a completed task. The record of what was tried, what worked, and what to do differently.

## What It Does

Loads the node's `state.json` and stores an AAR keyed by task ID in the node's `aars` map. If an AAR already exists for that task, it is overwritten. AARs capture the objective, what actually happened, what went well, what could improve, and any follow-up action items.

AARs flow into subsequent tasks as context and feed into [audit execution](../audits.md#after-action-reviews). They are richer than breadcrumbs: structured narrative rather than timestamped notes.

## Usage

```
wolfcastle audit aar --node <path> --task <id> \
  --objective "..." --what-happened "..." \
  [--went-well "..."] [--improvements "..."] [--action-items "..."]
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Target node address. |
| `--task <id>` | **(Required)** Task ID for the review. |
| `--objective <text>` | **(Required)** What the task set out to do. |
| `--what-happened <text>` | **(Required)** What actually happened. |
| `--went-well <text>` | Things that went well. Repeatable. |
| `--improvements <text>` | Things that could be improved. Repeatable. |
| `--action-items <text>` | Follow-up items for next tasks. Repeatable. |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | AAR recorded. |
| 1 | Not initialized, identity not configured, or missing required flag. |

## Consequences

- Mutates the node's `state.json` AARs map.
- Overwrites any previous AAR for the same task ID.

## See Also

- [`wolfcastle audit breadcrumb`](audit-breadcrumb.md) for timestamped execution notes.
- [`wolfcastle audit report`](audit-report.md) to view the audit report that consumes AARs.
- [After Action Reviews](../audits.md#after-action-reviews) for how AARs fit into the audit lifecycle.
