# Config Write Commands

Four subcommands for mutating Wolfcastle configuration through the CLI. Each writes to a single tier overlay file, validates the result against the full merged config, and rolls back on failure. All commands live under `wolfcastle config` alongside the existing `show` subcommand.

## Governing ADRs

- ADR-009: Three-tier config hierarchy (base < custom < local)
- ADR: WithConfig writes to custom tier (user-facing CLI defaults to local per scope)

## Synopsis

```
wolfcastle config set    <key> <value> [--tier local|custom] [--json]
wolfcastle config unset  <key>         [--tier local|custom] [--json]
wolfcastle config append <key> <value> [--tier local|custom] [--json]
wolfcastle config remove <key> <value> [--tier local|custom] [--json]
```

## Commands

### config set

Sets a configuration key to a value. If intermediate keys along the dot-notation path do not exist, they are created as empty maps. If the key already exists, its value is replaced.

### config unset

Removes a configuration key by writing `null` at that path in the tier overlay. On the next `Load`, `DeepMerge` sees the null and deletes the key from the merged result, effectively reverting it to whatever lower tiers or defaults provide. If no lower tier sets the key, it disappears from the resolved config entirely.

### config append

Appends a value to an existing array at the given key. The current value at the key must already be an array (or absent, in which case a new single-element array is created). The appended value is parsed with the same JSON-then-string rules as `set`.

### config remove

Removes a value from an array at the given key. The current value must be an array. Equality is checked by JSON-serializing both the candidate and each array element and comparing the strings. If the value is not found in the array, the command exits with an error. If removal would leave the array empty, the empty array is written (not null; use `unset` to delete the key).

## Arguments and Flags

| Argument / Flag | Type | Required | Default | Description |
|-----------------|------|----------|---------|-------------|
| `key` | positional | Yes | — | Dot-notation config path (e.g. `daemon.poll_interval_seconds`, `pipeline.stages.audit.enabled`) |
| `value` | positional | Yes (except `unset`) | — | The value to set, append, or remove. Parsed as JSON first; bare strings become JSON strings |
| `--tier` | string | No | `local` | Target tier: `local` or `custom`. Writing to `base` is not allowed |
| `--json` | boolean | No | `false` | Wrap output in the standard `{ok, action, data}` envelope |

## Dot-Notation Path Semantics

Paths split on `.` to address nested keys. Each segment names a map key, walking deeper into the config structure.

- `daemon.poll_interval_seconds` addresses `{"daemon": {"poll_interval_seconds": ...}}`
- `pipeline.stages.audit.enabled` addresses `{"pipeline": {"stages": {"audit": {"enabled": ...}}}}`
- Intermediate maps that do not exist are created automatically by `set`
- Array indexing (e.g. `commands[0]`) is not supported. Paths containing `[` or `]` are rejected with a parse error
- Empty segments (e.g. `daemon..poll`) and trailing dots (e.g. `daemon.`) are rejected as malformed
- A single-segment path (e.g. `version`) addresses a top-level key

## Value Parsing

The `value` argument is interpreted as follows:

1. Attempt `json.Unmarshal` into `any`. If it succeeds, use the result. This handles numbers (`5` becomes `float64(5)`), booleans (`true`/`false`), null, objects (`{"a":1}`), and arrays (`[1,2,3]`).
2. If JSON parsing fails, treat the raw string as a JSON string value. `alice` becomes `"alice"`, `some words` becomes `"some words"`.

This means users can write `wolfcastle config set logs.max_files 5` and get an integer, or `wolfcastle config set identity.user alice` and get a string, without quoting gymnastics.

## Tier Targeting

The `--tier` flag accepts two values:

- `local` (default): writes to `.wolfcastle/system/local/config.json`
- `custom`: writes to `.wolfcastle/system/custom/config.json`

Writing to `base` is rejected with: `error: cannot write to base tier (base is managed by the system)`.

The default of `local` follows the tier's purpose as the highest-priority, machine-specific overlay. The `custom` tier exists for project-level overrides that should be shared. The ADR "WithConfig writes to custom tier" governs programmatic writes from `Environment.WithConfig`; the CLI defaults to `local` because interactive users are typically adjusting their own machine's config.

## Read-Modify-Write Flow

All four commands share the same transactional flow:

