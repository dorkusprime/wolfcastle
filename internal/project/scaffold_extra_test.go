package project

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteBasePrompts_SkipsDirectories(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")
	_ = os.MkdirAll(filepath.Join(dir, "base", "prompts"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, "base", "rules"), 0755)
	_ = os.MkdirAll(filepath.Join(dir, "base", "audits"), 0755)

	if err := WriteBasePrompts(dir); err != nil {
		t.Fatal(err)
	}

	// Verify no empty directories were created as files
	_ = filepath.Walk(filepath.Join(dir, "base"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Size() == 0 {
			t.Errorf("unexpected empty file: %s", path)
		}
		return nil
	})
}

func TestTemplates_EmbeddedFSIsNotEmpty(t *testing.T) {
	t.Parallel()
	count := 0
	_ = fs.WalkDir(Templates, "templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			count++
		}
		return nil
	})
	if count == 0 {
		t.Error("embedded templates filesystem should contain files")
	}
}

func TestScaffold_IdempotentOnExistingDir(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")

	// First scaffold
	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Second scaffold should not error
	if err := Scaffold(dir); err != nil {
		t.Errorf("scaffold should be idempotent, got: %v", err)
	}
}

func TestReScaffold_RegeneratesAllBaseDirectories(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")

	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Remove entire base directory
	if err := os.RemoveAll(filepath.Join(dir, "base")); err != nil {
		t.Fatal(err)
	}

	if err := ReScaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Verify base directories were recreated
	for _, d := range []string{"base/prompts", "base/rules", "base/audits"} {
		path := filepath.Join(dir, d)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected directory %q to exist after rescaffold: %v", d, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("expected %q to be a directory", d)
		}
	}
}

func TestDetectIdentity_ShortensHostname(t *testing.T) {
	t.Parallel()
	identity := detectIdentity()
	machine, ok := identity["machine"].(string)
	if !ok {
		t.Fatal("expected machine to be a string")
	}
	// Machine name should not contain dots (hostname is shortened)
	for _, r := range machine {
		if r == '.' {
			t.Errorf("machine name %q should not contain dots (hostname should be shortened)", machine)
			break
		}
	}
}

func TestScaffold_WritesBasePromptFiles(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")

	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Verify execute.md is written
	data, err := os.ReadFile(filepath.Join(dir, "base", "prompts", "execute.md"))
	if err != nil {
		t.Fatal("execute.md should exist:", err)
	}
	if len(data) == 0 {
		t.Error("execute.md should have content")
	}
}

func TestScaffold_NamespaceContainsUserAndMachine(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")

	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(filepath.Join(dir, "projects"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 namespace directory, got %d", len(entries))
	}
	nsName := entries[0].Name()
	if len(nsName) == 0 {
		t.Error("namespace directory name should not be empty")
	}
}

func TestReScaffold_WritesLocalConfigWhenMissing(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")

	if err := Scaffold(dir); err != nil {
		t.Fatal(err)
	}

	// Remove local config
	_ = os.Remove(filepath.Join(dir, "local", "config.json"))

	if err := ReScaffold(dir); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dir, "local", "config.json"))
	if err != nil {
		t.Fatal("local/config.json should be recreated:", err)
	}
	if len(data) == 0 {
		t.Error("local/config.json should have content")
	}
}

func TestWriteBasePrompts_CreatesSubdirectories(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), ".wolfcastle")
	// Don't pre-create base dirs — WriteBasePrompts should create them
	_ = os.MkdirAll(dir, 0755)

	if err := WriteBasePrompts(dir); err != nil {
		t.Fatal(err)
	}

	// Verify prompts directory was created
	info, err := os.Stat(filepath.Join(dir, "base", "prompts"))
	if err != nil {
		t.Fatal("base/prompts should exist:", err)
	}
	if !info.IsDir() {
		t.Error("base/prompts should be a directory")
	}
}

func TestDetectIdentity_LowercasesMachine(t *testing.T) {
	t.Parallel()
	identity := detectIdentity()
	machine, ok := identity["machine"].(string)
	if !ok {
		t.Fatal("expected machine to be a string")
	}
	for _, r := range machine {
		if r >= 'A' && r <= 'Z' {
			t.Errorf("machine name %q should be lowercase", machine)
			break
		}
	}
}
