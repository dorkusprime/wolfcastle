# wolfcastle task amend

Modifies fields on a task that has not yet started. Once a task is in progress or complete, it is beyond amendment.

## What It Does

Loads the node's `state.json`, locates the task by its address, and applies only the fields you specify. Everything else stays untouched. Rejects the call if the task is `in_progress` or `complete`.

This is useful when planning reveals a task needs a different description, additional deliverables, or a type correction before execution begins.

## Usage

```
wolfcastle task amend <task-address> [flags]
wolfcastle task amend --node <task-address> [flags]
```

The task address can be given as a positional argument or via `--node`. Both forms are equivalent. Providing both is an error.

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | Task address: `node-path/task-id`. Alias for the positional argument. |
| `--body <text>` | Replace the task body/description. |
| `--type <type>` | Set task type: `discovery`, `spec`, `adr`, `implementation`, `integration`, `cleanup`. |
| `--integration <text>` | Describe how this task connects to other work. |
| `--add-deliverable <path>` | Append a deliverable. Repeatable. Duplicates ignored. |
| `--add-constraint <text>` | Append a constraint. Repeatable. Duplicates ignored. |
| `--add-acceptance <text>` | Append an acceptance criterion. Repeatable. Duplicates ignored. |
| `--add-reference <text>` | Append a reference. Repeatable. Duplicates ignored. |
| `--json` | Output as structured JSON. |

## Examples

```
wolfcastle task amend my-project/task-0001 --body "updated description"
wolfcastle task amend my-project/task-0001 --add-deliverable "docs/api.md"
wolfcastle task amend my-project/task-0001 --type implementation --integration "feeds into auth module"
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Task amended. |
| 1 | Not initialized. |
| 2 | Node not found. |
| 3 | Task not found in node. |
| 4 | Task is `in_progress` or `complete`. Cannot amend. |

## Consequences

- Mutates the task's fields in `state.json`. Only the flags you provide change anything.
- Append flags (`--add-deliverable`, etc.) accumulate. They never replace existing entries.
- Invalid task types are rejected before any mutation occurs.

## See Also

- [`wolfcastle task claim`](task-claim.md) to take ownership of a task.
- [`wolfcastle task add`](task-add.md) to create a new task.
