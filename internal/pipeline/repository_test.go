package pipeline_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
)

func TestPromptRepository_Resolve_RawWhenNilCtx(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("greeting.md", "Hello, world.")

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	got, err := repo.Resolve("greeting", nil)
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got != "Hello, world." {
		t.Errorf("expected raw content, got %q", got)
	}
}

func TestPromptRepository_Resolve_ExecutesTemplate(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("greeting.md", "Hello, {{.Name}}.")

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	got, err := repo.Resolve("greeting", map[string]string{"Name": "Alice"})
	if err != nil {
		t.Fatalf("Resolve() error: %v", err)
	}
	if got != "Hello, Alice." {
		t.Errorf("expected executed template, got %q", got)
	}
}

func TestPromptRepository_Resolve_MissingPromptErrors(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	_, err := repo.Resolve("nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for missing prompt")
	}
	if !strings.HasPrefix(err.Error(), "prompts:") {
		t.Errorf("expected error prefixed with 'prompts:', got: %v", err)
	}
}

func TestPromptRepository_Resolve_InvalidTemplateErrors(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("broken.md", "Hello, {{.Name")

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	_, err := repo.Resolve("broken", map[string]string{"Name": "Alice"})
	if err == nil {
		t.Fatal("expected error for invalid template")
	}
	if !strings.HasPrefix(err.Error(), "prompts:") {
		t.Errorf("expected error prefixed with 'prompts:', got: %v", err)
	}
}

func TestPromptRepository_ResolveTemplate_RawWhenNilCtx(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithTemplate("artifacts/adr.md.tmpl", "# ADR: static content")

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	got, err := repo.ResolveTemplate("artifacts/adr.md", nil)
	if err != nil {
		t.Fatalf("ResolveTemplate() error: %v", err)
	}
	if got != "# ADR: static content" {
		t.Errorf("expected raw content, got %q", got)
	}
}

func TestPromptRepository_ResolveTemplate_ExecutesTemplate(t *testing.T) {
	t.Parallel()
	type ADRData struct {
		Title string
		Date  string
	}
	env := testutil.NewEnvironment(t).
		WithTemplate("artifacts/adr.md.tmpl", "# {{.Title}}\nDate: {{.Date}}")

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	got, err := repo.ResolveTemplate("artifacts/adr.md", ADRData{Title: "Use PostgreSQL", Date: "2026-03-22"})
	if err != nil {
		t.Fatalf("ResolveTemplate() error: %v", err)
	}
	want := "# Use PostgreSQL\nDate: 2026-03-22"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}

func TestPromptRepository_ResolveTemplate_MissingErrors(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	_, err := repo.ResolveTemplate("nonexistent", nil)
	if err == nil {
		t.Fatal("expected error for missing template")
	}
	if !strings.HasPrefix(err.Error(), "templates:") {
		t.Errorf("expected error prefixed with 'templates:', got: %v", err)
	}
}

func TestPromptRepository_ResolveTemplate_InvalidTemplateErrors(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithTemplate("broken.tmpl", "Hello, {{.Name")

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	_, err := repo.ResolveTemplate("broken", map[string]string{"Name": "Alice"})
	if err == nil {
		t.Fatal("expected error for invalid template")
	}
	if !strings.HasPrefix(err.Error(), "templates:") {
		t.Errorf("expected error prefixed with 'templates:', got: %v", err)
	}
}

