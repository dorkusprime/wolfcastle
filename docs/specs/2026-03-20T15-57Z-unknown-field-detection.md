# Unknown Field Detection in Config Unmarshalling

**Governing ADRs**: ADR-018 (null-deletion semantics), ADR-053 (centralized defaults), ADR-063 (three-tier configuration)

**Related Specs**: Config Schema (2026-03-12T00-01Z), ConfigRepository Contract (2026-03-18T21-57Z)

---

## 1. Problem

When a user misspells a config key (e.g., `"modles"` instead of `"models"`), the current loading pipeline silently ignores it. The typo vanishes into `json.Unmarshal`'s default behavior of dropping unrecognized fields, and the user sees the default value for the field they intended to set, with no indication that their override was discarded.

This is a common source of confusion: a user edits `local/config.json`, restarts the daemon, and wonders why nothing changed.

---

## 2. Detection Strategy

After the three-tier merge produces a `map[string]any` and before (or during) unmarshalling into the `Config` struct, compare the merged map's keys against the known JSON field names derived from the struct's `json` tags. Any key present in the map but absent from the struct is unknown.

Two approaches are viable. Both operate on the same data (the `map[string]any` produced by deep merge) and catch the same class of error. They differ in mechanism and in how gracefully they handle nested structures.

### 2.1 Approach A: `json.Decoder` with `DisallowUnknownFields()` (Recommended)

Replace the current `json.Unmarshal(merged, &cfg)` call with a `json.NewDecoder(bytes.NewReader(merged))` that has `DisallowUnknownFields()` set. When the decoder encounters a JSON key that does not match any `json` tag on the target struct (or its nested structs), it returns an error of the form `json.UnmarshalTypeError` or a string-formatted error naming the offending field.

**Where it plugs in**: lines 157-165 of `config.go` (the standalone `Load`) and lines 64-71 of `repository.go` (`ConfigRepository.Load`). The change is surgical: swap `json.Unmarshal` for a decoder, capture the error, and convert it to a warning instead of failing.

**Advantages**:
- Idiomatic Go; uses the stdlib's own mechanism for exactly this purpose.
- The Config struct already mirrors the JSON schema field-for-field, so `DisallowUnknownFields` can operate without any mapping tables or reflection.
- Recursion into nested structs is automatic. `PipelineConfig`, `DaemonConfig`, `LogsConfig`, and every other nested type will be checked without additional code.

**Limitations**:
- The decoder stops at the first unknown field. To collect all unknown fields in a single pass, we would need to catch the error, strip the offending key from the map, and retry. This is a loop, but the number of unknown fields is expected to be small (usually zero or one).
- `map[string]ModelDef` and `map[string]ClassDef` fields cannot flag unknown keys by nature; any key is valid as a map key. This is correct behavior (model names and class names are user-defined).

### 2.2 Approach B: Post-unmarshal Diff

After unmarshalling into `Config`, marshal the struct back to a `map[string]any` (via `json.Marshal` then `json.Unmarshal` into a map). Walk both maps recursively. Any key present in the input map but absent in the round-tripped output map is unknown.

**Advantages**:
- Collects all unknown fields in a single pass, no retry loop.
- Works even if the struct uses custom `UnmarshalJSON` methods that absorb unknown fields.

**Limitations**:
- More code: a recursive map-diff function, plus the round-trip marshal/unmarshal.
- Fields with `omitempty` that happen to have their zero value after unmarshalling will be absent from the round-tripped map, producing false positives. Mitigation requires inspecting `omitempty` tags during the diff, which adds complexity.
- `map[string]ModelDef` entries survive the round trip, so they won't produce false positives, but the diff walker needs to know when to skip map-typed fields.

### 2.3 Implementation Choice

The implementation uses **Approach B** (post-unmarshal diff via round-trip). The `checkUnknownFields` function in `internal/config/unknown.go` unmarshals leniently into `Config` (unknown fields silently dropped), marshals back to a `map[string]any` via `structToMap`, then recursively diffs the original map's keys against the round-tripped map's keys. Any key present in the original but absent after the round-trip is flagged as unknown.

This approach was chosen over Approach A because it collects all unknown fields in a single pass (no retry loop) and handles nested structs and array elements naturally via the recursive `diffKeys` function. The `omitempty` false-positive concern documented for Approach B is mitigated by the fact that Config defaults are always fully populated via `structToMap`, so round-tripped zero values are unlikely.

---

## 3. Severity: Warnings, Not Errors

Unknown fields must not prevent config loading. The rationale:

- **Forward compatibility**: a user running an older version of Wolfcastle may have config keys introduced in a newer version. Rejecting these would break downgrade scenarios and make `custom/config.json` (shared across a team) fragile when team members run different versions.
- **Graceful degradation**: the config system already has well-defined defaults for every field. An unrecognized key simply means the user's intent was not applied, which is worth reporting but not worth blocking.

Unknown fields are surfaced as warnings. The daemon, CLI commands, and any other caller can decide how to present them (stderr, structured log, TUI status line).

---

## 4. Warning Format

Each warning identifies the unknown field by its JSON path and, when per-tier detection is active, names the tier that introduced it:

```
config: unknown field "modles" in local/config.json
config: unknown field "pipeline.stages[0].promt_file" in custom/config.json
config: unknown field "daemn" in merged config
```

When per-tier detection is not performed (merged-only mode), the warning omits the tier:

```
config: unknown field "modles" in merged config
```

The field path uses dot-delimited notation for nested objects and bracket notation for array indices, matching the style Go's JSON decoder uses in its own error messages.

---

## 5. Return Type Change

The current signatures are:

```go
// config.go
func Load(wolfcastleDir string) (*Config, error)

// repository.go
func (r *ConfigRepository) Load() (*Config, error)
```

