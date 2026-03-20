# wolfcastle orchestrator criteria

Manages success criteria on an orchestrator node. Add them one at a time, or list what exists.

## What It Does

In add mode, loads the node's `state.json` and appends a success criterion to its `success_criteria` list. Duplicates are silently ignored.

In list mode (`--list`), reads the node and prints all defined criteria.

Success criteria tell the orchestrator what "done" looks like. "All tests pass." "Coverage above 90%." "No lint warnings." The orchestrator evaluates these when deciding whether children have finished the job.

## Usage

```
wolfcastle orchestrator criteria --node <path> "<criterion>"
wolfcastle orchestrator criteria --node <path> --list
```

## Flags

| Flag | Description |
|------|-------------|
| `--node <path>` | **(Required)** Target node address. |
| `--list` | List existing criteria instead of adding one. |
| `--json` | Output as structured JSON. |

## Arguments

| Argument | Description |
|----------|-------------|
| `criterion` | **(Required unless `--list`)** The success criterion to add. |

## Examples

```
wolfcastle orchestrator criteria --node my-project "all tests pass"
wolfcastle orchestrator criteria --node my-project --list
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Criterion added, or list displayed. |
| 1 | Not initialized. |
| 2 | Node not found. |
| 3 | Empty criterion text (add mode) or missing argument without `--list`. |

## Consequences

- In add mode, mutates the node's `state.json` success criteria list.
- Criteria accumulate. Duplicates are ignored.
- These criteria feed into orchestrator-level evaluation when determining project completion.

## See Also

- [`wolfcastle audit scope`](audit-scope.md) for defining audit verification scope.
- [`wolfcastle audit enrich`](audit-enrich.md) for adding context to the audit itself.
