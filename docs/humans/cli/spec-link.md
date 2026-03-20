# wolfcastle spec link

Links an existing spec file to a node. The spec becomes part of the model's context when working on that node.

## What It Does

Loads the target node's `state.json` and appends the spec filename to its `specs` array. Multiple nodes can reference the same spec for cross-cutting concerns.

## Usage

```
wolfcastle spec link --node backend/auth/oauth oauth-spec.md
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Target node. |
| `--json` | Output as structured JSON. |

## Arguments

| Argument | Description |
|----------|-------------|
| `filename` | **(Required)** Spec filename to link. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Spec linked. |
| 1 | Not initialized. |
| 2 | Node not found. |

## Consequences

- Mutates the node's `state.json` to include the spec reference.
- The spec will be injected into the model's context during [task execution](../how-it-works.md#execution-protocol) on this node.

## See Also

- [`wolfcastle spec create`](spec-create.md) to create a new spec.
- [`wolfcastle spec list`](spec-list.md) to see linked specs.
- [Specs](../collaboration.md#specs) for how specs work.