Two options for surfacing warnings.

### 5.1 Option A: Warnings on the Config struct

Add a `Warnings []string` field to `Config` (excluded from JSON via `json:"-"`):

```go
type Config struct {
    // ... existing fields ...
    Warnings []string `json:"-"`
}
```

**Advantages**: no signature change, no disruption to callers. Warnings travel with the config value. Callers that care can inspect `cfg.Warnings`; callers that don't can ignore it.

**Disadvantages**: attaching diagnostic metadata to a data struct is slightly impure. The field must be excluded from JSON serialization (via `json:"-"`) to avoid polluting config output, and it must be excluded from the unknown-field check itself (since it has no corresponding JSON key).

### 5.2 Option B: Return `(*Config, []Warning, error)`

Change the signature to return warnings as a separate value:

```go
type Warning struct {
    Field   string // JSON field path
    Tier    string // tier name or "merged"
    Message string
}

func Load(wolfcastleDir string) (*Config, []Warning, error)
```

**Advantages**: clean separation of concerns. Warnings are a distinct return channel.

**Disadvantages**: breaks every caller. Both `Load` functions and all call sites need updating.

### 5.3 Recommendation

Use Option A (warnings on the struct) for the initial implementation. It is non-breaking and sufficient. If the warning system grows in complexity (e.g., typed warning codes, severity levels), migrate to Option B in a subsequent change. The `json:"-"` tag keeps warnings out of serialized output, and the field being a simple `[]string` keeps the implementation minimal.

---

## 6. Per-Tier Detection

The ideal behavior is to detect unknown fields in each tier file individually, before the merge, so the warning can name the source file. This tells the user exactly which file to edit to fix the typo.

### 6.1 Strategy

For each tier file that exists (base, custom, local), unmarshal its raw JSON into `Config` using `DisallowUnknownFields`. Capture any unknown-field errors and record them as warnings tagged with the tier name. Then proceed with the normal merge-and-unmarshal pipeline.

This means each tier file is decoded twice: once for unknown-field detection (into a throwaway `Config`), and once as a `map[string]any` for the merge. The performance cost is negligible (config files are small, loaded once at startup).

### 6.2 Partial Overlays

Tier files are partial overlays, not complete configs. A `custom/config.json` might contain only `{"daemon": {"log_level": "debug"}}`. When decoded into a full `Config` struct, every field not present in the overlay takes its zero value. This is fine for unknown-field detection: `DisallowUnknownFields` only flags keys that are present but unrecognized. Missing keys are not flagged.

### 6.3 Map-Typed Fields in Tier Files

`Models` (`map[string]ModelDef`) and `TaskClasses` (`map[string]ClassDef`) accept arbitrary keys by design. Per-tier detection handles these correctly because `DisallowUnknownFields` does not inspect map keys, only struct field names. A tier file with `{"models": {"custom-model": {...}}}` will not trigger a warning, which is the correct behavior.

### 6.4 Fallback

If per-tier detection proves too complex to implement cleanly (e.g., due to interactions with null-deletion semantics from ADR-018), the merged-only detection from Section 2 is an acceptable first step. Per-tier detection can be added later without changing the warning format or the API surface.

---

## 7. Implementation Outline

### 7.1 New Types

```go
// In internal/config/types.go

type Config struct {
    // ... existing fields ...
    Warnings []string `json:"-"`
}
```

### 7.2 Detection Function

```go
// checkUnknownFields detects JSON keys that don't correspond to any
// field in the Config struct via round-trip diffing.
func checkUnknownFields(raw []byte, tier string) []string
```

This function:
1. Unmarshals the raw JSON into `map[string]any` (the original keys).
2. Unmarshals the raw JSON leniently into `Config` (unknown fields silently dropped).
3. Round-trips the `Config` back to `map[string]any` via `structToMap` (only known fields survive).
4. Recursively diffs the original map against the round-tripped map via `diffKeys`.
5. Any key present in the original but absent from the round-trip is flagged as unknown.
6. Returns the accumulated warnings. Array elements are compared pairwise for nested unknown fields.

### 7.3 Integration Points

In both `Load` functions (`config.go` and `repository.go`):

1. Before merging, run `checkUnknownFields` against each tier file's raw JSON, collecting warnings with tier labels.
2. After merging and before the final unmarshal, optionally run `checkUnknownFields` against the merged JSON as a catch-all (labeled "merged config").
3. Attach all collected warnings to the returned `Config.Warnings`.

### 7.4 Caller Responsibility

Callers of `Load` that wish to surface warnings should check `cfg.Warnings` after a successful load and present them appropriately (stderr for CLI, structured log for daemon). The config package itself does not write to stderr or log; it only populates the warnings slice.

---

## 8. Edge Cases

| Case | Behavior |
|------|----------|
| No unknown fields | `cfg.Warnings` is nil; no output |
| Multiple unknown fields in one tier | Each produces a separate warning |
| Same typo in multiple tiers | Each tier produces its own warning (the user needs to fix all of them) |
| `null` value for a known field (ADR-018 deletion) | Not an unknown field; handled by merge semantics as usual |
| `null` value for an unknown field | Flagged as unknown. The null-deletion semantic only applies during merge; the field name itself is still unrecognized |
| Nested unknown field (e.g., `pipeline.planing`) | Caught by `DisallowUnknownFields` during recursive struct decode |
| Unknown key inside a `map[string]ModelDef` value (e.g., `models.fast.commnd`) | Caught: `ModelDef` is a struct, so its fields are checked |
| Unknown model name (e.g., `models.typo-model`) | Not caught and should not be: model names are user-defined map keys |
| `base/config.json` absent | No per-tier check for that tier; merged check still runs |
