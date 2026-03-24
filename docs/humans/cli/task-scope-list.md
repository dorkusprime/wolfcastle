# wolfcastle task scope list

Lists file scope locks currently held by running tasks.

## What It Does

Reads the scope lock table and displays all locks, optionally filtered by node or task address. Useful for debugging parallel execution conflicts and inspecting which files are claimed.

## Usage

```
wolfcastle task scope list [--node <address>] [--task <task-address>]
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <address>` | Filter results to locks held by tasks under this node. |
| `--task <task-address>` | Filter results to locks held by this specific task (full task address). |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success (even if no locks found). |

## Consequences

- Read-only. Does not modify state.

## See Also

- [`wolfcastle task scope add`](task-scope-add.md) to acquire locks.
- [`wolfcastle task scope release`](task-scope-release.md) to release locks.
- [`wolfcastle status`](status.md) for a broader view of daemon activity.
