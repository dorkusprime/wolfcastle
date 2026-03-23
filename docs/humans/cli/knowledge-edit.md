# wolfcastle knowledge edit

Opens the codebase knowledge file in your editor.

## What It Does

Opens `.wolfcastle/docs/knowledge/{namespace}.md` in `$EDITOR` (falls back to `vi` if unset). Creates the file with an empty template if it doesn't exist yet.

With `--json`, reports the file path and editor that would be used without opening anything. This is useful for programmatic callers that need the path.

## Usage

```
wolfcastle knowledge edit
EDITOR=nano wolfcastle knowledge edit
```

## Flags

| Flag | Description |
|------|-------------|
| `--json` | Output path and editor as structured JSON without opening the editor. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Editor exited cleanly. |
| 1 | Not initialized, identity not configured, or editor error. |

## Consequences

- Creates the knowledge file if it doesn't exist.
- Any edits you make are saved when the editor exits.

## See Also

- [`wolfcastle knowledge show`](knowledge-show.md) to read without editing.
- [`wolfcastle knowledge prune`](knowledge-prune.md) for guided pruning with token count reporting.
