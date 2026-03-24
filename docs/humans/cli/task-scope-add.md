# wolfcastle task scope add

Claims exclusive access to files for a task during parallel execution. The agent calls this after studying the codebase and before writing any code.

## What It Does

Reads the scope lock table from `scope-locks.json`, checks each requested file for conflicts with locks held by other tasks, and either grants all locks or rejects the entire request. No partial acquisition: either every file is locked or none are.

If the requesting task already holds a lock on a file, that file is silently accepted (idempotent re-acquisition).

A file argument ending with `/` is treated as a directory scope. Two entries conflict if either is a prefix of the other (bidirectional containment). Paths containing `..` or starting with `/` are rejected.

## Usage

```
wolfcastle task scope add --node <address> --task <task-id> <file> [<file>...]
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <address>` | **(Required)** Node address for the task. |
| `--task <task-id>` | **(Required when not running inside a daemon iteration)** Task ID. |
| `--json` | Output as structured JSON. |

## Arguments

| Argument | Description |
|----------|-------------|
| `file` | **(Required, one or more)** File paths or directory paths (trailing `/`) to lock. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All locks acquired. |
| 1 | Scope conflict detected. No locks acquired. |

## Consequences

- Writes to `scope-locks.json` in the namespace directory.
- Acquires the namespace file lock during mutation.
- On conflict, names the conflicting file and holding task in the error output.

## See Also

- [`wolfcastle task scope list`](task-scope-list.md) to inspect current locks.
- [`wolfcastle task scope release`](task-scope-release.md) to release locks.
- [ADR-095](../../decisions/095-parallel-sibling-execution.md) for the design rationale.
