# Stages Migration

Add MigrateStagesFormat() to MigrationService that converts existing array-format stage configs in tier files to dict-format. Must run after MigrateOldConfig() and be idempotent (no-op if stages are already a map). Wired into ScaffoldService.Reinit() call chain. Includes comprehensive tests for the migration path covering: array-to-map conversion, already-migrated files, empty stages, single-stage files, and stage_order generation from array ordering.
