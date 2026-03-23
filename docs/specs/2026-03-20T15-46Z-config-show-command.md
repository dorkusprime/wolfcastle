# Config Show Command

Display the resolved Wolfcastle configuration, or inspect individual tier files for debugging. This is a read-only diagnostic command that requires no model invocation.

## Governing ADRs

- ADR-009: Three-tier config hierarchy (base < custom < local)
- ADR-013: Model definitions
- ADR-063: Directory structure and gitignore rules

## Synopsis

```
wolfcastle config show [section] [--tier <base|custom|local>] [--raw] [--json]
```

## Description

Prints the Wolfcastle configuration to stdout as indented JSON. By default, the output reflects the fully resolved config: hardcoded defaults merged with base, custom, and local tiers (the same object returned by `config.Load()`). The structure matches the `Config` type defined in `internal/config/types.go`.

An optional positional `section` argument filters the output to a single top-level key. For example, `wolfcastle config show pipeline` prints only the `pipeline` object. If the given section name does not exist in the resolved config, the command exits with an error listing all valid section names. The section filter applies after mode resolution (default, `--tier`, or `--raw`), so it works with any combination of flags.

Two flags modify what is shown:

- `--tier` restricts output to a single tier file's raw JSON content, before merge.
- `--raw` suppresses the hardcoded defaults layer, showing only what the tier files actually contain.

These flags serve different debugging needs. A user wondering "what is my effective retry config?" runs `wolfcastle config show` with no flags. A user wondering "did I override retries in local?" runs `wolfcastle config show --tier local`. A user wondering "what do the tier files contain without defaults mixed in?" runs `wolfcastle config show --raw`.

## Arguments and Flags

| Argument / Flag | Type | Required | Default | Description |
|-----------------|------|----------|---------|-------------|
| `section` | positional | No | (none) | Filter output to a single top-level key (e.g. `pipeline`, `models`, `logs`). If the key does not exist, exit 1 with the list of valid sections |
| `--tier` | string | No | (none) | Display a single tier's raw JSON content. Accepted values: `base`, `custom`, `local`. Omitting this flag shows the fully merged config |
| `--raw` | boolean | No | `false` | Suppress the hardcoded defaults layer. When used alone (without `--tier`), merges only the three tier files without seeding from `Defaults()`. When used with `--tier`, identical to `--tier` alone (tier files never include defaults) |
| `--json` | boolean | No | `false` | Wrap output in the standard `{ok, action, data}` envelope |

## Behavior

### Default mode (no flags)

1. Locate the `.wolfcastle/` directory (walk ancestors as usual). Exit 1 if not found.
2. Call `config.Load(wolfcastleDir)` to obtain the fully merged `*Config`.
3. Marshal the config to indented JSON (two-space indent, no HTML escaping).
4. Print to stdout.

### `--tier` mode

1. Locate `.wolfcastle/`. Exit 1 if not found.
2. Resolve the tier file path:
   - `base` reads `.wolfcastle/system/base/config.json`
   - `custom` reads `.wolfcastle/system/custom/config.json`
   - `local` reads `.wolfcastle/system/local/config.json`
3. Read the file contents.
   - If the file does not exist, treat the content as `{}`.
   - If the file exists but is not valid JSON, exit 1 with an error message.
4. Pretty-print the JSON to stdout (re-marshal with indentation for consistent formatting).

### `--raw` mode (without `--tier`)

1. Locate `.wolfcastle/`. Exit 1 if not found.
2. Read all three tier files (base, custom, local). Missing files are treated as `{}`.
3. Deep-merge the three tier objects in order (base < custom < local) without seeding from `Defaults()`.
4. Pretty-print the merged result to stdout.

### `--raw` with `--tier`

Identical to `--tier` alone. Tier files never contain the hardcoded defaults layer, so `--raw` is a no-op in this combination. No warning is emitted.

### `--json` mode

When `--json` is active, wrap the output in the standard response envelope:

```json
{
  "ok": true,
  "action": "config_show",
  "data": {
    "version": 1,
    "models": { ... },
    "pipeline": { ... },
    ...
  }
}
```

The `data` field contains the same JSON object that would have been printed to stdout in plain mode. The `action` string is `"config_show"`.

On error:

```json
{
  "ok": false,
  "action": "config_show",
  "error": "descriptive message",
  "code": 1
}
```

