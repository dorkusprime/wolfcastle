# wolfcastle archive delete

Permanently deletes an archived node and its subtree. Irreversible.

## What It Does

Removes an archived node's state directory from `.wolfcastle/system/.archive/` and purges the node (and all descendants) from the root index. The node must be a root-level archived entry (present in `archived_root`). The `--confirm` flag is required as a safety gate; the command refuses to run without it.

## Usage

```
wolfcastle archive delete --node my-project --confirm
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Address of the archived node to delete. |
| `--confirm` | **(Required)** Confirm permanent deletion. Without this flag, the command errors. |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Archived node permanently deleted. |
| 1 | Not initialized, identity not configured, `--confirm` missing, node not found, or node is not archived. |

## Consequences

- Permanently removes archived state files from disk. This cannot be undone.
- Removes the node and all descendants from the root index.
- Does not affect archive Markdown entries in `.wolfcastle/archive/` (those are separate from state).

## See Also

- [`wolfcastle archive add`](archive-add.md) to create an archive entry.
- [`wolfcastle archive restore`](archive-restore.md) to restore an archived node to active state.
- [Archive](../collaboration.md#archive) for how archiving fits into the project lifecycle.
