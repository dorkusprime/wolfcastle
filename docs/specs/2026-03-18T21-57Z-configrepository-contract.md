# ConfigRepository Contract

## Overview

`Repository` (formerly `ConfigRepository`) in `internal/config` owns three-tier configuration resolution. Callers receive a fully merged `*Config`; they never construct paths or read tier files themselves. The repository replaces the current `Load(wolfcastleDir)` function with a struct-based design backed by `tierfs.Resolver`, making tier resolution injectable for testing.

## Type

```go
type Repository struct {
    tiers tierfs.Resolver
    root  string
}
```

The `tiers` field provides file resolution across base, custom, and local tiers. The `root` field holds the wolfcastle root directory path (used by constructors and for path derivation when needed).

## Constructors

### NewRepository(wolfcastleRoot string) *Repository

Production constructor. Builds a `tierfs.FS` rooted at `filepath.Join(wolfcastleRoot, "system")` and stores it alongside the root path. This is the only place `tierfs.New` is called for config resolution.

### NewRepositoryWithTiers(tiers tierfs.Resolver, root string) *Repository

Testing constructor. Accepts an injected `tierfs.Resolver`, allowing tests to supply an in-memory or fixture-backed implementation without touching the filesystem. The `root` field is stored as given.

## Methods

### Load() (*Config, error)

Resolves the merged configuration across all tiers, returning a fully populated and validated `*Config`.

**Algorithm:**

1. Convert `Defaults()` to `map[string]any` via JSON round-trip (`structToMap`).
2. Iterate `tiers.TierDirs()` in order (base, custom, local). For each directory, attempt to read `config.json`. If the file exists, parse it as `map[string]any` and apply `DeepMerge` onto the accumulated result. If the file does not exist, skip silently. Permission errors and JSON parse errors propagate immediately.
3. Marshal the merged map back to JSON, then unmarshal into `*Config`.
4. Call `ValidateStructure` on the result. Validation failures propagate.
5. Return the validated `*Config`.

The key difference from the current `Load(wolfcastleDir)`: tier directories come from `tiers.TierDirs()` rather than hardcoded path construction against `configTiers`. The merge semantics (deep merge with null deletion) and validation step remain identical.

### WriteBase(cfg *Config) error

Marshals the full `*Config` to indented JSON and writes it via `tiers.WriteBase("config.json", data)`. This is a complete snapshot, replacing whatever the base tier previously held.

### WriteCustom(data map[string]any) error

Marshals the partial overlay map to indented JSON and writes it to the custom tier's `config.json`. The target path is derived from `tiers.TierDirs()[1]` (the custom tier directory). Creates parent directories as needed.

This writes a partial overlay, not a complete config. Only the keys present in `data` will appear in the custom tier file; on the next `Load`, they merge over base.

### WriteLocal(data map[string]any) error

Marshals the partial overlay map to indented JSON and writes it to the local tier's `config.json`. The target path is derived from `tiers.TierDirs()[2]` (the local tier directory). Creates parent directories as needed.

Same partial-overlay semantics as `WriteCustom`, but targeting the highest-priority tier.

## Error Behavior

All errors returned by ConfigRepository are prefixed with `"config:"` for consistent identification at call sites.

- **Load**: missing tier files are silently skipped. Permission errors on tier files propagate. JSON parse errors propagate. `structToMap` failures (marshaling defaults) propagate. `ValidateStructure` failures propagate.
- **WriteBase**: propagates errors from JSON marshaling and from `tiers.WriteBase`.
- **WriteCustom / WriteLocal**: propagates errors from JSON marshaling, directory creation, and file writing.

## Thread Safety

ConfigRepository holds an immutable `tierfs.Resolver` reference and an immutable `root` string. `Load` performs only reads with no shared mutable state. The `Write*` methods mutate the filesystem; concurrent writes to the same tier file are not synchronized by the repository. Callers requiring concurrent writes must coordinate externally.

## Invariants

- `Defaults()` always produces a valid `*Config` that passes `ValidateStructure`. A `Load` call with no tier files on disk returns exactly `Defaults()`.
- `TierDirs()` returns exactly three entries in base, custom, local order. `WriteCustom` and `WriteLocal` index into positions 1 and 2 respectively.
- `WriteBase` writes a complete config snapshot. `WriteCustom` and `WriteLocal` write partial overlays. The distinction is structural: base is always a full document, custom/local contain only overridden keys.
- The repository does not cache. Each `Load` call reads from disk. This matches the current `Load` function's behavior and avoids staleness in a system where config files may be edited between iterations.
