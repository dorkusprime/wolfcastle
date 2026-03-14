# wolfcastle spec create

Creates a new specification document and optionally links it to a node.

## What It Does

Generates a timestamped filename, writes a template spec file to `.wolfcastle/docs/specs/`. If `--node` is provided, also appends the filename to that node's `specs` array in its `state.json`, making the spec part of the model's context when working on that node.

## Usage

```
wolfcastle spec create "Authentication Protocol"
wolfcastle spec create --node backend/auth "Authentication Protocol"
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | Link the new spec to this node immediately. |
| `--json` | Output as structured JSON. |

## Arguments

| Argument | Description |
|----------|-------------|
| `title` | **(Required)** Spec title. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Spec created. |
| 1 | Not initialized. |

## Consequences

- Creates a new Markdown file in `.wolfcastle/docs/specs/`.
- If `--node` is specified, mutates the node's `state.json` to reference the spec.

## See Also

- [`wolfcastle spec link`](spec-link.md) to link an existing spec to a node.
- [`wolfcastle spec list`](spec-list.md) to see what specs exist.
- [Specs](../collaboration.md#specs) for how specs travel with work.
