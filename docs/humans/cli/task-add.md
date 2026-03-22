# wolfcastle task add

Places a task exactly where you want it. No model involved. No decomposition. You know what needs doing and where it belongs.

## What It Does

Loads the target leaf node's `state.json`, generates the next task ID (`task-N`), and inserts the new task into the task list before the [audit task](../audits.md#the-audit-system). The new task starts as `not_started` with a failure count of zero.

Only works on [leaf nodes](../how-it-works.md#the-project-tree). If you point it at an orchestrator, it refuses. Tasks belong in leaves.

## Usage

```
wolfcastle task add --node <path> "<description>"
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Tree address of the target leaf node. |
| `--body <text>` | Detailed task description/body. |
| `--stdin` | Read task body from stdin. |
| `--deliverable <path>` | Expected output file. Repeatable. |
| `--type <type>` | Task type: `discovery`, `spec`, `adr`, `implementation`, `integration`, `cleanup`. |
| `--class <class>` | [Task class](../task-classes.md) override (e.g., `coding/go`). Must match a configured class. |
| `--constraint <text>` | Constraint: what not to do. Repeatable. |
| `--acceptance <text>` | Acceptance criterion. Repeatable. |
| `--reference <path>` | Reference material path. Repeatable. |
| `--integration <text>` | How this task connects to other work. |
| `--parent <task-id>` | Parent task ID for hierarchical decomposition (e.g., `task-0001`). |
| `--json` | Output as structured JSON. |

## Arguments

| Argument | Description |
|----------|-------------|
| `title` | **(Required)** What the task should accomplish. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Task added. |
| 1 | Not initialized. |
| 2 | Node not found. |
| 3 | Target node is not a leaf. |
| 4 | Empty description. |

## Consequences

- Mutates the leaf's `state.json` to include the new task.
- No [state propagation](../how-it-works.md#state-propagation) needed; a `not_started` task does not change the parent's computed state.

## See Also

- [`wolfcastle project create`](project-create.md) to create the node first.
- [`wolfcastle inbox add`](inbox-add.md) if you'd rather let the daemon figure out where the task belongs.
- [Getting Work In](../how-it-works.md#getting-work-in) for how work enters the system.
- [Task Classes](../task-classes.md) for available classes and how to create your own.
