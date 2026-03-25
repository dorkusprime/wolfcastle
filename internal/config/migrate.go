package config

import "fmt"

// CurrentVersion is the latest config schema version this binary understands.
const CurrentVersion = 2

// Migration describes a single config schema upgrade step. The Migrate
// function receives the raw map (pre-deserialization) and may rename,
// delete, or restructure fields freely.
type Migration struct {
	FromVersion int
	ToVersion   int
	Description string
	Migrate     func(raw map[string]any) error
}

// migrations is the ordered registry of all known schema migrations.
// Entries must be sorted by FromVersion ascending.
var migrations = []Migration{
	{
		FromVersion: 1,
		ToVersion:   2,
		Description: "Version 2 is structurally identical to version 1. This migration exists to demonstrate the upgrade path.",
		Migrate: func(raw map[string]any) error {
			// No-op: schema is unchanged. Real migrations would rename
			// keys, restructure nested objects, etc.
			return nil
		},
	},
}

// MigrateConfig applies sequential migrations to raw until it reaches
// CurrentVersion. It returns the (possibly mutated) map, a slice of
// human-readable descriptions for each migration applied, and any error
// encountered during migration.
func MigrateConfig(raw map[string]any) (map[string]any, []string, error) {
	version := configVersion(raw)

	if version > CurrentVersion {
		return nil, nil, fmt.Errorf("config version %d is newer than this binary supports (%d); upgrade wolfcastle", version, CurrentVersion)
	}

	var applied []string
	for _, m := range migrations {
		if version < m.FromVersion {
			// Gap in the registry; shouldn't happen but be defensive.
			continue
		}
		if version != m.FromVersion {
			continue
		}
		if err := m.Migrate(raw); err != nil {
			return nil, applied, fmt.Errorf("migration v%d->v%d failed: %w", m.FromVersion, m.ToVersion, err)
		}
		version = m.ToVersion
		raw["version"] = float64(version) // JSON numbers are float64
		applied = append(applied, m.Description)
	}

	return raw, applied, nil
}

// configVersion reads the "version" field from raw config. Missing or
// zero values are treated as version 1 for backward compatibility.
func configVersion(raw map[string]any) int {
	v, ok := raw["version"]
	if !ok {
		return 1
	}
	switch n := v.(type) {
	case float64:
		if n == 0 {
			return 1
		}
		return int(n)
	case int:
		if n == 0 {
			return 1
		}
		return n
	default:
		return 1
	}
}
