package project

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/tierfs"
)

// stubPromptWriter records calls to WriteAllBase and optionally returns an error.
type stubPromptWriter struct {
	called    bool
	templates fs.FS
	err       error
}

func (s *stubPromptWriter) WriteAllBase(templates fs.FS) error {
	s.called = true
	s.templates = templates
	return s.err
}

func newScaffoldService(t *testing.T) (*ScaffoldService, *stubPromptWriter, string) {
	t.Helper()
	root := filepath.Join(t.TempDir(), ".wolfcastle")
	if err := os.MkdirAll(root, 0755); err != nil {
		t.Fatal(err)
	}
	tiers := tierfs.New(filepath.Join(root, "system"))
	cfg := config.NewConfigRepositoryWithTiers(tiers, root)
	pw := &stubPromptWriter{}
	svc := NewScaffoldService(cfg, pw, nil, root)
	return svc, pw, root
}

func testIdentity() *config.Identity {
	return &config.Identity{
		User:      "testuser",
		Machine:   "testbox",
		Namespace: "testuser-testbox",
	}
}

// --- Init ---

func TestScaffoldService_Init_CreatesDirectoryStructure(t *testing.T) {
	t.Parallel()
	svc, _, root := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}

	expectedDirs := []string{
		"system/base/prompts",
		"system/base/prompts/stages",
		"system/base/prompts/classes",
		"system/base/prompts/audits",
		"system/base/rules",
		"system/base/audits",
		"system/custom",
		"system/local",
		"system/projects",
		"system/logs",
		"archive",
		"artifacts",
		"docs/decisions",
		"docs/specs",
	}
	for _, d := range expectedDirs {
		info, err := os.Stat(filepath.Join(root, d))
		if err != nil {
			t.Errorf("expected directory %q: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%q should be a directory", d)
		}
	}
}

func TestScaffoldService_Init_WritesConfigFiles(t *testing.T) {
	t.Parallel()
	svc, _, root := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}

	// Base config: should have defaults, no identity.
	baseData, err := os.ReadFile(filepath.Join(root, "system", "base", "config.json"))
	if err != nil {
		t.Fatal("base/config.json:", err)
	}
	var baseCfg map[string]any
	if err := json.Unmarshal(baseData, &baseCfg); err != nil {
		t.Fatal("base/config.json not valid JSON:", err)
	}
	if _, ok := baseCfg["models"]; !ok {
		t.Error("base config should contain models")
	}
	if _, ok := baseCfg["identity"]; ok {
		t.Error("base config should NOT contain identity")
	}

	// Custom config: should be empty object.
	customData, err := os.ReadFile(filepath.Join(root, "system", "custom", "config.json"))
	if err != nil {
		t.Fatal("custom/config.json:", err)
	}
	var customCfg map[string]any
	if err := json.Unmarshal(customData, &customCfg); err != nil {
		t.Fatal("custom/config.json not valid JSON:", err)
	}
	if len(customCfg) != 0 {
		t.Error("custom/config.json should be empty object")
	}

	// Local config: should contain identity.
	localData, err := os.ReadFile(filepath.Join(root, "system", "local", "config.json"))
	if err != nil {
		t.Fatal("local/config.json:", err)
	}
	var localCfg map[string]any
	if err := json.Unmarshal(localData, &localCfg); err != nil {
		t.Fatal("local/config.json not valid JSON:", err)
	}
	identity, ok := localCfg["identity"].(map[string]any)
	if !ok {
		t.Fatal("local config should contain identity object")
	}
	if identity["user"] != "testuser" {
		t.Errorf("identity.user: got %v, want testuser", identity["user"])
	}
	if identity["machine"] != "testbox" {
		t.Errorf("identity.machine: got %v, want testbox", identity["machine"])
	}
}

func TestScaffoldService_Init_CreatesNamespaceProjectDir(t *testing.T) {
	t.Parallel()
	svc, _, root := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}

	nsDir := filepath.Join(root, "system", "projects", "testuser-testbox")
	info, err := os.Stat(nsDir)
	if err != nil {
		t.Fatal("namespace project directory should exist:", err)
	}
	if !info.IsDir() {
		t.Error("namespace project directory should be a directory")
	}

	// Should contain a state.json root index.
	stateData, err := os.ReadFile(filepath.Join(nsDir, "state.json"))
	if err != nil {
		t.Fatal("state.json should exist in namespace directory:", err)
	}
	var idx map[string]any
	if err := json.Unmarshal(stateData, &idx); err != nil {
		t.Fatal("state.json not valid JSON:", err)
	}
	if idx["version"] != float64(1) {
		t.Errorf("state.json version: got %v, want 1", idx["version"])
	}
}

