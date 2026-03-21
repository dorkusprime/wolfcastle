# Dict-Format Pipeline Stages

**Governing ADRs**: ADR-018 (merge semantics), ADR-053 (centralized defaults), ADR-063 (three-tier configuration)

**Related Specs**: Config Schema (2026-03-12T00-01Z), Pipeline Stage Contract (2026-03-12T00-03Z), Unknown Field Detection (2026-03-20T15-57Z), MigrationService Contract (2026-03-18T22-45Z)

---

## 1. Motivation

The current `pipeline.stages` field is a JSON array. Arrays have full-replacement merge semantics per ADR-018: a higher tier providing `pipeline.stages` replaces the entire list from the tier below. This means a team that wants to change a single stage property (say, swap the model on `execute` from `heavy` to `mid`) must redeclare every stage in `custom/config.json`, duplicating the base defaults. The duplication is brittle. When a new Wolfcastle release adds or renames a default stage, the team's override silently masks it.

Switching `pipeline.stages` to an object (map) keyed by stage name brings it in line with `models`, which already benefits from per-key deep merge. A team can override `stages.execute.model` in `custom/config.json` without touching `stages.intake` at all. The base tier's defaults flow through for every key the higher tier does not mention.

The trade-off is that map keys are unordered in JSON. A separate `pipeline.stage_order` field restores explicit ordering while remaining a simple array that replaces per tier (the correct semantic for an ordering declaration).

---

## 2. New JSON Schema for `pipeline.stages`

`pipeline.stages` becomes an object whose keys are stage names (slugs) and whose values are `PipelineStage` objects. The `name` field is removed from `PipelineStage` because the map key serves that purpose.

### 2.1 PipelineStage Object

```json
{
  "model": "<string, required>",
  "prompt_file": "<string, required>",
  "enabled": "<boolean, optional, default: true>",
  "skip_prompt_assembly": "<boolean, optional, default: false>",
  "allowed_commands": "<[]string, optional>"
}
```

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `model` | string | yes | -- | Key into the top-level `models` dictionary. Resolved at pipeline load time; a missing key is a fatal config error. |
| `prompt_file` | string | yes | -- | Filename of the stage-specific prompt. Resolved through the three-tier merge (base/custom/local) per ADR-009 and ADR-018. Must be non-empty. |
| `enabled` | boolean | no | `true` | When `false`, the stage is skipped during pipeline execution. |
| `skip_prompt_assembly` | boolean | no | `false` | When `true`, the stage receives only its own `prompt_file` content, without rule fragments or script reference. |
| `allowed_commands` | []string | no | `nil` | Restricts which wolfcastle CLI commands the stage may invoke. When nil/absent, all commands are allowed. |

### 2.2 Full Pipeline Schema

```json
{
  "pipeline": {
    "stages": {
      "type": "object",
      "description": "Named pipeline stages. Keys are stage slugs, values are PipelineStage objects.",
      "additionalProperties": {
        "$ref": "#/$defs/PipelineStage"
      }
    },
    "stage_order": {
      "type": "array",
      "items": { "type": "string" },
      "description": "Execution order of pipeline stages. Each entry must name a key in stages."
    },
    "planning": { "..." : "unchanged" }
  }
}
```

### 2.3 Default Value

```json
{
  "pipeline": {
    "stages": {
      "intake": {
        "model": "mid",
        "prompt_file": "intake.md"
      },
      "execute": {
        "model": "heavy",
        "prompt_file": "execute.md"
      }
    },
    "stage_order": ["intake", "execute"]
  }
}
```

### 2.4 Go Type Changes

```go
type PipelineConfig struct {
    Stages     map[string]PipelineStage `json:"stages"`
    StageOrder []string                 `json:"stage_order,omitempty"`
    Planning   PlanningConfig           `json:"planning"`
}

type PipelineStage struct {
    // Name field removed; the map key is the stage name.
    Model              string   `json:"model"`
    PromptFile         string   `json:"prompt_file"`
    Enabled            *bool    `json:"enabled,omitempty"`
    SkipPromptAssembly *bool    `json:"skip_prompt_assembly,omitempty"`
    AllowedCommands    []string `json:"allowed_commands,omitempty"`
}
```

Code that previously accessed `stage.Name` instead uses the map key. Code that iterated `cfg.Pipeline.Stages` as a slice now iterates `cfg.Pipeline.StageOrder` and looks up each name in the map.

---

## 3. The `stage_order` Field

### 3.1 Purpose

JSON object key order is not guaranteed. `stage_order` is a `[]string` that declares the execution order for pipeline stages. The daemon iterates this array, looking up each name in the `stages` map.

### 3.2 Default When Omitted

