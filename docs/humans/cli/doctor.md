# wolfcastle doctor

Validates the structural integrity of your project tree and repairs what it finds.

## What It Does

Loads the root index and runs 17 structural checks against the [distributed state](../how-it-works.md#distributed-state) files:

- Root index consistency (all nodes registered, no phantom entries).
- Per-node state integrity (required fields present, valid values).
- Parent-child consistency (directory contents match children lists).
- Orphaned files (directories not referenced by any parent or index).
- Stale in-progress tasks (tasks claimed but no daemon running).
- Missing [audit tasks](../audits.md#the-audit-system) on leaves.
- Missing description files.

Reports findings with severity (error, warning, info), location, and fix type. Without `--fix`, it reports only. With `--fix`, it runs multi-pass repair: each pass validates, fixes, and re-validates until no fixable issues remain or 5 passes complete. Cascading fixes (e.g. resetting stale tasks changes propagation, which changes audit status) are handled automatically across passes. After deterministic fixes, it attempts model-assisted repair if `doctor.model` is configured.

Fixes come in two categories. **Deterministic fixes** (11 of 17 issue types) are applied directly by Go code: missing index entries, stale entries, missing audit tasks, reset orphaned in-progress tasks, create missing state files, STALE_IN_PROGRESS (reset to not_started), and MULTIPLE_IN_PROGRESS (reset to not_started). **Model-assisted fixes** (3 types) handle ambiguous cases like conflicting state or unclear resolution. These invoke a [model you configure](../configuration.md#models) to reason about the fix, with Go validating the result before applying it.

INVALID_AUDIT_SCOPE only fires when the scope has criteria or files but no description after audit completed. Default empty scopes are normal and do not trigger this check.

Issues that survive all fix passes get escalation guidance: the exact `wolfcastle task update` and `wolfcastle unblock` commands you need to resolve them manually.

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
