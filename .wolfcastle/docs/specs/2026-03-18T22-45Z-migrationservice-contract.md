# MigrationService Contract

## Overview

`MigrationService` in `internal/project` handles layout and config migrations for users upgrading from older wolfcastle directory structures. It is separate from `ScaffoldService` because migrations serve a distinct audience (upgrading users) with a distinct error profile (non-fatal, best-effort). Both methods are idempotent: running them against an already-migrated directory is a no-op.

`ScaffoldService.Reinit()` calls both migration methods before regenerating base files. Migration errors are discarded (logged but not propagated), so rescaffold proceeds even on a clean install where there is nothing to migrate.

## Type

```go
type MigrationService struct {
    config *config.Repository
    root   string // path to .wolfcastle/
}
```

The `config` field provides access to `ConfigRepository` for tier-aware config writes during `MigrateOldConfig`. The `root` field is the path to the `.wolfcastle/` directory, from which all source and destination paths are derived.

`MigrationService` is not constructed via a public constructor. `ScaffoldService.Reinit()` builds it inline as a short-lived value:

```go
migrator := &MigrationService{config: s.config, root: s.root}
_ = migrator.MigrateDirectoryLayout()
_ = migrator.MigrateOldConfig()
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

## Error Behavior

Both methods return `error`, but callers discard the return value. Errors are informational, suitable for logging, not for control flow.

- **MigrateDirectoryLayout**: wraps errors from `os.MkdirAll` and `os.Rename` with context strings like `"creating system: ..."` and `"migrating base to system/base: ..."`. Missing source directories and files are not errors.
- **MigrateOldConfig**: wraps errors from file reads, JSON parsing, and file writes. Invalid JSON in `config.local.json` returns an error (the file is left in place). Missing source files are not errors. `os.MkdirAll` failures for tier directories are silently discarded (best-effort directory creation).

Neither method logs directly. The caller (`Reinit`) is responsible for logging if it chooses to inspect the error before discarding it.

## Thread Safety

`MigrationService` is a short-lived value created and consumed within a single `Reinit` call. It holds no shared state and performs filesystem mutations that are inherently single-threaded in the daemon's lifecycle (rescaffold runs at daemon startup, before any concurrent iteration work begins). No synchronization is needed.

## Invariants

- `MigrateDirectoryLayout` is a no-op when `system/` exists. It will never move directories that have already been migrated.
- `MigrateOldConfig` will not overwrite an existing `system/custom/config.json`. If the custom tier already has config, the old root `config.json` is removed without copying.
- `MigrateOldConfig` merges into `system/local/config.json` rather than replacing it, preserving any keys that were set by other means.
- Both methods leave the source files removed on success. A second call with no source files is a silent no-op.
- Neither method creates tier directories beyond what it needs (`system/custom/` and `system/local/`). The `system/base/` directory comes from `MigrateDirectoryLayout` (renaming the old `base/`), not from config migration.
