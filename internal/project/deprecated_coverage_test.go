package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── Deprecated migrateOldConfig error paths ───────────────────────
//
// These test the standalone deprecated migrateOldConfig function
// (scaffold.go:301), distinct from MigrationService.MigrateOldConfig
// which is covered in migration_service_coverage_test.go.

func TestDeprecatedMigrateOldConfig_NoOldConfigs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := migrateOldConfig(dir); err != nil {
		t.Errorf("expected no error when no old configs exist, got: %v", err)
	}
}

func TestDeprecatedMigrateOldConfig_ReadErrorOnOldConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "system", "custom"), 0755)

	oldCfg := filepath.Join(dir, "config.json")
	_ = os.WriteFile(oldCfg, []byte(`{"key":"value"}`), 0644)
	_ = os.Chmod(oldCfg, 0000)
	defer func() { _ = os.Chmod(oldCfg, 0644) }()

	err := migrateOldConfig(dir)
	if err == nil || !strings.Contains(err.Error(), "reading old config.json") {
		t.Errorf("expected 'reading old config.json' error, got: %v", err)
	}
}

func TestDeprecatedMigrateOldConfig_WriteErrorForCustomConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_ = os.WriteFile(filepath.Join(dir, "config.json"), []byte(`{"team":"alpha"}`), 0644)

	customDir := filepath.Join(dir, "system", "custom")
	_ = os.MkdirAll(customDir, 0755)
	_ = os.Chmod(customDir, 0555)
	defer func() { _ = os.Chmod(customDir, 0755) }()

	err := migrateOldConfig(dir)
	if err == nil || !strings.Contains(err.Error(), "writing system/custom/config.json") {
		t.Errorf("expected 'writing system/custom/config.json' error, got: %v", err)
	}
}

func TestDeprecatedMigrateOldConfig_ReadErrorOnOldLocalConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "system", "local"), 0755)

	oldLocal := filepath.Join(dir, "config.local.json")
	_ = os.WriteFile(oldLocal, []byte(`{"identity":{}}`), 0644)
	_ = os.Chmod(oldLocal, 0000)
	defer func() { _ = os.Chmod(oldLocal, 0644) }()

	err := migrateOldConfig(dir)
	if err == nil || !strings.Contains(err.Error(), "reading old config.local.json") {
		t.Errorf("expected 'reading old config.local.json' error, got: %v", err)
	}
}

func TestDeprecatedMigrateOldConfig_MalformedLocalJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	_ = os.MkdirAll(filepath.Join(dir, "system", "local"), 0755)

	_ = os.WriteFile(filepath.Join(dir, "config.local.json"), []byte(`{not json`), 0644)

	err := migrateOldConfig(dir)
	if err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("expected JSON parse error, got: %v", err)
	}
}

func TestDeprecatedMigrateOldConfig_MergeWithExistingLocalConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	localDir := filepath.Join(dir, "system", "local")
	_ = os.MkdirAll(localDir, 0755)

	_ = os.WriteFile(filepath.Join(localDir, "config.json"), []byte(`{"existing":"yes"}`), 0644)
	_ = os.WriteFile(filepath.Join(dir, "config.local.json"), []byte(`{"added":"field"}`), 0644)

	if err := migrateOldConfig(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(localDir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, `"existing"`) || !strings.Contains(content, `"added"`) {
		t.Errorf("expected merged config with both keys, got: %s", content)
	}
}

func TestDeprecatedMigrateOldConfig_WriteErrorForLocalConfig(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	localDir := filepath.Join(dir, "system", "local")
	_ = os.MkdirAll(localDir, 0755)

	_ = os.WriteFile(filepath.Join(dir, "config.local.json"), []byte(`{"key":"val"}`), 0644)

	_ = os.Chmod(localDir, 0555)
	defer func() { _ = os.Chmod(localDir, 0755) }()

	err := migrateOldConfig(dir)
	if err == nil || !strings.Contains(err.Error(), "writing system/local/config.json") {
		t.Errorf("expected 'writing system/local/config.json' error, got: %v", err)
	}
}

// ── Deprecated ReScaffold additional error paths ──────────────────

