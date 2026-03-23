# MigrationService Contract

## Overview

`MigrationService` in `internal/project` handles layout and config migrations for users upgrading from older wolfcastle directory structures. It is separate from `ScaffoldService` because migrations serve a distinct audience (upgrading users) with a distinct error profile (non-fatal, best-effort). Both methods are idempotent: running them against an already-migrated directory is a no-op.

`ScaffoldService.Reinit()` calls all four migration methods before regenerating base files. Migration errors propagate (Reinit returns them), so callers see migration failures.

## Type

```go
type MigrationService struct {
    config *config.Repository
    root   string // path to .wolfcastle/
}
```

The `config` field provides access to `config.Repository` for tier-aware config writes during `MigrateOldConfig`. The `root` field is the path to the `.wolfcastle/` directory, from which all source and destination paths are derived.

`MigrationService` is not constructed via a public constructor. `ScaffoldService.Reinit()` builds it inline as a short-lived value:

```go
m := &MigrationService{config: s.config, root: s.root}
if err := m.MigrateDirectoryLayout(); err != nil { return ... }
if err := m.MigrateOldConfig(); err != nil { return ... }
if err := m.MigrateStagesFormat(); err != nil { return ... }
if err := m.MigratePromptLayout(); err != nil { return ... }
```

## Methods

### MigrateDirectoryLayout() error

Moves the pre-ADR-077 flat directory layout into `system/`. Idempotent: if `system/` already exists, returns nil immediately.

**Algorithm:**

1. Stat `{root}/system/`. If it exists, return nil (migration already done or fresh install).
2. Stat `{root}/base/`. If it does not exist, create `{root}/system/` and return (no old layout to migrate, but ensure the target directory is ready).
3. Create `{root}/system/` with `0755` permissions.
4. For each directory in `[base, custom, local, projects, logs]`: if the source `{root}/{dir}` exists, rename it to `{root}/system/{dir}`. Missing directories are silently skipped.
5. For each loose file in `[wolfcastle.pid, stop, daemon.log, daemon.meta.json]`: if the source `{root}/{file}` exists, rename it to `{root}/system/{file}`. Missing files are silently skipped.
6. Return nil on success, or a wrapped error from any failed rename or mkdir.

The rename calls use `os.Rename`, so source and destination must be on the same filesystem. This is always true within `.wolfcastle/`.

### MigrateOldConfig() error

Migrates pre-ADR-063 config files from the wolfcastle root into the three-tier layout under `system/`. Assumes `MigrateDirectoryLayout` has already run (so `system/` exists).

**Algorithm:**

1. **Root config.json to custom tier:**
   - Stat `{root}/config.json`. If absent, skip this phase.
   - Ensure `{root}/system/custom/` exists (mkdir with `0755`).
   - Stat `{root}/system/custom/config.json`. If it already exists, skip the copy (do not overwrite existing custom config) but still remove the old file.
   - Read `{root}/config.json`, write its contents to `{root}/system/custom/config.json` with `0644` permissions.
   - Remove `{root}/config.json`.

2. **config.local.json to local tier (with merge):**
   - Stat `{root}/config.local.json`. If absent, skip this phase.
   - Ensure `{root}/system/local/` exists (mkdir with `0755`).
   - Read and JSON-parse `{root}/config.local.json` into `map[string]any`.
   - If `{root}/system/local/config.json` already exists, read and JSON-parse it as the base map. Otherwise start from an empty map.
   - Deep-merge the old local data onto the existing map using `config.DeepMerge(existing, oldLocal)`. The old local values take precedence for conflicting keys.
   - Marshal the merged result as indented JSON (with trailing newline) and write to `{root}/system/local/config.json` with `0644` permissions.
   - Remove `{root}/config.local.json`.

The merge step preserves keys already present in `system/local/config.json` while layering in the migrated values. This means running `MigrateOldConfig` twice (if the old file somehow reappears) produces the same final state.

### MigratePromptLayout() error

Moves flat prompt files into the new subdirectory structure (`stages/`, `audits/`) within each tier. Idempotent: if `prompts/stages/` already exists in a tier, that tier is skipped.

**Algorithm:**

1. Define the stage files to move: `intake.md`, `execute.md`, `intake-planning.md`, `plan-initial.md`, `plan-amend.md`, `plan-review.md`, `plan-remediate.md`.
2. Define the audit files to move: `audit.md`.
3. For each tier (base, custom, local):
   a. Check if `{root}/system/{tier}/prompts/` exists. If not, skip the tier.
   b. Check if `{root}/system/{tier}/prompts/stages/` exists. If it does, skip the tier (already migrated).
   c. For each stage file: if it exists in `prompts/`, move it to `prompts/stages/`.
   d. For each audit file: if it exists in `prompts/`, move it to `prompts/audits/`.

### MigrateStagesFormat() error

Converts `pipeline.stages` from the old JSON array format to the new dict (map) format in each tier's `config.json`. Documented in detail in the Dict-Format Pipeline Stages spec (2026-03-21T03-11Z). Idempotent: if `stages` is already a map or absent, the tier is skipped.

## Error Behavior

All four methods return `error`. Migration errors propagate through `ScaffoldService.Reinit()`.

- **MigrateDirectoryLayout**: wraps errors from `os.MkdirAll` and `os.Rename` with context strings like `"creating system/: ..."` and `"migrating base to system/base: ..."`. Missing source directories and files are not errors. Directory names to move are derived from `tierfs.TierNames` plus `"projects"` and `"logs"`.
- **MigrateOldConfig**: wraps errors from file reads, JSON parsing, and file writes. Invalid JSON in `config.local.json` returns an error (the file is left in place). Missing source files are not errors.
- **MigratePromptLayout**: wraps errors from `os.MkdirAll` and `os.Rename`. Missing source files and tiers without a `prompts/` directory are silently skipped.
- **MigrateStagesFormat**: wraps errors from file reads, JSON marshaling, and file writes. Invalid JSON is silently skipped (not this method's responsibility). Missing tier files are silently skipped.

No method logs directly. The caller (`Reinit`) wraps errors with `"scaffold: migrating ..."` prefixes before returning them.

## Thread Safety

`MigrationService` is a short-lived value created and consumed within a single `Reinit` call. It holds no shared state and performs filesystem mutations that are inherently single-threaded in the daemon's lifecycle (rescaffold runs at daemon startup, before any concurrent iteration work begins). No synchronization is needed.

## Invariants

- `MigrateDirectoryLayout` is a no-op when `system/` exists. It will never move directories that have already been migrated.
- `MigrateOldConfig` will not overwrite an existing `system/custom/config.json`. If the custom tier already has config, the old root `config.json` is removed without copying.
- `MigrateOldConfig` merges into `system/local/config.json` rather than replacing it, preserving any keys that were set by other means.
- `MigratePromptLayout` is a no-op when `prompts/stages/` already exists in a tier. It processes all three tiers independently.
- `MigrateStagesFormat` is a no-op when `pipeline.stages` is already a map or absent. It processes all three tiers independently.
- All methods leave source files removed on success. A second call with no source files is a silent no-op.
- Neither `MigrateDirectoryLayout` nor `MigrateOldConfig` creates tier directories beyond what it needs (`system/custom/` and `system/local/`). The `system/base/` directory comes from `MigrateDirectoryLayout` (renaming the old `base/`), not from config migration.