func TestScaffoldService_Init_ExtractsEmbeddedPrompts(t *testing.T) {
	t.Parallel()
	svc, pw, _ := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}

	if !pw.called {
		t.Error("Init should call WriteAllBase on the prompt writer")
	}
}

func TestScaffoldService_Init_WritesGitignore(t *testing.T) {
	t.Parallel()
	svc, _, root := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, ".gitignore"))
	if err != nil {
		t.Fatal(".gitignore should exist:", err)
	}
	content := string(data)
	for _, want := range []string{"system/base/", "system/local/", "system/logs/"} {
		if !strings.Contains(content, want) {
			t.Errorf(".gitignore should contain %q", want)
		}
	}
}

// --- Reinit ---

func TestScaffoldService_Reinit_RegeneratesBaseTier(t *testing.T) {
	t.Parallel()
	svc, pw, root := newScaffoldService(t)

	// Set up a scaffolded directory first.
	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}
	pw.called = false

	// Corrupt the base config so we can verify it gets overwritten.
	baseCfgPath := filepath.Join(root, "system", "base", "config.json")
	if err := os.WriteFile(baseCfgPath, []byte(`{"stale": true}`), 0644); err != nil {
		t.Fatal(err)
	}

	if err := svc.Reinit(); err != nil {
		t.Fatal(err)
	}

	// Base config should have fresh defaults.
	data, _ := os.ReadFile(baseCfgPath)
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg["stale"]; ok {
		t.Error("Reinit should overwrite base config with fresh defaults")
	}
	if _, ok := cfg["models"]; !ok {
		t.Error("Reinit should write full defaults to base config")
	}

	// Prompts should be re-extracted.
	if !pw.called {
		t.Error("Reinit should call WriteAllBase")
	}
}