If `stage_order` is absent (nil or not provided), the daemon sorts the map keys alphabetically and uses that as the execution order. This is deterministic and predictable, though rarely the desired order for a real pipeline. The default value in `base/config.json` always includes an explicit `stage_order`.

### 3.3 Merge Semantics

`stage_order` is an array. Per ADR-018, arrays are full-replacement: a higher tier providing `stage_order` replaces the lower tier's array entirely. This is the correct semantic. If a team reorders stages or inserts a new one, they express the complete desired order, not a partial patch.

---

## 4. Merge Semantics for `pipeline.stages`

With `stages` as a map, it inherits the recursive deep-merge behavior that ADR-018 defines for objects.

### 4.1 Per-Key Override

A higher tier can override individual properties of a stage without replacing the entire stages map. The merge recurses into each stage's object by key.

**Base tier:**
```json
{
  "pipeline": {
    "stages": {
      "intake": { "model": "mid", "prompt_file": "intake.md" },
      "execute": { "model": "heavy", "prompt_file": "execute.md" }
    },
    "stage_order": ["intake", "execute"]
  }
}
```

**Custom tier (team override):**
```json
{
  "pipeline": {
    "stages": {
      "execute": { "model": "mid" }
    }
  }
}
```

**Resolved:**
```json
{
  "pipeline": {
    "stages": {
      "intake": { "model": "mid", "prompt_file": "intake.md" },
      "execute": { "model": "mid", "prompt_file": "execute.md" }
    },
    "stage_order": ["intake", "execute"]
  }
}
```

The custom tier changed only `execute.model`. The `execute.prompt_file` inherited from base. The `intake` stage was untouched. The `stage_order` inherited from base because the custom tier did not provide one.

### 4.2 Adding a Stage

To add a new stage, a higher tier provides both the stage definition and an updated `stage_order`:

```json
{
  "pipeline": {
    "stages": {
      "lint": { "model": "fast", "prompt_file": "lint.md" }
    },
    "stage_order": ["intake", "lint", "execute"]
  }
}
```

The `lint` entry merges into the stages map alongside `intake` and `execute` from the base. The `stage_order` replaces entirely, positioning `lint` between the other two.

### 4.3 Removing a Stage

