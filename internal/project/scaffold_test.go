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

func TestScaffold_CreatesConfigJSON(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")

	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal("config.json not created:", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal("config.json is not valid JSON:", err)
	}
}

func TestScaffold_CreatesConfigLocalJSON(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")

	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "config.local.json"))
	if err != nil {
		t.Fatal("config.local.json not created:", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal("config.local.json is not valid JSON:", err)
	}

	identity, ok := cfg["identity"].(map[string]any)
	if !ok {
		t.Fatal("expected identity object in config.local.json")
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

func TestWriteBasePrompts_CreatesPromptFiles(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")
	os.MkdirAll(filepath.Join(dir, "base", "prompts"), 0755)
	os.MkdirAll(filepath.Join(dir, "base", "rules"), 0755)
	os.MkdirAll(filepath.Join(dir, "base", "audits"), 0755)

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
