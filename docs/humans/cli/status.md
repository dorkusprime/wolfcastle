# wolfcastle status

Shows the current state of your work. A tree-shaped battlefield report. Read-only.

## What It Does

Loads the root [state index](../how-it-works.md#distributed-state) and walks the tree, rendering every node with its current state. Each node gets a glyph and (in terminals that support it) a color:

| Glyph | Color | Meaning |
|-------|-------|---------|
| ● | Green | Complete. All tasks finished. |
| ◐ | Yellow | In progress. At least one task is being worked. |
| ◯ | (none) | Not started. Nothing claimed yet. |
| ☢ | Red | Blocked. Something needs human attention. |

Node addresses are shown in parentheses after the node name. Each task within a node is listed with its state and description. Blocked tasks include their block reason. Tasks that have failed show the failure count. Open [audit gaps](../audits.md) are printed inline beneath the node they belong to.

At the bottom, an inbox summary shows the count of new and filed items.

Without `--watch`, prints once and exits. With `--watch`, holds the screen and refreshes at the configured interval.

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | Show status for a specific subtree only. |
| `--all` | Aggregate status across all engineer namespaces. |
| `--watch`, `-w` | Continuously refresh the tree view. |
| `--interval <seconds>` | Refresh interval for `--watch`, in seconds. Accepts float64. Default: `5`. |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success. |
| 1 | Not initialized or identity not configured. |

## Consequences

None. This command is strictly read-only.

## See Also

- [`wolfcastle log`](log.md) to read daemon output (`follow` still works as an alias).
- [The Project Tree](../how-it-works.md#the-project-tree) for how the tree is structured.
