# wolfcastle audit scope

Sets structured audit scope on a node: what to verify, which files, which systems, which criteria. The audit task uses this scope to know what "correct" looks like.

## What It Does

Loads the node's `state.json` and updates its `audit.scope` object. Each flag sets its corresponding field. Fields not specified are left unchanged, so you can build the scope incrementally.

List values (files, systems, criteria) are pipe-delimited. Duplicates are removed.

## Usage

```
wolfcastle audit scope --node <path> --description "Verify auth module"
wolfcastle audit scope --node <path> --files "auth.go|login.go" --systems "auth|session"
wolfcastle audit scope --node <path> --criteria "no SQL injection|input validation"
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Target node. |
| `--description <text>` | Audit scope description. |
| `--files <list>` | Pipe-delimited list of files to audit. |
| `--systems <list>` | Pipe-delimited list of systems to audit. |
| `--criteria <list>` | Pipe-delimited list of acceptance criteria. |
| `--json` | Output as structured JSON. |

At least one of `--description`, `--files`, `--systems`, or `--criteria` is required.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Scope updated. |
| 1 | Not initialized, node not found, or no fields specified. |

## Consequences

- Mutates the node's `state.json` audit scope.
- The [audit task](../audits.md#the-audit-system) will use this scope to verify the node's work.

## See Also

- [`wolfcastle audit show`](audit-show.md) to see the full audit state.
- [Scopes](../audits.md#scopes) for how audit scopes fit into verification.
