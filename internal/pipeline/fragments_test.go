package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTiers(t *testing.T, dir string) {
	t.Helper()
	for _, tier := range []string{"base", "custom", "local"} {
		if err := os.MkdirAll(filepath.Join(dir, tier, "rules"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, tier, "prompts"), 0755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestResolveFragment_ReturnsBaseContent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	if err := os.WriteFile(filepath.Join(dir, "base", "prompts", "test.md"), []byte("base content"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveFragment(dir, "prompts/test.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "base content" {
		t.Errorf("expected %q, got %q", "base content", got)
	}
}

func TestResolveFragment_CustomReplacesBase(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	os.WriteFile(filepath.Join(dir, "base", "prompts", "test.md"), []byte("base"), 0644)
	os.WriteFile(filepath.Join(dir, "custom", "prompts", "test.md"), []byte("custom"), 0644)

	got, err := ResolveFragment(dir, "prompts/test.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "custom" {
		t.Errorf("expected %q, got %q", "custom", got)
	}
}

func TestResolveFragment_LocalReplacesBoth(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	os.WriteFile(filepath.Join(dir, "base", "prompts", "test.md"), []byte("base"), 0644)
	os.WriteFile(filepath.Join(dir, "custom", "prompts", "test.md"), []byte("custom"), 0644)
	os.WriteFile(filepath.Join(dir, "local", "prompts", "test.md"), []byte("local"), 0644)

	got, err := ResolveFragment(dir, "prompts/test.md")
	if err != nil {
		t.Fatal(err)
	}
	if got != "local" {
		t.Errorf("expected %q, got %q", "local", got)
	}
}

func TestResolveFragment_ErrorWhenNotFound(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	_, err := ResolveFragment(dir, "prompts/missing.md")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestResolveAllFragments_DiscoversMergesAcrossTiers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	// base has a.md and b.md
	os.WriteFile(filepath.Join(dir, "base", "rules", "a.md"), []byte("base-a"), 0644)
	os.WriteFile(filepath.Join(dir, "base", "rules", "b.md"), []byte("base-b"), 0644)

	// custom overrides a.md, adds c.md
	os.WriteFile(filepath.Join(dir, "custom", "rules", "a.md"), []byte("custom-a"), 0644)
	os.WriteFile(filepath.Join(dir, "custom", "rules", "c.md"), []byte("custom-c"), 0644)

	contents, err := ResolveAllFragments(dir, "rules", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(contents) != 3 {
		t.Fatalf("expected 3 fragments, got %d", len(contents))
	}
	// Sorted: a.md, b.md, c.md
	// a.md should be custom override
	if contents[0] != "custom-a" {
		t.Errorf("a.md: expected %q, got %q", "custom-a", contents[0])
	}
	if contents[1] != "base-b" {
		t.Errorf("b.md: expected %q, got %q", "base-b", contents[1])
	}
	if contents[2] != "custom-c" {
		t.Errorf("c.md: expected %q, got %q", "custom-c", contents[2])
	}
}

func TestResolveAllFragments_RespectsExcludeList(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	os.WriteFile(filepath.Join(dir, "base", "rules", "a.md"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "base", "rules", "b.md"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(dir, "base", "rules", "c.md"), []byte("c"), 0644)

	contents, err := ResolveAllFragments(dir, "rules", nil, []string{"b.md"})
	if err != nil {
		t.Fatal(err)
	}

	if len(contents) != 2 {
		t.Fatalf("expected 2 fragments, got %d", len(contents))
	}
	if contents[0] != "a" {
		t.Errorf("expected %q, got %q", "a", contents[0])
	}
	if contents[1] != "c" {
		t.Errorf("expected %q, got %q", "c", contents[1])
	}
}

func TestResolveAllFragments_RespectsIncludeListOrdering(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupTiers(t, dir)

	os.WriteFile(filepath.Join(dir, "base", "rules", "a.md"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "base", "rules", "b.md"), []byte("b"), 0644)
	os.WriteFile(filepath.Join(dir, "base", "rules", "c.md"), []byte("c"), 0644)

	// Include list specifies order: c, a (skipping b)
	contents, err := ResolveAllFragments(dir, "rules", []string{"c.md", "a.md"}, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(contents) != 2 {
		t.Fatalf("expected 2 fragments, got %d", len(contents))
	}
	if contents[0] != "c" {
		t.Errorf("first: expected %q, got %q", "c", contents[0])
	}
	if contents[1] != "a" {
		t.Errorf("second: expected %q, got %q", "a", contents[1])
	}
}
