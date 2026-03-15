# wolfcastle task deliverable

Tells Wolfcastle what a task is supposed to produce. No deliverable, no proof. No proof, no victory.

## What It Does

Appends a file path to the task's deliverables list. The daemon checks that every declared deliverable exists before it accepts `WOLFCASTLE_COMPLETE`. If anything is missing, the task is marked as failed and the model gets another chance to finish the job.

Duplicate paths are silently ignored. Wolfcastle does not punish you for being thorough.

## Usage

```
wolfcastle task deliverable "<file-path>" --node <task-address>
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <task-address>` | **(Required)** Full task address: `node-path/task-id`. |
| `--json` | Output as structured JSON. |

## Arguments

| Argument | Description |
|----------|-------------|
| `file-path` | **(Required)** Path to the file the task must produce. |

## Examples

```
wolfcastle task deliverable "docs/pos-research.md" --node pizza-docs/task-0001
wolfcastle task deliverable "src/api/handler.go" --node my-project/task-0002
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Deliverable added (or already present). |
| 1 | Not initialized. |
| 2 | `--node` missing or malformed. |
| 3 | Task not found at the given address. |
| 4 | Empty deliverable path. |

## Consequences

- Mutates the target node's `state.json`, adding the path to the task's deliverables list.
- The daemon will verify this file exists at completion time. A missing deliverable means failure. Failure means re-invocation. The model does not get to claim victory without evidence.

## See Also

- [`wolfcastle task add`](task-add.md) to create the task in the first place.
- [`wolfcastle task complete`](task-complete.md) for what happens when all deliverables are accounted for.
- [`wolfcastle task block`](task-block.md) if the task cannot produce its deliverables yet.
