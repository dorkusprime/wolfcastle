# ADR-018: Merge Semantics for Config and Prompt Layering

## Status
Accepted

## Date
2026-03-12

## Context
ADR-009 establishes a three-tier layering system (base → custom → local) but does not specify how merging works. Config files (JSON) and prompt/rule files (Markdown) have different needs. A user overriding one model definition shouldn't clobber the entire models dictionary. A user replacing a rule fragment should replace it entirely, not interleave with the base version.

## Decision

### Config: Deep Merge
`config.json` and `config.local.json` are merged via recursive deep merge. Keys in `config.local.json` override the same keys in `config.json` at the deepest level. Unspecified keys inherit from the lower tier.

Example — if `config.json` defines:
```json
{
  "models": {
    "fast": { "command": "claude", "args": ["--model", "haiku"] },
    "heavy": { "command": "claude", "args": ["--model", "opus"] }
  }
}
```

And `config.local.json` defines:
```json
{
  "models": {
    "heavy": { "command": "claude", "args": ["--model", "sonnet"] }
  }
}
```

The resolved config has both `fast` (from config.json) and `heavy` (from config.local.json). Only the `heavy` model is overridden.

### Arrays: Full Replacement
Arrays are never element-merged. An array in `config.local.json` completely replaces the array from `config.json`. This applies to `pipeline.stages`, model `args`, `prompts.fragments`, and any other array-valued field.

### Null Deletion
A field set to `null` in `config.local.json` removes that key from the resolved config. This allows local config to explicitly disable a team-defined setting.

### Prompts and Rules: File-Level Replacement
Prompt fragments and rule files in `base/`, `custom/`, and `local/` merge at the file level. A same-named file in a higher tier completely replaces the lower tier's version. There is no partial merge of Markdown content.

- `base/rules/git-conventions.md` exists → included
- `custom/rules/git-conventions.md` also exists → completely replaces base version
- `local/rules/git-conventions.md` also exists → completely replaces custom version

Files with no same-named override in a higher tier pass through unchanged. Files that exist only in a higher tier are added (not just replacements — new fragments compose too).

## Consequences
- Config overrides are surgical — change one key without affecting siblings
- Prompt/rule overrides are atomic — no risk of partial Markdown merging producing incoherent content
- Two distinct, well-defined merge strategies rather than one ambiguous one
- Deep merge implementation is straightforward in Go (recursive map merge)
- Developers can reason about resolved config by mentally layering keys, and resolved prompts by checking which tier has the file
