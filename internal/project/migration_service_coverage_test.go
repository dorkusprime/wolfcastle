package project

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// ── MigrateDirectoryLayout error paths ─────────────────────────────

func TestMigrateDirectoryLayout_MkdirAllFailsWhenOldLayoutExists(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod semantics differ on Windows")
	}
	t.Parallel()
	svc, root := newMigrationService(t)

	// Create old layout so the function takes the migration path.
	if err := os.MkdirAll(filepath.Join(root, "base"), 0755); err != nil {
		t.Fatal(err)
	}

	// Make root read-only so os.MkdirAll("system/") fails.
	if err := os.Chmod(root, 0555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(root, 0755) }()

	err := svc.MigrateDirectoryLayout()
	if err == nil {
		t.Fatal("expected error when MkdirAll fails for system/")
	}
	if !strings.Contains(err.Error(), "creating system/") {
		t.Errorf("unexpected error: %v", err)
	}
}

// NOTE: os.Rename error paths (lines 46-48, 58-60) are structurally
// untestable without concurrent filesystem manipulation. The function
// creates system/ via MkdirAll and immediately renames into it; we
// cannot inject a blocker between those two calls.

func TestMigrateDirectoryLayout_MkdirAllFailsFreshInstall(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod semantics differ on Windows")
	}
	t.Parallel()

	// When there's no old layout AND MkdirAll fails (line 32).
	svc, root := newMigrationService(t)

	// Make root read-only so MkdirAll for system/ fails on the fresh
	// install path (no base/ exists).
	if err := os.Chmod(root, 0555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(root, 0755) }()

	err := svc.MigrateDirectoryLayout()
	if err == nil {
		t.Fatal("expected error when MkdirAll fails on fresh install path")
	}
	// This error comes from os.MkdirAll directly, not wrapped.
	if !strings.Contains(err.Error(), "permission denied") {
		t.Errorf("expected permission denied, got: %v", err)
	}
}

// ── MigrateOldConfig error paths ───────────────────────────────────

func TestMigrateOldConfig_ReadErrorOnOldConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod semantics differ on Windows")
	}
	t.Parallel()
	svc, root := newMigrationService(t)

	// Create config.json but make it unreadable.
	oldCfgPath := filepath.Join(root, "config.json")
	if err := os.WriteFile(oldCfgPath, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Ensure system/custom exists so the code reaches ReadFile.
	if err := os.MkdirAll(filepath.Join(root, "system", "custom"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(oldCfgPath, 0000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(oldCfgPath, 0644) }()

	err := svc.MigrateOldConfig()
	if err == nil {
		t.Fatal("expected error when old config.json is unreadable")
	}
	if !strings.Contains(err.Error(), "reading old config.json") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMigrateOldConfig_WriteErrorForCustomConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod semantics differ on Windows")
	}
	t.Parallel()
	svc, root := newMigrationService(t)

	// Create old config.json with valid content.
	if err := os.WriteFile(filepath.Join(root, "config.json"), []byte(`{"a":1}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Create system/custom/ but make it read-only so WriteFile fails.
	customDir := filepath.Join(root, "system", "custom")
	if err := os.MkdirAll(customDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(customDir, 0555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(customDir, 0755) }()

	err := svc.MigrateOldConfig()
	if err == nil {
		t.Fatal("expected error when writing system/custom/config.json fails")
	}
	if !strings.Contains(err.Error(), "writing system/custom/config.json") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMigrateOldConfig_ReadErrorOnOldLocalConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod semantics differ on Windows")
	}
	t.Parallel()
	svc, root := newMigrationService(t)

	// Create config.local.json but make it unreadable.
	oldLocalPath := filepath.Join(root, "config.local.json")
	if err := os.WriteFile(oldLocalPath, []byte(`{}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(oldLocalPath, 0000); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(oldLocalPath, 0644) }()

	err := svc.MigrateOldConfig()
	if err == nil {
		t.Fatal("expected error when old config.local.json is unreadable")
	}
	if !strings.Contains(err.Error(), "reading old config.local.json") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMigrateOldConfig_MalformedLocalJSON(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// Write malformed JSON to config.local.json.
	oldLocalPath := filepath.Join(root, "config.local.json")
	if err := os.WriteFile(oldLocalPath, []byte(`{not json`), 0644); err != nil {
		t.Fatal(err)
	}

	err := svc.MigrateOldConfig()
	if err == nil {
		t.Fatal("expected error on malformed config.local.json")
	}
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMigrateOldConfig_WriteErrorForLocalConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod semantics differ on Windows")
	}
	t.Parallel()
	svc, root := newMigrationService(t)

	// Write valid config.local.json.
	if err := os.WriteFile(filepath.Join(root, "config.local.json"), []byte(`{"key":"val"}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Create system/local/ but make it read-only so WriteFile fails.
	localDir := filepath.Join(root, "system", "local")
	if err := os.MkdirAll(localDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(localDir, 0555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(localDir, 0755) }()

	err := svc.MigrateOldConfig()
	if err == nil {
		t.Fatal("expected error when writing system/local/config.json fails")
	}
	if !strings.Contains(err.Error(), "writing system/local/config.json") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ── MigrateOldConfig: MkdirAll for custom/local dirs ───────────────

func TestMigrateOldConfig_CreatesCustomDirWhenMissing(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// Ensure system/ exists but NOT system/custom/.
	if err := os.MkdirAll(filepath.Join(root, "system"), 0755); err != nil {
		t.Fatal(err)
	}

	// Write old config.json.
	if err := os.WriteFile(filepath.Join(root, "config.json"), []byte(`{"x":1}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := svc.MigrateOldConfig(); err != nil {
		t.Fatal(err)
	}

	// custom/config.json should have been created.
	data, err := os.ReadFile(filepath.Join(root, "system", "custom", "config.json"))
	if err != nil {
		t.Fatal("expected system/custom/config.json to be created:", err)
	}
	if string(data) != `{"x":1}` {
		t.Errorf("got %s, want {\"x\":1}", data)
	}
}

func TestMigrateOldConfig_CreatesLocalDirWhenMissing(t *testing.T) {
	t.Parallel()
	svc, root := newMigrationService(t)

	// Ensure system/ exists but NOT system/local/.
	if err := os.MkdirAll(filepath.Join(root, "system"), 0755); err != nil {
		t.Fatal(err)
	}

	// Write valid config.local.json.
	if err := os.WriteFile(filepath.Join(root, "config.local.json"), []byte(`{"y":2}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := svc.MigrateOldConfig(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, "system", "local", "config.json"))
	if err != nil {
		t.Fatal("expected system/local/config.json to be created:", err)
	}
	if !strings.Contains(string(data), `"y"`) {
		t.Errorf("merged config should contain key y: %s", data)
	}
}

// NOTE: json.MarshalIndent error (line 112-114) is structurally unreachable.
// DeepMerge returns map[string]any produced by json.Unmarshal, which only
// contains JSON-safe types (strings, float64, bool, nil, nested maps/slices).
// MarshalIndent cannot fail on such input.
