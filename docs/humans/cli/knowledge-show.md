# wolfcastle knowledge show

Displays the current namespace's codebase knowledge file.

## What It Does

Reads `.wolfcastle/docs/knowledge/{namespace}.md` and prints its contents. If no knowledge has been recorded yet, prints a message saying so. With `--json`, includes the file path and current token count alongside the content.

## Usage

```
wolfcastle knowledge show
wolfcastle knowledge show --json
```

## Flags

| Flag | Description |
|------|-------------|
| `--json` | Output as structured JSON (includes `namespace`, `content`, `path`, `token_count`). |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | File read (or empty file reported). |
| 1 | Not initialized or identity not configured. |

## Consequences

None. Read-only.

## See Also

- [`wolfcastle knowledge add`](knowledge-add.md) to append new entries.
- [`wolfcastle knowledge edit`](knowledge-edit.md) for free-form editing.
