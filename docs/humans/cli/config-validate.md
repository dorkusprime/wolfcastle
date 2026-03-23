# wolfcastle config validate

Checks the resolved Wolfcastle configuration for errors and warnings.

## What It Does

Loads and merges all configuration tiers, then runs validation checks against the result. By default, only structural checks run: field constraints, stage consistency, threshold bounds, and similar invariants that don't require a running project. Pass `--full` to include identity and cross-reference checks (model references from summary, doctor, unblock, and audit subsystems).

Warnings (such as unknown fields in tier files) are always reported, regardless of mode. Warnings do not affect the exit code.

## Usage

```
wolfcastle config validate
wolfcastle config validate --full
wolfcastle config validate --json
wolfcastle config validate --full --json
```

## Flags

| Flag | Description |
|------|-------------|
| `--full` | Run full validation including identity presence and cross-reference checks. Without this flag, only structural checks run. |
| `--json` | Output as a structured JSON envelope with an `issues` array and summary counts. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Configuration is valid (warnings may still be present). |
| 1 | Validation errors found, or configuration could not be loaded. |

## Output

In human mode, each issue prints on its own line as `[severity] category: description`, followed by a summary line showing error and warning counts.

In JSON mode, the envelope contains:

```json
{
  "action": "config_validate",
  "ok": true,
  "data": {
    "issues": [
      {"severity": "warning", "category": "unknown_field", "description": "..."},
      {"severity": "error", "category": "structure", "description": "..."}
    ],
    "error_count": 0,
    "warning_count": 1
  }
}
```

The `category` field is `unknown_field` for warnings, `structure` for structural errors, and `validation` for full-mode cross-reference errors.

## Structural vs Full Validation

**Structural checks** (`ValidateStructure`, the default) verify that the configuration is internally consistent without requiring project identity or external state:

- Pipeline has at least one stage
- Stage order contains no duplicates and references only defined stages
- Every stage appears in both the stages map and the stage order
- All stage model references resolve to entries in the models map
- Stage prompt files are non-empty
- Failure thresholds and caps are within valid ranges
- Daemon timing values are positive and above minimums
- Log retention, retry, and validation command constraints are met
- Git commit message format is non-empty
- Overlap advisory threshold is between 0 and 1
- Knowledge token budget is at least 1
- Every model definition has a non-empty command

**Full checks** (`--full`) run all structural checks, then additionally verify:

- Summary, doctor, unblock, and audit subsystems reference valid models
- Summary, doctor, unblock, and audit prompt files are non-empty
- Project identity is configured

## Examples

Run a quick structural check before committing a config change:

```
wolfcastle config validate
```

Verify everything including identity (useful after `wolfcastle init`):

```
wolfcastle config validate --full
```

Use in a CI pipeline or pre-commit hook, failing the step on any error:

```
wolfcastle config validate --json || exit 1
```

Parse the JSON output programmatically to count issues:

```
wolfcastle config validate --json | jq '.data.error_count'
```

## Consequences

None. This command is read-only and makes no changes to configuration files.

## See Also

- [`wolfcastle config show`](config-show.md) to display the resolved configuration.
- [`wolfcastle config set`](config-set.md) to write a value into a tier overlay.
- [Config Reference](../config-reference.md) for every field, its type, and default value.
