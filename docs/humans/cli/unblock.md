# wolfcastle unblock

Model-assisted or agent-assisted unblocking for stuck tasks. This is [Tier 2 and Tier 3](../failure-and-recovery.md#the-unblock-workflow) of the unblock workflow.

For the simple status flip (Tier 1), use [`wolfcastle task unblock`](task-unblock.md) instead.

## What It Does

### Interactive Mode (Tier 2)

Without `--agent`, starts a multi-turn conversation with a model. The session is pre-loaded with everything relevant: block reason, failure count, decomposition depth, failure history, [breadcrumbs](../audits.md#breadcrumbs), [audit state](../audits.md#the-audit-system), and the surrounding code. You and the model work through the fix together.

This is not autonomous. You drive. When you've resolved the issue, run [`wolfcastle task unblock`](task-unblock.md) to reset the task.

### Agent Mode (Tier 3)

With `--agent`, outputs a rich structured Markdown diagnostic to stdout. No model invocation, no interactive session. The output includes block context, failure history, breadcrumbs, audit state, file paths, suggested approaches, and instructions. Feed it to whatever coding agent you're running.

## Usage

```
wolfcastle unblock --node <path>
wolfcastle unblock --agent --node <path>
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Tree address of the blocked task. |
| `--agent` | Output diagnostic context for an external agent instead of starting an interactive session. |
| `--json` | Output as structured JSON (agent mode only). |

## Configuration

Interactive mode uses a configurable model and prompt:

```json
{
  "unblock": {
    "model": "heavy",
    "prompt_file": "unblock.md"
  }
}
```

Agent mode does not invoke a model.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Session completed (interactive) or context dumped (agent). |
| 1 | Not initialized. |
| 2 | Node not found. |
| 3 | Task is not blocked. |

## Consequences

- **Interactive mode**: may create a conversation transcript. Does not change task state (you do that with [`task unblock`](task-unblock.md) when ready).
- **Agent mode**: read-only diagnostic. No state changes.

## See Also

- [`wolfcastle task unblock`](task-unblock.md) for the zero-cost Tier 1 status flip.
- [The Unblock Workflow](../failure-and-recovery.md#the-unblock-workflow) for all three tiers.
