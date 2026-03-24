# wolfcastle task scope release

Releases file scope locks held by a task. Normally called by the daemon after committing a completed task's changes, not by the agent directly.

## What It Does

Removes scope locks from the lock table. Without file arguments, releases all locks held by the specified task. With file arguments, releases only those specific files. Releasing a lock not held by the task is a silent no-op.

If the lock table becomes empty after release, the `scope-locks.json` file is deleted.

## Usage

```
wolfcastle task scope release --node <address> --task <task-id> [<file>...]
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <address>` | **(Required)** Node address for the task. |
| `--task <task-id>` | **(Required)** Task ID whose locks to release. |
| `--json` | Output as structured JSON. |

## Arguments

| Argument | Description |
|----------|-------------|
| `file` | (Optional) Specific files to release. If omitted, all locks for the task are released. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success. |

## Consequences

- Mutates `scope-locks.json` in the namespace directory.
- Acquires the namespace file lock during mutation.
- Deletes `scope-locks.json` entirely when the last lock is released.

## See Also

- [`wolfcastle task scope add`](task-scope-add.md) to acquire locks.
- [`wolfcastle task scope list`](task-scope-list.md) to inspect current locks.
