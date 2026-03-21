# wolfcastle archive restore

Restores an archived node back to active state.

## What It Does

Moves a previously archived node and its entire subtree from `.wolfcastle/system/.archive/` back to the active state directory. Updates the root index: removes the node from `archived_root`, adds it back to `root`, and clears the `archived` and `archived_at` fields on every node in the subtree.

The node must be a root-level archived entry (present in `archived_root`).

## Usage

```
wolfcastle archive restore --node my-project
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Address of the archived node to restore. |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Node restored to active state. |
| 1 | Not initialized, identity not configured, node not found, or node is not archived. |

## Consequences

- Moves state directories from archive storage back to active locations.
- Mutates the root index to reflect the restored node.
- The restored node retains its previous state (complete, in_progress, etc.).

## See Also

- [`wolfcastle archive add`](archive-add.md) to create an archive entry.
- [`wolfcastle archive delete`](archive-delete.md) to permanently remove an archived node.
- [Archive](../collaboration.md#archive) for how archiving fits into the project lifecycle.
