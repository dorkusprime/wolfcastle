# wolfcastle audit breadcrumb

Appends a timestamped note to a node's audit trail. This is how tasks record what they did and why.

## What It Does

Loads the node's `state.json` and appends a breadcrumb to its `audit.breadcrumbs` array with the current UTC timestamp, the calling task's address, and the provided text.

Breadcrumbs are the raw material for [audit verification](../audits.md#audit-execution). They should be rich and explanatory, not terse commit messages. Describe what was done, why it was done, and what changed.

This is typically called by the model during the [record phase](../how-it-works.md#seven-phase-execution) of task execution.

## Usage

```
wolfcastle audit breadcrumb --node <path> "<text>"
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Target node. |
| `--json` | Output as structured JSON. |

## Arguments

| Argument | Description |
|----------|-------------|
| `text` | **(Required)** The breadcrumb content. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Breadcrumb recorded. |
| 1 | Not initialized. |
| 2 | Node not found. |
| 3 | Empty text. |

## Consequences

- Mutates the node's `state.json` audit trail.
- Breadcrumbs are permanent; they accumulate and feed into [audit execution](../audits.md#audit-execution) and the [archive](../collaboration.md#archive).

## See Also

- [`wolfcastle audit escalate`](audit-escalate.md) when a gap needs to go up the chain.
- [Breadcrumbs](../audits.md#breadcrumbs) for how they're used during audits.
