# wolfcastle project create

Creates a new project node in the tree. Orchestrators organize work. This is how you build structure.

## What It Does

Generates a URL-safe slug from the project name. Creates the project directory, writes a `state.json` (type: orchestrator, empty children list), and creates a project description Markdown file. Registers the new node in the parent's children list and updates the root index.

If no `--node` is specified, the project is created at the root level. If a sibling with the same slug already exists, appends a numeric suffix.

## Usage

```
wolfcastle project create --node <parent> "<name>"
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <parent>` | Parent node path. Omit for a root-level project. Must be an [orchestrator](../how-it-works.md#the-project-tree), not a leaf. |
| `--json` | Output as structured JSON. |

## Arguments

| Argument | Description |
|----------|-------------|
| `name` | **(Required)** Human-readable project name. Gets slugified for the directory name. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Project created. |
| 1 | Not initialized. |
| 2 | Parent node not found. |
| 3 | Parent is a leaf (cannot add children to leaves). |
| 4 | Empty name. |

## Consequences

- Creates a new directory under `projects/{identity}/`.
- Writes `state.json` and `{slug}.md` in the new directory.
- Mutates the parent's `state.json` to include the new child.
- Updates the root state index.

## See Also

- [`wolfcastle task add`](task-add.md) to add tasks to a leaf node.
- [`wolfcastle inbox add`](inbox-add.md) to let the daemon handle organization.
- [The Project Tree](../how-it-works.md#the-project-tree) for how nodes relate.
