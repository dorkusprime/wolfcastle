# Config Versioning and Migration

## Status
Accepted

## Date
2026-03-24

## Context

Wolfcastle's configuration schema will evolve as features are added, renamed, or restructured. Without a versioning mechanism, users upgrading the binary would face silent breakage or confusing validation errors when their on-disk config no longer matches the expected shape. Manually migrating config files is error-prone and hostile to automation.

The config already carried a `version` field (defaulting to 1) but nothing read or acted on it.

## Decision

Introduce a migration registry that upgrades config schemas automatically at load time.

**Version constant.** A `CurrentVersion` constant in the `config` package defines the latest schema version the binary understands. `Defaults()` sets `Version` to `CurrentVersion`.

**Migration registry.** A sorted slice of `Migration` structs, each carrying `FromVersion`, `ToVersion`, a human-readable `Description`, and a `Migrate func(raw map[string]any) error`. Migrations operate on the raw JSON map before deserialization into the `Config` struct, giving them freedom to rename fields, restructure nested objects, and delete obsolete keys.

**Automatic application.** Both `config.Load()` (the standalone loader) and `Repository.Load()` call `MigrateConfig` on the merged result before unmarshaling. Migration descriptions are appended to `Config.Warnings` so callers can surface them.

**Future version rejection.** `ValidateStructure` rejects configs whose version exceeds `CurrentVersion`, producing a clear "upgrade wolfcastle" error. `MigrateConfig` returns an error for the same condition.

**Backward compatibility.** Missing or zero version fields are treated as version 1.

**Startup advisory.** `wolfcastle start` checks the base config's version. If it trails `CurrentVersion`, it prints an informational message (not blocking) suggesting `wolfcastle init --force`.

**Seed migration.** A no-op v1 to v2 migration demonstrates the pattern and exercises the full path in tests.

## Consequences

- Config schema changes can be made safely by adding a new migration entry and bumping `CurrentVersion`.
- Users upgrading the binary see migration warnings in logs but are not blocked unless the config is from a newer binary.
- The base config advisory on startup nudges users toward regenerating defaults without forcing it.
- Each migration must be idempotent (safe to re-apply if the version field wasn't persisted between runs).
