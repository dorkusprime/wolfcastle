# wolfcastle knowledge add

Appends a knowledge entry to the current namespace's codebase knowledge file.

## What It Does

Adds a line to `.wolfcastle/docs/knowledge/{namespace}.md`. Creates the file if it doesn't exist. The entry is formatted as a markdown bullet (a `- ` prefix is added if missing) and appended to the end of the file.

Before writing, the command checks the entry against the configured token budget (`knowledge.max_tokens`, default 2000). If the new entry would push the file over budget, the command refuses to write and prints an error directing you to [`wolfcastle knowledge prune`](knowledge-prune.md). Nothing is silently truncated or dropped.

## Usage

```
wolfcastle knowledge add "the integration tests require docker compose up before running"
wolfcastle knowledge add "state.Store serializes mutations through a file lock; never hold it while calling pipeline.AssemblePrompt"
```

## Arguments

| Argument | Description |
|----------|-------------|
| `entry` | **(Required)** The knowledge to record. Should be concrete, durable, and non-obvious. |

## Flags

| Flag | Description |
|------|-------------|
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Entry appended. |
| 1 | Not initialized or identity not configured. |
| 1 | Empty entry. |
| 1 | File would exceed token budget. |

## Consequences

- Appends to the namespace's knowledge file on disk.
- The new entry is visible to the next daemon iteration (knowledge files are read fresh each time, not cached).

## See Also

- [`wolfcastle knowledge show`](knowledge-show.md) to read the file.
- [`wolfcastle knowledge prune`](knowledge-prune.md) when the file exceeds its token budget.
- [Codebase Knowledge Files](../how-it-works.md#codebase-knowledge-files) for how knowledge fits into the execution context.
- [Configuration](../configuration.md#codebase-knowledge) for the `knowledge.max_tokens` setting.
