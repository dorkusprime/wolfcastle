# wolfcastle doctor

Validates the structural integrity of your project tree and repairs what it finds.

## What It Does

Loads the root index and runs a comprehensive set of checks against the [distributed state](../how-it-works.md#distributed-state) files:

- Root index consistency (all nodes registered, no phantom entries).
- Per-node state integrity (required fields present, valid values).
- Parent-child consistency (directory contents match children lists).
- Orphaned files (directories not referenced by any parent or index).
- Stale in-progress tasks (tasks claimed but no daemon running).
- Missing [audit tasks](../audits.md#the-audit-system) on leaves.
- Missing description files.

Reports findings with severity (error, warning, info), location, and fix type. Without `--fix`, it reports only. With `--fix`, it applies deterministic repairs and attempts model-assisted fixes for ambiguous cases.

Fixes come in two categories. **Deterministic fixes** (9 of 17 issue types) are applied directly by Go code: missing index entries, stale entries, missing audit tasks, reset orphaned in-progress tasks, create missing state files. **Model-assisted fixes** (5 types) handle ambiguous cases like conflicting state or unclear resolution. These invoke a [model you configure](../configuration.md#models) to reason about the fix, with Go validating the result before applying it.

A subset of these checks also runs automatically on [daemon startup](start.md). If the tree is corrupted, the daemon refuses to start.

## Usage

```
wolfcastle doctor
wolfcastle doctor --fix
```

## Flags

| Flag | Description |
|------|-------------|
| `--fix` | Apply deterministic and model-assisted fixes. Without this flag, doctor only reports issues. |
| `--json` | Output as structured JSON. |

## Configuration

```json
{
  "doctor": {
    "model": "mid",
    "prompt_file": "doctor.md"
  }
}
```

The model is only invoked for ambiguous fixes. Deterministic fixes never call a model.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | No issues found, or all issues fixed. |
| 1 | Not initialized. |
| 2 | User aborted. |
| 3 | Some fixes failed. |

## Consequences

- **Without `--fix`**: reports issues only, changes nothing.
- **Deterministic fixes**: mutates state files directly (missing entries, stale states, orphaned files).
- **Model-assisted fixes**: invokes a model, then validates and applies the proposed fix.

## See Also

- [Structural Validation](../audits.md#structural-validation) for the full list of issue types.
- [`wolfcastle start`](start.md) for the startup validation subset.
