package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/tierfs"
)

func newMigrationService(t *testing.T) (*MigrationService, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), ".wolfcastle")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	tiers := tierfs.New(filepath.Join(root, "system"))
	cfg := config.NewConfigRepositoryWithTiers(tiers, root)
	return &MigrationService{config: cfg, root: root}, root
}

// --- MigrateDirectoryLayout ---

func TestMigrateDirectoryLayout_MovesFlatDirectories(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// Create old flat layout at root level.
	for _, d := range []string{"base", "custom", "local", "projects", "logs"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0755); err != nil {
			t.Fatal(err)
		}
		// Drop a marker file so we can verify the move.
		if err := os.WriteFile(filepath.Join(root, d, "marker.txt"), []byte(d), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if err := svc.MigrateDirectoryLayout(); err != nil {
		t.Fatal(err)
	}

	// Each directory should now live under system/.
	for _, d := range []string{"base", "custom", "local", "projects", "logs"} {
		marker := filepath.Join(root, "system", d, "marker.txt")
		data, err := os.ReadFile(marker)
		if err != nil {
			t.Errorf("expected system/%s/marker.txt to exist: %v", d, err)
			continue
		}
		if string(data) != d {
			t.Errorf("system/%s/marker.txt: got %q, want %q", d, data, d)
		}

		// Original should be gone.
		if _, err := os.Stat(filepath.Join(root, d)); !os.IsNotExist(err) {
			t.Errorf("old directory %s should have been moved", d)
		}
	}
}

func TestMigrateDirectoryLayout_MovesFiles(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// Need base/ to trigger migration (otherwise treated as fresh install).
	if err := os.MkdirAll(filepath.Join(root, "base"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"wolfcastle.pid", "stop", "daemon.log", "daemon.meta.json"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte(f), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if err := svc.MigrateDirectoryLayout(); err != nil {
		t.Fatal(err)
	}

	for _, f := range []string{"wolfcastle.pid", "stop", "daemon.log", "daemon.meta.json"} {
		data, err := os.ReadFile(filepath.Join(root, "system", f))
		if err != nil {
			t.Errorf("expected system/%s to exist: %v", f, err)
			continue
		}
		if string(data) != f {
			t.Errorf("system/%s: got %q, want %q", f, data, f)
		}
	}
}

func TestMigrateDirectoryLayout_IdempotentWhenSystemExists(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// Create system/ so migration sees it and returns early.
	if err := os.MkdirAll(filepath.Join(root, "system"), 0755); err != nil {
		t.Fatal(err)
	}

	// Also create a flat directory that should NOT be moved.
	if err := os.MkdirAll(filepath.Join(root, "base"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "base", "sentinel.txt"), []byte("stay"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := svc.MigrateDirectoryLayout(); err != nil {
		t.Fatal(err)
	}

	// base/ at root should still be there; migration was a no-op.
	if _, err := os.Stat(filepath.Join(root, "base", "sentinel.txt")); err != nil {
		t.Error("base/sentinel.txt should still exist when system/ already present")
	}
}

func TestMigrateDirectoryLayout_FreshInstallation(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// No old layout, no system/ directory. Migration should just create system/.
	if err := svc.MigrateDirectoryLayout(); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(filepath.Join(root, "system"))
	if err != nil {
		t.Fatal("system/ should be created on fresh install:", err)
	}
	if !info.IsDir() {
		t.Error("system/ should be a directory")
	}
}

// --- MigrateOldConfig ---

func TestMigrateOldConfig_MovesRootConfigToCustomTier(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// Ensure system/custom exists.
	if err := os.MkdirAll(filepath.Join(root, "system", "custom"), 0755); err != nil {
		t.Fatal(err)
	}

	oldCfg := `{"failure": {"hard_cap": 42}}`
	if err := os.WriteFile(filepath.Join(root, "config.json"), []byte(oldCfg), 0644); err != nil {
		t.Fatal(err)
	}

	if err := svc.MigrateOldConfig(); err != nil {
		t.Fatal(err)
	}

	// custom/config.json should contain the old content.
	data, err := os.ReadFile(filepath.Join(root, "system", "custom", "config.json"))
	if err != nil {
		t.Fatal("custom/config.json should exist:", err)
	}
	if string(data) != oldCfg {
		t.Errorf("custom/config.json: got %s, want %s", data, oldCfg)
	}

	// Old file should be removed.
	if _, err := os.Stat(filepath.Join(root, "config.json")); !os.IsNotExist(err) {
		t.Error("root config.json should be removed after migration")
	}
}

func TestMigrateOldConfig_SkipsMoveWhenCustomConfigExists(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	customDir := filepath.Join(root, "system", "custom")
	if err := os.MkdirAll(customDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Pre-existing custom config.
	existing := `{"existing": true}`
	if err := os.WriteFile(filepath.Join(customDir, "config.json"), []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	// Old root config that should NOT overwrite the existing custom one.
	if err := os.WriteFile(filepath.Join(root, "config.json"), []byte(`{"old": true}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := svc.MigrateOldConfig(); err != nil {
		t.Fatal(err)
	}

	// Custom config should be unchanged.
	data, err := os.ReadFile(filepath.Join(customDir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != existing {
		t.Error("custom/config.json should not be overwritten when it already exists")
	}

	// Old file still gets removed.
	if _, err := os.Stat(filepath.Join(root, "config.json")); !os.IsNotExist(err) {
		t.Error("root config.json should still be removed")
	}
}

func TestMigrateOldConfig_MergesLocalConfig(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	localDir := filepath.Join(root, "system", "local")
	if err := os.MkdirAll(localDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Existing local config with some keys.
	existingLocal := map[string]any{"keep_me": "yes"}
	writeTestJSON(t, filepath.Join(localDir, "config.json"), existingLocal)

	// Old config.local.json with keys to merge in.
	oldLocal := map[string]any{
		"identity": map[string]any{"user": "alice", "machine": "box"},
		"extra":    "value",
	}
	writeTestJSON(t, filepath.Join(root, "config.local.json"), oldLocal)

	if err := svc.MigrateOldConfig(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(localDir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}

	// Both old and existing keys should be present.
	if result["keep_me"] != "yes" {
		t.Error("existing local keys should be preserved")
	}
	if result["extra"] != "value" {
		t.Error("merged keys from config.local.json should be present")
	}
	identity, _ := result["identity"].(map[string]any)
	if identity["user"] != "alice" {
		t.Error("identity from config.local.json should be merged")
	}

	// Old file should be removed.
	if _, err := os.Stat(filepath.Join(root, "config.local.json")); !os.IsNotExist(err) {
		t.Error("config.local.json should be removed after migration")
	}
}

func TestMigrateOldConfig_HandlesMissingSources(t *testing.T) {
	t.Parallel()
	svc, _ := newMigrationService(t)

	// No config.json or config.local.json exist. Should succeed silently.
	if err := svc.MigrateOldConfig(); err != nil {
		t.Fatal("MigrateOldConfig should handle missing source files gracefully:", err)
	}
}

func writeTestJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
}
