package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
)

// ═══════════════════════════════════════════════════════════════════════════
// BaseVersion: all branches (valid override, missing file, bad JSON, no field)
// ═══════════════════════════════════════════════════════════════════════════

func TestBaseVersion_DefaultFromEnvironment(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)

	// NewEnvironment writes the default config (version 2) to base tier.
	got := repo.BaseVersion()
	if got != config.CurrentVersion {
		t.Errorf("BaseVersion() = %d, want %d", got, config.CurrentVersion)
	}
}

func TestBaseVersion_OverriddenVersion(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	// Overwrite the base config with a different version.
	if err := env.Tiers.WriteBase("config.json", []byte(`{"version": 99}`)); err != nil {
		t.Fatalf("writing base config: %v", err)
	}

	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)
	got := repo.BaseVersion()
	if got != 99 {
		t.Errorf("BaseVersion() = %d, want 99", got)
	}
}

func TestBaseVersion_InvalidJSON(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	// Write invalid JSON to the base config.
	if err := env.Tiers.WriteBase("config.json", []byte(`{not valid json`)); err != nil {
		t.Fatalf("writing base config: %v", err)
	}

	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)
	got := repo.BaseVersion()
	if got != 0 {
		t.Errorf("BaseVersion() = %d, want 0 for invalid JSON", got)
	}
}

func TestBaseVersion_MissingFile(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	// Remove the base config file.
	dirs := env.Tiers.TierDirs()
	if len(dirs) > 0 {
		_ = os.Remove(filepath.Join(dirs[0], "config.json"))
	}

	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)
	got := repo.BaseVersion()
	if got != 0 {
		t.Errorf("BaseVersion() = %d, want 0 for missing file", got)
	}
}

func TestBaseVersion_NoVersionField(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	// Write valid JSON without a version field.
	if err := env.Tiers.WriteBase("config.json", []byte(`{"daemon": {}}`)); err != nil {
		t.Fatalf("writing base config: %v", err)
	}

	repo := config.NewRepositoryWithTiers(env.Tiers, env.Root)
	// Missing version field is treated as implicit v1 by configVersion().
	got := repo.BaseVersion()
	if got != 1 {
		t.Errorf("BaseVersion() = %d, want 1 for missing version field (implicit v1)", got)
	}
}
