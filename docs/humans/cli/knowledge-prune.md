# wolfcastle knowledge prune

Review and consolidate the codebase knowledge file to bring it under budget.

## What It Does

Opens the knowledge file in `$EDITOR` for manual pruning: remove stale entries, merge related ones, tighten wording. After you save and close the editor, the command reports the new token count relative to the configured budget so you can see whether the file is under budget.

With `--json`, operates non-interactively: reports the current token count, budget, and whether the file is over budget, without opening an editor. The daemon's maintenance task uses this mode.

## Usage

```
wolfcastle knowledge prune
wolfcastle knowledge prune --json
```

## Flags

| Flag | Description |
|------|-------------|
| `--json` | Report token count and budget status as structured JSON without opening the editor. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Prune completed (or status reported in JSON mode). |
| 1 | Not initialized, identity not configured, or editor error. |

## Consequences

- Opens the knowledge file for editing (interactive mode).
- Reports token count after editing, e.g. `Token count: 1450/2000` or `Token count: 2100/2000 (still over budget)`.
- Git history preserves anything you remove, so pruning is safe.

## Typical Workflow

When `wolfcastle knowledge add` rejects an entry with "Knowledge file exceeds budget," run `prune` to make room:

```
$ wolfcastle knowledge add "new discovery about the build system"
Error: Knowledge file exceeds budget (2050/2000 tokens). Run `wolfcastle knowledge prune` to review and consolidate.
$ wolfcastle knowledge prune
# ... editor opens, you consolidate entries ...
Token count: 1680/2000
$ wolfcastle knowledge add "new discovery about the build system"
Knowledge: new discovery about the build system
```

## See Also

- [`wolfcastle knowledge add`](knowledge-add.md) to append entries.
- [`wolfcastle knowledge show`](knowledge-show.md) to read the current file.
- [Configuration](../configuration.md#codebase-knowledge) for the `knowledge.max_tokens` setting.
