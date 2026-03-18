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
		"prompts/one.md":  {Data: []byte("first prompt")},
		"prompts/two.md":  {Data: []byte("second prompt")},
		"rules/style.md":  {Data: []byte("style rule")},
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