### Section filtering

The optional `section` positional argument is applied as the final step, after the mode (default, `--tier`, or `--raw`) has resolved the config to a JSON object. The flow is:

1. Resolve the config via the selected mode (default merge, `--tier`, or `--raw`).
2. If no section argument was given, proceed to output.
3. If a section was given, convert the result to a map (for `*Config`, this means a JSON marshal-roundtrip), then look up the key.
   - If the key exists, replace the output with that key's value.
   - If the key does not exist, exit 1 with: `unknown section "X"; valid sections: a, b, c` (sections listed alphabetically).
4. Proceed to output formatting (`--json` envelope wraps the filtered value, not the full config).

The section argument composes freely with `--tier`, `--raw`, and `--json`. For example, `wolfcastle config show pipeline --tier base --json` returns only the `pipeline` key from the base tier file, wrapped in the JSON envelope.

## Identity Requirement

This command does **not** require identity resolution. It reads config files only, with no need to locate a project directory. It should work immediately after `wolfcastle init`, even before the daemon has started.

## Output Examples

### Merged config (default)

```bash
$ wolfcastle config show
{
  "version": 1,
  "models": {
    "fast": {
      "command": "claude",
      "args": ["-p", "--model", "haiku", ...]
    },
    "mid": { ... },
    "heavy": { ... }
  },
  "pipeline": {
    "stages": [ ... ],
    "planning": { "enabled": true, ... }
  },
  "logs": { "max_files": 50, "max_age_days": 7, "compress": true },
  "retries": { "initial_delay_seconds": 5, ... },
  ...
}
```

### Filtered by section

```bash
$ wolfcastle config show pipeline
{
  "stages": [ ... ],
  "planning": { "enabled": true, ... }
}
```

### Section with `--tier`

```bash
$ wolfcastle config show models --tier base
{
  "fast": { "command": "claude", "args": [...] },
  "mid": { ... },
  "heavy": { ... }
}
```

### Unknown section

```bash
$ wolfcastle config show nosuchkey
error: unknown section "nosuchkey"; valid sections: archive, identity, logs, models, pipeline, retries, version
```

### Single tier (local overrides only)

```bash
$ wolfcastle config show --tier local
{
  "identity": {
    "user": "wild",
    "machine": "macbook-pro"
  }
}
```

### Missing tier file

```bash
$ wolfcastle config show --tier custom
{}
```

### JSON envelope

```bash
$ wolfcastle config show --tier base --json
{
  "ok": true,
  "action": "config_show",
  "data": {
    "version": 1,
    "models": { ... },
    ...
  }
}
```

## Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Success |
| 1 | No `.wolfcastle/` directory found |
| 1 | Tier file contains malformed JSON |
| 1 | Invalid `--tier` value (not one of `base`, `custom`, `local`) |
| 1 | Unknown section name (positional argument does not match any top-level key) |

## Error Cases

| Scenario | Behavior |
|----------|----------|
| `.wolfcastle/` not found | Exit 1: `fatal: not a wolfcastle project (no .wolfcastle/ found)` |
| `--tier` given invalid value | Exit 1: `error: --tier must be one of: base, custom, local` |
| Tier file missing | Print `{}` (not an error) |
| Tier file is malformed JSON | Exit 1: `error: {tier}/config.json is not valid JSON: {parse error}` |
| Unknown section argument | Exit 1: `error: unknown section "X"; valid sections: a, b, c` (alphabetical) |
| `config.Load()` fails (merged mode) | Exit 1: `error: failed to load config: {underlying error}` |

## Cobra Registration

Register as a subcommand of `config`:

```
wolfcastle config show
```

The `config` command group is a bare group (no action of its own) under the root command. Its subcommands are `show`, `set`, `unset`, `append`, `remove`, and `validate`.

## Implementation Notes

- Use `json.MarshalIndent` with two-space indent and no prefix for output formatting.
- Set `json.Encoder.SetEscapeHTML(false)` to avoid mangling `&`, `<`, `>` in string values.
- The `--raw` merge (without defaults) can reuse the existing `DeepMerge` utility from the config package, just starting with an empty map instead of `Defaults()`.
- No `App.RequireIdentity()` call. The command needs only the wolfcastle directory path, not the engineer identity.