func TestReScaffold_MigrateOldConfigError(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".wolfcastle")
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Remove system/ so migrateToSystemLayout runs the fresh-install
	// path, then plant an unreadable config.json so migrateOldConfig fails.
	_ = os.RemoveAll(filepath.Join(dir, "system"))
	oldCfg := filepath.Join(dir, "config.json")
	_ = os.WriteFile(oldCfg, []byte(`{"x":1}`), 0644)
	_ = os.Chmod(oldCfg, 0000)
	defer func() { _ = os.Chmod(oldCfg, 0644) }()

	err := ReScaffold(dir)
	if err == nil || !strings.Contains(err.Error(), "reading old config.json") {
		t.Errorf("expected migrateOldConfig error to propagate, got: %v", err)
	}
}

// NOTE: ReScaffold lines 201-203 (WriteBasePrompts error) and 213-215
// (base config.json write error) are unreachable in unit tests.
// RemoveAll wipes system/base/ before MkdirAll recreates it with fresh
// writable directories. Any filesystem blocker placed before the call
// is removed by RemoveAll. Triggering these paths would require the
// filesystem to fail mid-execution (disk full, mount going read-only),
// same constraint as the os.Rename paths in migrateToSystemLayout.

func TestReScaffold_EnsureCustomConfigCreated(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".wolfcastle")
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Remove the custom config so the ensure-exists path fires.
	_ = os.Remove(filepath.Join(dir, "system", "custom", "config.json"))

	if err := ReScaffold(dir); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "system", "custom", "config.json")); err != nil {
		t.Errorf("custom config.json should have been recreated: %v", err)
	}
}

func TestReScaffold_InvalidLocalConfigJSON(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".wolfcastle")
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	localPath := filepath.Join(dir, "system", "local", "config.json")
	_ = os.WriteFile(localPath, []byte(`{not json}`), 0644)

	err := ReScaffold(dir)
	if err == nil || !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("expected JSON parse error, got: %v", err)
	}
}

// ── Scaffold: namespace dir creation failure ──────────────────────

func TestScaffold_NamespaceDirCreationFailurePrecise(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".wolfcastle")

	// Pre-create ALL directories from the dirs list (including
	// system/projects) so the MkdirAll loop passes entirely.
	allDirs := []string{
		"system/base/prompts", "system/base/rules", "system/base/audits",
		"system/custom", "system/local", "system/projects", "system/logs",
		"archive", "artifacts", "docs/decisions", "docs/specs",
	}
	for _, d := range allDirs {
		_ = os.MkdirAll(filepath.Join(dir, d), 0755)
	}

	// Place a file where the namespace directory needs to be created.
	// detectIdentity() gives us the same user-machine slug that Scaffold
	// will compute.
	identity := detectIdentity()
	ns := identity["user"].(string) + "-" + identity["machine"].(string)
	_ = os.WriteFile(filepath.Join(dir, "system", "projects", ns), []byte("block"), 0644)

	err := Scaffold(dir)
	if err == nil || !strings.Contains(err.Error(), "creating namespace directory") {
		t.Errorf("expected 'creating namespace directory' error, got: %v", err)
	}
}

// ── Scaffold: precise .gitignore write failure ────────────────────

func TestScaffold_GitignoreWriteFailurePrecise(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	dir := filepath.Join(tmp, ".wolfcastle")

	// Pre-create ALL directories from the Scaffold dirs list so the
	// MkdirAll loop passes entirely, then make root dir read-only.
	allDirs := []string{
		"system/base/prompts", "system/base/rules", "system/base/audits",
		"system/custom", "system/local", "system/projects", "system/logs",
		"archive", "artifacts", "docs/decisions", "docs/specs",
	}
	for _, d := range allDirs {
		_ = os.MkdirAll(filepath.Join(dir, d), 0755)
	}

	_ = os.Chmod(dir, 0555)
	defer func() { _ = os.Chmod(dir, 0755) }()

	err := Scaffold(dir)
	if err == nil || !strings.Contains(err.Error(), "writing .gitignore") {
		t.Errorf("expected '.gitignore' write error, got: %v", err)
	}
}
