package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveFragment_UnreadableFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a file in the base tier that can't be read
	baseDir := filepath.Join(dir, "base")
	_ = os.MkdirAll(baseDir, 0755)
	unreadable := filepath.Join(baseDir, "rules.md")
	_ = os.WriteFile(unreadable, []byte("content"), 0644)
	_ = os.Chmod(unreadable, 0000)
	defer func() { _ = os.Chmod(unreadable, 0644) }()

	_, err := ResolveFragment(dir, "rules.md")
	if err == nil {
		t.Error("expected error when file exists but is unreadable")
	}
}

func TestResolveAllFragments_WithIncludeAndExclude(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	baseDir := filepath.Join(dir, "base", "rules")
	_ = os.MkdirAll(baseDir, 0755)
	_ = os.WriteFile(filepath.Join(baseDir, "alpha.md"), []byte("alpha"), 0644)
	_ = os.WriteFile(filepath.Join(baseDir, "beta.md"), []byte("beta"), 0644)
	_ = os.WriteFile(filepath.Join(baseDir, "gamma.md"), []byte("gamma"), 0644)

	// Include only alpha and gamma, exclude gamma
	results, err := ResolveAllFragments(dir, "rules", []string{"alpha.md", "gamma.md"}, []string{"gamma.md"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result (alpha only), got %d", len(results))
	}
}

func TestResolveAllFragments_TierOverride(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Write in base tier
	baseDir := filepath.Join(dir, "base", "rules")
	_ = os.MkdirAll(baseDir, 0755)
	_ = os.WriteFile(filepath.Join(baseDir, "rule.md"), []byte("base version"), 0644)

	// Override in local tier
	localDir := filepath.Join(dir, "local", "rules")
	_ = os.MkdirAll(localDir, 0755)
	_ = os.WriteFile(filepath.Join(localDir, "rule.md"), []byte("local version"), 0644)

	results, err := ResolveAllFragments(dir, "rules", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if len(results) > 0 && results[0] != "local version" {
		t.Errorf("expected local tier override, got %q", results[0])
	}
}