func TestScaffoldService_Reinit_PreservesCustomAndLocalConfig(t *testing.T) {
	t.Parallel()
	svc, _, root := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}

	// Write custom overrides.
	customPath := filepath.Join(root, "system", "custom", "config.json")
	customContent := `{"team_setting": "keep_me"}`
	if err := os.WriteFile(customPath, []byte(customContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Write extra keys to local config.
	localPath := filepath.Join(root, "system", "local", "config.json")
	localCfg := map[string]any{
		"identity":  map[string]any{"user": "old", "machine": "old"},
		"extra_key": "preserved",
	}
	writeTestJSON(t, localPath, localCfg)

	if err := svc.Reinit(); err != nil {
		t.Fatal(err)
	}

	// Custom config should be untouched.
	data, err := os.ReadFile(customPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != customContent {
		t.Error("Reinit should preserve custom/config.json")
	}

	// Local config should preserve extra_key, though identity may be refreshed.
	localData, err := os.ReadFile(localPath)
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(localData, &result); err != nil {
		t.Fatal(err)
	}
	if result["extra_key"] != "preserved" {
		t.Error("Reinit should preserve extra keys in local/config.json")
	}
	if _, ok := result["identity"]; !ok {
		t.Error("Reinit should maintain identity in local/config.json")
	}
}

func TestScaffoldService_Reinit_CallsMigrationsBeforeRegeneration(t *testing.T) {
	t.Parallel()
	svc, _, root := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}

	// Simulate old root config.json that migration should pick up.
	oldCfg := `{"failure": {"hard_cap": 999}}`
	if err := os.WriteFile(filepath.Join(root, "config.json"), []byte(oldCfg), 0644); err != nil {
		t.Fatal(err)
	}

	// Remove custom config so the migrated file is the only source.
	_ = os.Remove(filepath.Join(root, "system", "custom", "config.json"))

	if err := svc.Reinit(); err != nil {
		t.Fatal(err)
	}

	// Old file should be gone (migration ran).
	if _, err := os.Stat(filepath.Join(root, "config.json")); !os.IsNotExist(err) {
		t.Error("Reinit should run MigrateOldConfig, removing root config.json")
	}

	// custom/config.json should exist (either from migration or Reinit's ensure step).
	if _, err := os.Stat(filepath.Join(root, "system", "custom", "config.json")); err != nil {
		t.Error("custom/config.json should exist after Reinit:", err)
	}
}

func TestScaffoldService_Reinit_CreatesCustomConfigWhenMissing(t *testing.T) {
	t.Parallel()
	svc, _, root := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}

	// Remove custom config.
	_ = os.Remove(filepath.Join(root, "system", "custom", "config.json"))

	if err := svc.Reinit(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(root, "system", "custom", "config.json"))
	if err != nil {
		t.Fatal("Reinit should create custom/config.json when missing:", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg) != 0 {
		t.Error("newly created custom/config.json should be empty object")
	}
}

func TestScaffoldService_Reinit_HandlesMissingLocalConfig(t *testing.T) {
	t.Parallel()
	svc, _, root := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}

	_ = os.Remove(filepath.Join(root, "system", "local", "config.json"))

	if err := svc.Reinit(); err != nil {
		t.Fatal("Reinit should handle missing local/config.json:", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "system", "local", "config.json"))
	if err != nil {
		t.Fatal("Reinit should create local/config.json:", err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg["identity"]; !ok {
		t.Error("created local/config.json should contain identity")
	}
}

// --- Scaffold file snapshot tests ---

// TestScaffoldService_Init_SnapshotScaffoldFiles verifies that Init produces
// byte-for-byte identical output to the embedded templates for every scaffold
// file: the .gitignore and all five READMEs.
func TestScaffoldService_Init_SnapshotScaffoldFiles(t *testing.T) {
	t.Parallel()
	svc, _, root := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}

	for tmpl, dest := range scaffoldFiles {
		t.Run(dest, func(t *testing.T) {
			want, err := Templates.ReadFile(tmpl)
			if err != nil {
				t.Fatalf("reading embedded template %s: %v", tmpl, err)
			}
			got, err := os.ReadFile(filepath.Join(root, dest))
			if err != nil {
				t.Fatalf("scaffold file %s missing after Init: %v", dest, err)
			}
			if string(got) != string(want) {
				t.Errorf("snapshot mismatch for %s:\ngot %d bytes, want %d bytes\ngot:\n%s\nwant:\n%s",
					dest, len(got), len(want), string(got), string(want))
			}
		})
	}
}

// TestScaffoldService_Reinit_SnapshotScaffoldFiles verifies that Reinit
// preserves scaffold files (.gitignore and READMEs) byte-for-byte. Files
// outside the base tier survive the teardown-rebuild cycle. The base-prompts
// README lives inside system/base/ and is destroyed by RemoveAll during
// Reinit; that case is tested separately below.
func TestScaffoldService_Reinit_SnapshotScaffoldFiles(t *testing.T) {
	t.Parallel()
	svc, _, root := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}
	if err := svc.Reinit(); err != nil {
		t.Fatal(err)
	}

	// The base-prompts README sits inside system/base/ which Reinit wipes.
	const baseTierTemplate = "templates/scaffold/readme-base-prompts.md.tmpl"

	for tmpl, dest := range scaffoldFiles {
		if tmpl == baseTierTemplate {
			continue
		}
		t.Run(dest, func(t *testing.T) {
			want, err := Templates.ReadFile(tmpl)
			if err != nil {
				t.Fatalf("reading embedded template %s: %v", tmpl, err)
			}
			got, err := os.ReadFile(filepath.Join(root, dest))
			if err != nil {
				t.Fatalf("scaffold file %s missing after Reinit: %v", dest, err)
			}
			if string(got) != string(want) {
				t.Errorf("snapshot mismatch for %s after Reinit:\ngot %d bytes, want %d bytes\ngot:\n%s\nwant:\n%s",
					dest, len(got), len(want), string(got), string(want))
			}
		})
	}
}

// TestScaffoldService_Reinit_BaseTierREADMELost verifies that the
// system/base/prompts/README.md scaffold file does not survive Reinit,
// because Reinit tears down and rebuilds the entire base tier directory.
func TestScaffoldService_Reinit_BaseTierREADMELost(t *testing.T) {
	t.Parallel()
	svc, _, root := newScaffoldService(t)

	if err := svc.Init(testIdentity()); err != nil {
		t.Fatal(err)
	}

	readmePath := filepath.Join(root, "system", "base", "prompts", "README.md")
	if _, err := os.Stat(readmePath); err != nil {
		t.Fatal("README should exist after Init:", err)
	}

	if err := svc.Reinit(); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(readmePath); !os.IsNotExist(err) {
		t.Error("system/base/prompts/README.md should not survive Reinit (base tier is wiped)")
	}
}
