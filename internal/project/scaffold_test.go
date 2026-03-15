package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestScaffold_CreatesAllRequiredDirectories(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")

	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	expectedDirs := []string{
		"base/prompts",
		"base/rules",
		"base/audits",
		"custom",
		"local",
		"archive",
		"docs/decisions",
		"docs/specs",
		"logs",
	}
	for _, d := range expectedDirs {
		path := filepath.Join(dir, d)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected directory %q to exist: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %q to be a directory", d)
		}
	}
}

func TestScaffold_CreatesBaseConfigJSON(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")

	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "base", "config.json"))
	if err != nil {
		t.Fatal("base/config.json not created:", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal("base/config.json is not valid JSON:", err)
	}

	// Verify populated defaults (not empty {})
	models, ok := cfg["models"].(map[string]any)
	if !ok || len(models) == 0 {
		t.Error("base/config.json should contain default models")
	}
	pipeline, ok := cfg["pipeline"].(map[string]any)
	if !ok {
		t.Error("base/config.json should contain pipeline config")
	} else if stages, ok := pipeline["stages"].([]any); !ok || len(stages) == 0 {
		t.Error("base/config.json should contain default pipeline stages")
	}
	if _, ok := cfg["identity"]; ok {
		t.Error("base/config.json should NOT contain identity (belongs in local/config.json)")
	}
}

func TestScaffold_CreatesCustomConfigJSON(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")

	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "custom", "config.json"))
	if err != nil {
		t.Fatal("custom/config.json not created:", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal("custom/config.json is not valid JSON:", err)
	}

	if len(cfg) != 0 {
		t.Error("custom/config.json should be empty object")
	}
}

func TestScaffold_CreatesLocalConfigJSON(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")

	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "local", "config.json"))
	if err != nil {
		t.Fatal("local/config.json not created:", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal("local/config.json is not valid JSON:", err)
	}

	identity, ok := cfg["identity"].(map[string]any)
	if !ok {
		t.Fatal("expected identity object in local/config.json")
	}
	if _, ok := identity["user"]; !ok {
		t.Error("expected identity.user")
	}
	if _, ok := identity["machine"]; !ok {
		t.Error("expected identity.machine")
	}
}

func TestScaffold_CreatesRootIndex(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")

	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Find the projects directory (it's under projects/<user>-<machine>/)
	entries, err := os.ReadDir(filepath.Join(dir, "projects"))
	if err != nil {
		t.Fatal("projects directory not created:", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 namespace directory, got %d", len(entries))
	}

	stateFile := filepath.Join(dir, "projects", entries[0].Name(), "state.json")
	data, err := os.ReadFile(stateFile)
	if err != nil {
		t.Fatal("state.json not created:", err)
	}

	var idx map[string]any
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatal("state.json is not valid JSON:", err)
	}
	if idx["version"] != float64(1) {
		t.Errorf("expected version=1, got %v", idx["version"])
	}
}

func TestScaffold_CreatesGitignore(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")

	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(".gitignore not created:", err)
	}

	content := string(data)
	for _, expected := range []string{"!custom/", "!projects/", "!archive/", "!docs/"} {
		if !contains(content, expected) {
			t.Errorf(".gitignore should contain %q", expected)
		}
	}
}

func TestReScaffold_RegeneratesBase(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")

	// Initial scaffold
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Remove a base file to verify it gets regenerated
	promptFile := filepath.Join(dir, "base", "prompts", "execute.md")
	if err := os.Remove(promptFile); err != nil {
		t.Fatal("removing prompt file:", err)
	}

	// ReScaffold should regenerate it
	if err := ReScaffold(dir); err != nil {
		t.Fatal("ReScaffold failed:", err)
	}

	if _, err := os.Stat(promptFile); err != nil {
		t.Error("ReScaffold should regenerate missing base files:", err)
	}
}

func TestReScaffold_PreservesCustomConfigJSON(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")

	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Modify custom/config.json
	cfgPath := filepath.Join(dir, "custom", "config.json")
	custom := []byte(`{"custom_key": "custom_value"}`)
	if err := os.WriteFile(cfgPath, custom, 0644); err != nil {
		t.Fatal(err)
	}

	// ReScaffold should not overwrite custom/config.json
	if err := ReScaffold(dir); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(cfgPath)
	if string(data) != string(custom) {
		t.Error("ReScaffold should preserve custom/config.json content")
	}
}

func TestReScaffold_OverwritesBaseConfigJSON(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")

	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Modify base/config.json
	cfgPath := filepath.Join(dir, "base", "config.json")
	if err := os.WriteFile(cfgPath, []byte(`{"stale": true}`), 0644); err != nil {
		t.Fatal(err)
	}

	// ReScaffold should overwrite base/config.json with fresh defaults
	if err := ReScaffold(dir); err != nil {
		t.Fatal(err)
	}

	data, _ := os.ReadFile(cfgPath)
	var cfg map[string]any
	_ = json.Unmarshal(data, &cfg)
	if _, ok := cfg["stale"]; ok {
		t.Error("ReScaffold should overwrite base/config.json with fresh defaults")
	}
	if _, ok := cfg["models"]; !ok {
		t.Error("ReScaffold should write full defaults to base/config.json")
	}
}

