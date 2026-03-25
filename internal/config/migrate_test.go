package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateConfig_CurrentVersion_Noop(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"version": float64(CurrentVersion),
	}
	result, descs, err := MigrateConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(descs) != 0 {
		t.Errorf("expected no migrations applied, got %d: %v", len(descs), descs)
	}
	if result["version"] != float64(CurrentVersion) {
		t.Errorf("version should remain %d, got %v", CurrentVersion, result["version"])
	}
}

func TestMigrateConfig_FutureVersion_Error(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"version": float64(CurrentVersion + 1),
	}
	_, _, err := MigrateConfig(raw)
	if err == nil {
		t.Fatal("expected error for future version")
	}
	if !strings.Contains(err.Error(), "newer than this binary supports") {
		t.Errorf("expected 'newer than this binary supports' in error, got: %v", err)
	}
}

func TestMigrateConfig_MissingVersion_TreatedAsOne(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"failure": map[string]any{"hard_cap": float64(50)},
	}
	result, descs, err := MigrateConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should apply the v1->v2 migration
	if len(descs) == 0 {
		t.Error("expected at least one migration to be applied for missing version")
	}
	if result["version"] != float64(CurrentVersion) {
		t.Errorf("version should be %d after migration, got %v", CurrentVersion, result["version"])
	}
}

func TestMigrateConfig_VersionZero_TreatedAsOne(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"version": float64(0),
	}
	result, descs, err := MigrateConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(descs) == 0 {
		t.Error("expected migration from version 0 (treated as 1)")
	}
	if result["version"] != float64(CurrentVersion) {
		t.Errorf("version should be %d, got %v", CurrentVersion, result["version"])
	}
}

func TestMigrateConfig_Version1_MigratesToCurrent(t *testing.T) {
	t.Parallel()
	raw := map[string]any{
		"version": float64(1),
	}
	result, descs, err := MigrateConfig(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(descs) != 1 {
		t.Errorf("expected exactly 1 migration, got %d", len(descs))
	}
	if result["version"] != float64(2) {
		t.Errorf("expected version 2, got %v", result["version"])
	}
}

func TestMigrations_RegistryOrdering(t *testing.T) {
	t.Parallel()
	for i := 1; i < len(migrations); i++ {
		if migrations[i].FromVersion < migrations[i-1].FromVersion {
			t.Errorf("migrations not sorted: [%d].FromVersion=%d < [%d].FromVersion=%d",
				i, migrations[i].FromVersion, i-1, migrations[i-1].FromVersion)
		}
	}
}

func TestMigrations_ChainCovers(t *testing.T) {
	t.Parallel()
	// Verify the migration chain reaches CurrentVersion from version 1.
	version := 1
	for _, m := range migrations {
		if m.FromVersion == version {
			version = m.ToVersion
		}
	}
	if version != CurrentVersion {
		t.Errorf("migration chain ends at version %d, expected %d", version, CurrentVersion)
	}
}

func TestConfigVersion_Defaults(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  map[string]any
		want int
	}{
		{"missing key", map[string]any{}, 1},
		{"zero float", map[string]any{"version": float64(0)}, 1},
		{"zero int", map[string]any{"version": 0}, 1},
		{"valid float", map[string]any{"version": float64(2)}, 2},
		{"valid int", map[string]any{"version": 2}, 2},
		{"string value", map[string]any{"version": "bad"}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := configVersion(tt.raw)
			if got != tt.want {
				t.Errorf("configVersion(%v) = %d, want %d", tt.raw, got, tt.want)
			}
		})
	}
}

func TestLoad_AppliesMigrations(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Write a version-1 base config. Load should migrate transparently.
	_ = os.MkdirAll(filepath.Join(dir, "system", "base"), 0755)
	_ = os.WriteFile(
		filepath.Join(dir, "system", "base", "config.json"),
		[]byte(`{"version": 1}`),
		0644,
	)

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Version != CurrentVersion {
		t.Errorf("expected version %d after migration, got %d", CurrentVersion, cfg.Version)
	}
	// Migration descriptions should appear as warnings.
	found := false
	for _, w := range cfg.Warnings {
		if strings.Contains(w, "config migrated") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected migration warning in Config.Warnings")
	}
}

func TestValidateStructure_FutureVersion_ReturnsError(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	cfg.Version = CurrentVersion + 5

	err := ValidateStructure(cfg)
	if err == nil {
		t.Fatal("expected error for future version")
	}
	if !strings.Contains(err.Error(), "newer than this binary supports") {
		t.Errorf("expected version error message, got: %v", err)
	}
}

func TestDefaults_VersionMatchesCurrentVersion(t *testing.T) {
	t.Parallel()
	cfg := Defaults()
	if cfg.Version != CurrentVersion {
		t.Errorf("Defaults().Version = %d, want CurrentVersion = %d", cfg.Version, CurrentVersion)
	}
}