func TestPromptRepository_ResolveTemplate_TierOverride(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithTemplate("artifacts/spec.md.tmpl", "base: {{.Title}}")

	// Write custom tier override.
	tierDirs := env.Tiers.TierDirs()
	customDir := filepath.Join(tierDirs[1], "templates", "artifacts")
	if err := os.MkdirAll(customDir, 0o755); err != nil {
		t.Fatalf("creating custom dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(customDir, "spec.md.tmpl"), []byte("custom: {{.Title}}"), 0o644); err != nil {
		t.Fatalf("writing custom template: %v", err)
	}

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	got, err := repo.ResolveTemplate("artifacts/spec.md", struct{ Title string }{"My Spec"})
	if err != nil {
		t.Fatalf("ResolveTemplate() error: %v", err)
	}
	if got != "custom: My Spec" {
		t.Errorf("expected custom to override base, got %q", got)
	}
}

func TestPromptRepository_RenderToFile_WritesExecutedTemplate(t *testing.T) {
	t.Parallel()
	type SpecData struct {
		Title string
		Body  string
	}
	env := testutil.NewEnvironment(t).
		WithTemplate("artifacts/spec.md.tmpl", "# {{.Title}}\n\n{{.Body}}")

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	dest := filepath.Join(t.TempDir(), "output", "spec.md")
	err := repo.RenderToFile("artifacts/spec.md", SpecData{Title: "Auth Flow", Body: "Details here."}, dest)
	if err != nil {
		t.Fatalf("RenderToFile() error: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	want := "# Auth Flow\n\nDetails here."
	if string(got) != want {
		t.Errorf("expected %q, got %q", want, string(got))
	}

	info, err := os.Stat(dest)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o644 {
		t.Errorf("expected permissions 0644, got %04o", perm)
	}
}

func TestPromptRepository_RenderToFile_CreatesParentDirs(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithTemplate("simple.tmpl", "hello")

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	dest := filepath.Join(t.TempDir(), "a", "b", "c", "out.txt")
	err := repo.RenderToFile("simple", nil, dest)
	if err != nil {
		t.Fatalf("RenderToFile() error: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("expected %q, got %q", "hello", string(got))
	}
}

func TestPromptRepository_RenderToFile_MissingTemplateErrors(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	dest := filepath.Join(t.TempDir(), "out.txt")
	err := repo.RenderToFile("nonexistent", nil, dest)
	if err == nil {
		t.Fatal("expected error for missing template")
	}
	if !strings.HasPrefix(err.Error(), "templates:") {
		t.Errorf("expected error prefixed with 'templates:', got: %v", err)
	}
}

func TestPromptRepository_RenderToFile_NilCtxWritesRaw(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithTemplate("static.tmpl", "raw content with {{ braces }}")

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	dest := filepath.Join(t.TempDir(), "out.txt")
	err := repo.RenderToFile("static", nil, dest)
	if err != nil {
		t.Fatalf("RenderToFile() error: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("reading output: %v", err)
	}
	if string(got) != "raw content with {{ braces }}" {
		t.Errorf("expected raw content, got %q", string(got))
	}
}

func TestPromptRepository_ResolveRaw_ReturnsContent(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithRule("style.md", "Use short sentences.")

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	got, err := repo.ResolveRaw("rules", "style.md")
	if err != nil {
		t.Fatalf("ResolveRaw() error: %v", err)
	}
	if got != "Use short sentences." {
		t.Errorf("expected rule content, got %q", got)
	}
}

func TestPromptRepository_ResolveRaw_TierOverride(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithRule("priority.md", "base content")

	// Write custom tier override.
	tierDirs := env.Tiers.TierDirs()
	customPath := filepath.Join(tierDirs[1], "rules", "priority.md")
	if err := os.WriteFile(customPath, []byte("custom content"), 0o644); err != nil {
		t.Fatalf("writing custom rule: %v", err)
	}

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	got, err := repo.ResolveRaw("rules", "priority.md")
	if err != nil {
		t.Fatalf("ResolveRaw() error: %v", err)
	}
	if got != "custom content" {
		t.Errorf("expected custom to override base, got %q", got)
	}

	// Local overrides custom.
	localPath := filepath.Join(tierDirs[2], "rules", "priority.md")
	if err := os.WriteFile(localPath, []byte("local content"), 0o644); err != nil {
		t.Fatalf("writing local rule: %v", err)
	}

	got, err = repo.ResolveRaw("rules", "priority.md")
	if err != nil {
		t.Fatalf("ResolveRaw() error: %v", err)
	}
	if got != "local content" {
		t.Errorf("expected local to override custom, got %q", got)
	}
}

func TestPromptRepository_ListFragments_SortedByName(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithRule("beta.md", "beta content").
		WithRule("alpha.md", "alpha content").
		WithRule("gamma.md", "gamma content")

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	got, err := repo.ListFragments("rules", nil, nil)
	if err != nil {
		t.Fatalf("ListFragments() error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 fragments, got %d", len(got))
	}
	if got[0] != "alpha content" || got[1] != "beta content" || got[2] != "gamma content" {
		t.Errorf("expected sorted order [alpha, beta, gamma], got %v", got)
	}
}

func TestPromptRepository_ListFragments_IncludeFiltering(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithRule("alpha.md", "alpha").
		WithRule("beta.md", "beta").
		WithRule("gamma.md", "gamma")

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	got, err := repo.ListFragments("rules", []string{"gamma.md", "alpha.md"}, nil)
	if err != nil {
		t.Fatalf("ListFragments() error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 fragments, got %d", len(got))
	}
	if got[0] != "gamma" || got[1] != "alpha" {
		t.Errorf("expected include order [gamma, alpha], got %v", got)
	}
}

func TestPromptRepository_ListFragments_ExcludeFiltering(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithRule("alpha.md", "alpha").
		WithRule("beta.md", "beta").
		WithRule("gamma.md", "gamma")

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	got, err := repo.ListFragments("rules", nil, []string{"beta.md"})
	if err != nil {
		t.Fatalf("ListFragments() error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 fragments, got %d", len(got))
	}
	if got[0] != "alpha" || got[1] != "gamma" {
		t.Errorf("expected [alpha, gamma] with beta excluded, got %v", got)
	}
}

func TestPromptRepository_ListFragments_TierOverride(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithRule("shared.md", "base version")

	// Custom tier replaces the same-named file.
	tierDirs := env.Tiers.TierDirs()
	customPath := filepath.Join(tierDirs[1], "rules", "shared.md")
	if err := os.WriteFile(customPath, []byte("custom version"), 0o644); err != nil {
		t.Fatalf("writing custom rule: %v", err)
	}

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	got, err := repo.ListFragments("rules", nil, nil)
	if err != nil {
		t.Fatalf("ListFragments() error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 fragment (overridden), got %d", len(got))
	}
	if got[0] != "custom version" {
		t.Errorf("expected custom to override base, got %q", got[0])
	}
}

func TestPromptRepository_ListFragments_IncludeMissingErrors(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithRule("alpha.md", "alpha")

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	_, err := repo.ListFragments("rules", []string{"missing.md"}, nil)
	if err == nil {
		t.Fatal("expected error for include entry not found in any tier")
	}
	if !strings.HasPrefix(err.Error(), "prompts:") {
		t.Errorf("expected error prefixed with 'prompts:', got: %v", err)
	}
}

func TestPromptRepository_WriteBase_PersistsAndResolves(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	if err := repo.WriteBase("prompts/dynamic.md", []byte("written content")); err != nil {
		t.Fatalf("WriteBase() error: %v", err)
	}

	got, err := repo.Resolve("dynamic", nil)
	if err != nil {
		t.Fatalf("Resolve() after WriteBase error: %v", err)
	}
	if got != "written content" {
		t.Errorf("expected written content, got %q", got)
	}
}

func TestPromptRepository_WriteAllBase_WritesFromFS(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	templates := fstest.MapFS{
		"prompts/one.md": {Data: []byte("first prompt")},
		"prompts/two.md": {Data: []byte("second prompt")},
		"rules/style.md": {Data: []byte("style rule")},
	}

	repo := pipeline.NewPromptRepositoryWithTiers(env.Tiers)
	if err := repo.WriteAllBase(templates); err != nil {
		t.Fatalf("WriteAllBase() error: %v", err)
	}

	// Verify prompts were written.
	got, err := repo.Resolve("one", nil)
	if err != nil {
		t.Fatalf("Resolve(one) error: %v", err)
	}
	if got != "first prompt" {
		t.Errorf("expected 'first prompt', got %q", got)
	}

	got, err = repo.Resolve("two", nil)
	if err != nil {
		t.Fatalf("Resolve(two) error: %v", err)
	}
	if got != "second prompt" {
		t.Errorf("expected 'second prompt', got %q", got)
	}

	// Verify rule was written.
	got, err = repo.ResolveRaw("rules", "style.md")
	if err != nil {
		t.Fatalf("ResolveRaw(rules/style.md) error: %v", err)
	}
	if got != "style rule" {
		t.Errorf("expected 'style rule', got %q", got)
	}
}

func TestPromptRepository_NewPromptRepository_Production(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("prod.md", "production content")

	repo := pipeline.NewPromptRepository(env.Root)
	got, err := repo.Resolve("prod", nil)
	if err != nil {
		t.Fatalf("Resolve() via production constructor error: %v", err)
	}
	if got != "production content" {
		t.Errorf("expected production content, got %q", got)
	}
}

func TestPromptRepository_NewPromptRepository_UsesCaching(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("cached.md", "original")

	repo := pipeline.NewPromptRepository(env.Root)

	// First resolve populates the cache.
	got1, err := repo.Resolve("cached", nil)
	if err != nil {
		t.Fatalf("first resolve: %v", err)
	}

	// Overwrite the file on disk.
	promptPath := filepath.Join(env.Root, "system", "base", "prompts", "cached.md")
	if err := os.WriteFile(promptPath, []byte("updated"), 0o644); err != nil {
		t.Fatalf("overwrite: %v", err)
	}

	// Second resolve should return the cached value (TTL not expired).
	got2, err := repo.Resolve("cached", nil)
	if err != nil {
		t.Fatalf("second resolve: %v", err)
	}

	if got1 != "original" {
		t.Errorf("first resolve: expected 'original', got %q", got1)
	}
	if got2 != "original" {
		t.Errorf("second resolve should return cached 'original', got %q", got2)
	}
}