func TestReScaffold_RefreshesIdentity(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")

	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Write local/config.json with extra keys
	localPath := filepath.Join(dir, "local", "config.json")
	localCfg := map[string]any{
		"identity":  map[string]any{"user": "old-user", "machine": "old-machine"},
		"extra_key": "should_be_preserved",
	}
	data, _ := json.MarshalIndent(localCfg, "", "  ")
	_ = os.WriteFile(localPath, data, 0644)

	// ReScaffold should refresh identity but preserve extra keys
	if err := ReScaffold(dir); err != nil {
		t.Fatal(err)
	}

	newData, _ := os.ReadFile(localPath)
	var result map[string]any
	_ = json.Unmarshal(newData, &result)

	if _, ok := result["extra_key"]; !ok {
		t.Error("ReScaffold should preserve extra keys in local/config.json")
	}
	identity, _ := result["identity"].(map[string]any)
	if _, ok := identity["user"]; !ok {
		t.Error("ReScaffold should maintain identity.user")
	}
}

func TestReScaffold_HandlesCorruptLocalConfig(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")

	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Write invalid JSON to local/config.json
	localPath := filepath.Join(dir, "local", "config.json")
	_ = os.WriteFile(localPath, []byte("not json"), 0644)

	err := ReScaffold(dir)
	if err == nil {
		t.Error("ReScaffold should return an error for corrupt local/config.json")
	}
}

func TestReScaffold_HandlesMissingLocalConfig(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")

	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Remove local/config.json
	_ = os.Remove(filepath.Join(dir, "local", "config.json"))

	// ReScaffold should create it
	if err := ReScaffold(dir); err != nil {
		t.Fatal("ReScaffold should handle missing local/config.json:", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "local", "config.json"))
	if err != nil {
		t.Fatal("ReScaffold should create local/config.json:", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal("created local/config.json is not valid JSON:", err)
	}
	if _, ok := cfg["identity"]; !ok {
		t.Error("created local/config.json should contain identity")
	}
}

func TestReScaffold_MigratesOldConfigJSON(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")

	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Simulate old-style config: write root config.json and config.local.json
	oldCfg := `{"failure": {"hard_cap": 999}}`
	_ = os.WriteFile(filepath.Join(dir, "config.json"), []byte(oldCfg), 0644)

	oldLocal := `{"identity": {"user": "migrated", "machine": "host"}, "extra": "kept"}`
	_ = os.WriteFile(filepath.Join(dir, "config.local.json"), []byte(oldLocal), 0644)

	// Remove the three-tier files so migration is the only source
	_ = os.Remove(filepath.Join(dir, "custom", "config.json"))
	_ = os.Remove(filepath.Join(dir, "local", "config.json"))

	if err := ReScaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Old files should be gone
	if _, err := os.Stat(filepath.Join(dir, "config.json")); !os.IsNotExist(err) {
		t.Error("old config.json should be removed after migration")
	}
	if _, err := os.Stat(filepath.Join(dir, "config.local.json")); !os.IsNotExist(err) {
		t.Error("old config.local.json should be removed after migration")
	}

	// custom/config.json should contain the migrated content
	data, err := os.ReadFile(filepath.Join(dir, "custom", "config.json"))
	if err != nil {
		t.Fatal("custom/config.json should exist after migration:", err)
	}
	var customCfg map[string]any
	_ = json.Unmarshal(data, &customCfg)
	failure, _ := customCfg["failure"].(map[string]any)
	if failure["hard_cap"] != float64(999) {
		t.Error("custom/config.json should contain migrated hard_cap")
	}

	// local/config.json should contain identity from migration, plus refreshed identity from ReScaffold
	localData, err := os.ReadFile(filepath.Join(dir, "local", "config.json"))
	if err != nil {
		t.Fatal("local/config.json should exist after migration:", err)
	}
	var localCfg map[string]any
	_ = json.Unmarshal(localData, &localCfg)
	if _, ok := localCfg["identity"]; !ok {
		t.Error("local/config.json should contain identity after migration")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestWriteBasePrompts_CreatesPromptFiles(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")
	_ = os.MkdirAll(filepath.Join(dir, "base", "prompts"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, "base", "rules"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, "base", "audits"), 0755)

	if err := WriteBasePrompts(dir); err != nil {
		t.Fatal(err)
	}

	expectedFiles := []string{
		"base/prompts/execute.md",
		"base/prompts/expand.md",
		"base/prompts/file.md",
		"base/prompts/summary.md",
		"base/prompts/script-reference.md",
		"base/rules/git-conventions.md",
	}
	for _, f := range expectedFiles {
		path := filepath.Join(dir, f)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected file %q to exist: %v", f, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("expected file %q to have content", f)
		}
	}
}