To remove a stage from execution, a higher tier either sets it to `null` (removing it from the merged map per ADR-018's null-deletion semantics) or sets `enabled: false` (keeping it in the map but skipping it). The `stage_order` should be updated accordingly.

Setting a stage to `null`:
```json
{
  "pipeline": {
    "stages": {
      "intake": null
    },
    "stage_order": ["execute"]
  }
}
```

Disabling without removing:
```json
{
  "pipeline": {
    "stages": {
      "intake": { "enabled": false }
    }
  }
}
```

---

## 5. Validation Rules

These rules apply to the resolved (post-merge) config.

### 5.1 Stage Name Format

Stage names (map keys) must match the pattern `[a-z][a-z0-9_-]*`. Slug format: lowercase alphanumeric with hyphens and underscores, starting with a letter. An empty string is not a valid stage name.

### 5.2 Stage Order Integrity

Every name in `stage_order` must exist as a key in `stages`. If `stage_order` references a name that does not exist in the map, that is a fatal config error.

Every key in `stages` should appear in `stage_order`. If a stage exists in the map but is absent from `stage_order`, the config loader emits a warning. The stage will never execute (it has no position in the pipeline), so this likely indicates a misconfiguration. It is a warning rather than an error because the stage may be intentionally defined but temporarily excluded from execution.

### 5.3 Model References

Every `model` value in every stage must reference a key that exists in the resolved `models` dictionary. This is unchanged from the current validation; the lookup just happens against map values instead of array elements.

### 5.4 Prompt File

Every stage's `prompt_file` must be non-empty. Prompt file existence validation (resolving through the three tiers) is unchanged.

### 5.5 Duplicate Detection

The array format required explicit duplicate-name detection. With a map, duplicate keys are impossible at the JSON level (the parser takes the last value). This validation rule is retired.

---

## 6. Migration Contract

The `MigrationService` (defined in the MigrationService Contract spec) gains a new method for converting old array-format stages to the new dict format.

### 6.1 Method Signature

```go
func (m *MigrationService) MigrateStagesFormat() error
```

### 6.2 Algorithm

For each tier file that exists (`base/config.json`, `custom/config.json`, `local/config.json`):

1. Read and parse the file as `map[string]any`.
2. Navigate to `pipeline.stages`. If absent, skip this tier.
3. Check the type of the `stages` value:
   - If it is already a map (`map[string]any`), this tier is already migrated. Skip.
   - If it is an array (`[]any`), convert it.
4. For each element in the array, extract the `name` field as the map key. Remove the `name` field from the element. Insert the element into a new `map[string]any` keyed by name.
5. If the array had entries, construct a `stage_order` array from the names in their original array order. Set `pipeline.stage_order` in the tier's map.
6. Replace `pipeline.stages` with the new map.
7. Write the modified map back to the tier file as indented JSON.

### 6.3 Idempotency

If `pipeline.stages` is already a map in every tier, the method performs no writes. Running it against an already-migrated directory is a no-op.

### 6.4 Edge Cases

| Case | Behavior |
|------|----------|
| `stages` is absent | No conversion needed; skip tier |
| `stages` is an empty array `[]` | Convert to empty map `{}`; no `stage_order` emitted |
| Array element missing `name` field | Skip element, emit warning |
| Duplicate names in array | Last-wins (matching JSON object duplicate-key semantics) |
| `stage_order` already exists alongside an array `stages` | Preserve existing `stage_order` rather than regenerating |

### 6.5 Invocation

`ScaffoldService.Reinit()` calls `MigrateStagesFormat()` alongside the existing migration methods, before regenerating base files:

```go
migrator := &MigrationService{config: s.config, root: s.root}
_ = migrator.MigrateDirectoryLayout()
_ = migrator.MigrateOldConfig()
_ = migrator.MigrateStagesFormat()
```

Migration errors are discarded (logged but not propagated), consistent with the existing pattern.

---

## 7. Impact on Unknown Field Detection

The unknown-field detection spec (2026-03-20T15-57Z) describes how `DisallowUnknownFields` catches misspelled config keys. The stages-to-dict change affects the field path format in warning messages and the recursion behavior of detection.

### 7.1 Path Format Change

With array-format stages, unknown fields within a stage produced paths using bracket notation:

```
config: unknown field "pipeline.stages[0].promt_file" in custom/config.json
```

With dict-format stages, the path uses dot-delimited map-key notation:

```
config: unknown field "pipeline.stages.execute.promt_file" in custom/config.json
```

This is a natural consequence of the structural change. The `DisallowUnknownFields` decoder (Approach A from the unknown-field spec) handles this automatically: when decoding into a `map[string]PipelineStage`, each map value is decoded as a `PipelineStage` struct, and the decoder checks field names within that struct. The map key becomes part of the path in the error message.

### 7.2 Recursion Behavior

With stages as a map, `diffKeys` (from Approach B, if used) recurses into each stage's config naturally by map key, the same way it already recurses into `models.fast`, `models.mid`, etc. No special array-index handling is needed for stages.

### 7.3 Valid Map Keys

Stage names are user-defined map keys, like model names. Unknown-field detection does not flag unrecognized stage names, only unrecognized fields within a stage's value object. A stage named `"typo-stage"` is not a warning; a field `"promt_file"` inside that stage is.

---

## 8. Impact on Existing Code

### 8.1 Config Loading (`internal/config`)

- `PipelineConfig.Stages` changes from `[]PipelineStage` to `map[string]PipelineStage`.
- `PipelineConfig.StageOrder` is a new `[]string` field.
- `Defaults()` returns a map and an explicit `StageOrder`.
- `Validate()` replaces the duplicate-name check with the stage-order integrity checks from Section 5.

### 8.2 Pipeline Execution (`internal/pipeline`)

- Stage iteration changes from ranging over `cfg.Pipeline.Stages` to ranging over `cfg.Pipeline.StageOrder` and looking up each stage by name.
- `AssemblePrompt` receives a stage name (string) alongside the `PipelineStage` value, since the name is no longer embedded in the struct.

### 8.3 Prompt Assembly

Unchanged in structure. The stage name is passed separately rather than read from `stage.Name`.

### 8.4 Tests

All tests that construct `PipelineStage` values with a `Name` field need updating. Tests that construct `PipelineConfig` with a `Stages` slice need to switch to a map and provide `StageOrder`.

### 8.5 Config Spec

The Config Schema spec (2026-03-12T00-01Z) must be updated to reflect the new schema, merge semantics paragraph, validation rules, and defaults table.

### 8.6 Pipeline Stage Contract

The Pipeline Stage Contract (2026-03-12T00-03Z) must be updated in Section 1 (Stage Definition Schema) to remove the `name` field and describe the map structure. Section 5 (Stage Ordering) should reference `stage_order`.

---

## 9. Backwards Compatibility

The migration in Section 6 ensures existing config files are converted automatically on the next `wolfcastle init` or `wolfcastle update`. Users who have never customized `pipeline.stages` (relying on defaults) are unaffected, since `base/config.json` is regenerated from the new Go defaults.

Users with custom stages in `custom/config.json` or `local/config.json` will have their files rewritten by the migration. The rewrite preserves all stage properties and execution order. The only visible change is the JSON structure.

No flag day is required. The migration runs as part of the normal rescaffold flow.