1. **Read** the current tier overlay file using `readTierFile(wolfcastleRoot, tier)`. If the file does not exist, start with an empty map (`{}`).
2. **Mutate** the overlay map according to the command:
   - `set`: walk the dot-notation path, creating intermediate maps as needed, and assign the parsed value at the leaf.
   - `unset`: walk the dot-notation path and assign `nil` (JSON null) at the leaf. Intermediate maps are created if needed so the null lands at the correct depth. The null value flows through `DeepMerge` on the next `Load`, deleting the key from the merged result.
   - `append`: walk the path to the leaf. If the leaf is an array, append the parsed value. If the leaf does not exist, create a single-element array. If the leaf exists but is not an array, return an error.
   - `remove`: walk the path to the leaf. If the leaf is not an array, return an error. Search for the value by JSON-string equality and remove it. If not found, return an error.
3. **Write** the modified overlay back to the tier file using `ConfigRepository.WriteCustom` or `ConfigRepository.WriteLocal` (depending on the tier).
4. **Validate**: call `ConfigRepository.Load()` to produce the fully merged config. This runs `ValidateStructure` internally.
5. **Rollback on failure**: if validation fails, restore the original tier file content (saved before step 3) and return the validation error to the user. The config on disk remains as it was before the command ran.
6. **Output**: on success, print a confirmation message (human mode) or a JSON envelope (JSON mode).

The read-modify-write is not locked against concurrent writers. This matches `ConfigRepository`'s documented thread-safety contract: callers coordinate externally. In practice, only one CLI invocation or daemon iteration writes config at a time.

## Output

### Human Mode

Each command prints a single confirmation line:

```
Set daemon.poll_interval_seconds = 10 in local/config.json
Unset pipeline.stages.audit.enabled in local/config.json
Appended "review" to pipeline.stage_order in custom/config.json
Removed "review" from pipeline.stage_order in custom/config.json
```

### JSON Mode

Success:

```json
{
  "ok": true,
  "action": "config_set",
  "data": {
    "key": "daemon.poll_interval_seconds",
    "value": 10,
    "tier": "local"
  }
}
```

The `action` field matches the command: `config_set`, `config_unset`, `config_append`, `config_remove`.

For `unset`, the `value` field is `null`. For `remove`, the `value` field is the value that was removed.

Error:

```json
{
  "ok": false,
  "action": "config_set",
  "error": "descriptive message",
  "code": 1
}
```

## Error Conditions

| Scenario | Behavior |
|----------|----------|
| `.wolfcastle/` not found | Exit 1: `fatal: not a wolfcastle project (no .wolfcastle/ found)` |
| `--tier base` | Exit 1: `error: cannot write to base tier (base is managed by the system)` |
| `--tier` given invalid value | Exit 1: `error: --tier must be one of: local, custom` |
| Malformed key: empty segments | Exit 1: `error: invalid key "daemon..poll": empty path segment` |
| Malformed key: trailing dot | Exit 1: `error: invalid key "daemon.": trailing dot` |
| Malformed key: array indexing | Exit 1: `error: invalid key "commands[0]": array indexing is not supported` |
| `append` on non-array value | Exit 1: `error: cannot append to "logs.max_files": existing value is not an array` |
| `remove` on non-array value | Exit 1: `error: cannot remove from "logs.max_files": existing value is not an array` |
| `remove` value not found | Exit 1: `error: value not found in array at "pipeline.stage_order"` |
| Validation failure after write | Exit 1: `error: validation failed, rolled back: {validation error}`. Tier file restored to pre-write state |
| Tier file is malformed JSON | Exit 1: `error: {tier}/config.json is not valid JSON: {parse error}` |

## Identity Requirement

These commands do not require identity resolution. They operate on config files only, same as `config show`.

## Cobra Registration

Register as four subcommands of the existing `config` command group:

```
wolfcastle config set
wolfcastle config unset
wolfcastle config append
wolfcastle config remove
```

Each command registers its own `--tier` and `--json` flags. The positional arguments (`key`, `value`) are handled via `cobra.ExactArgs(2)` for `set`, `append`, and `remove`, and `cobra.ExactArgs(1)` for `unset`.

## Implementation Notes

- Reuse `readTierFile` from `cmd/config/show.go` for reading tier overlays. Consider extracting it to a shared location within `cmd/config/` if it isn't already.
- The dot-notation path walker and value parser are new utilities. Place them in `cmd/config/` as shared helpers since they serve all four write commands.
- For rollback, save the original file bytes (or "absent" sentinel) before writing. On validation failure, either write the saved bytes back or remove the file if it was absent.
- Use `json.Marshal` for value equality comparison in `remove`: marshal both the candidate and each element, compare the byte strings.
- The `unset` command writes `nil` into the overlay map, not `json.RawMessage("null")`. When the overlay is serialized, `nil` map values become JSON `null`, which `DeepMerge` treats as deletion on the next `Load`.
