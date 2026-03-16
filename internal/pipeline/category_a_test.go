package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
)

// ═══════════════════════════════════════════════════════════════════════════
// fragments.go — include list references missing fragment
// ═══════════════════════════════════════════════════════════════════════════

func TestResolveAllFragments_IncludeListMissingFragment(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create one fragment but include-list references a second that doesn't exist
	baseDir := filepath.Join(dir, "system", "base", "rules")
	_ = os.MkdirAll(baseDir, 0755)
	_ = os.WriteFile(filepath.Join(baseDir, "exists.md"), []byte("content"), 0644)

	_, err := ResolveAllFragments(dir, "rules", []string{"exists.md", "missing.md"}, nil)
	if err == nil {
		t.Fatal("expected error for missing fragment in include list")
	}
	if !strings.Contains(err.Error(), "missing.md") {
		t.Errorf("expected error to mention 'missing.md', got: %v", err)
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// fragments.go — invalid Go template syntax
// ═══════════════════════════════════════════════════════════════════════════

func TestResolvePromptTemplate_InvalidGoTemplateSyntax(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	promptsDir := filepath.Join(dir, "system", "base", "prompts")
	_ = os.MkdirAll(promptsDir, 0755)
	_ = os.WriteFile(filepath.Join(promptsDir, "bad.md"),
		[]byte("Hello {{.Name"), 0644) // Unclosed template action

	_, err := ResolvePromptTemplate(dir, "bad.md", map[string]any{"Name": "World"})
	if err == nil {
		t.Fatal("expected error for invalid Go template syntax")
	}
	if !strings.Contains(err.Error(), "parsing prompt template") {
		t.Errorf("expected 'parsing prompt template' in error, got: %v", err)
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// prompt.go — skip assembly error path
// ═══════════════════════════════════════════════════════════════════════════

func TestAssemblePrompt_SkipAssembly_MissingPromptFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Create minimal structure but don't write the prompt file
	_ = os.MkdirAll(filepath.Join(dir, "system", "base", "prompts"), 0755)

	cfg := config.Defaults()
	skip := true
	stage := config.PipelineStage{
		Name:               "navigate",
		PromptFile:         "nonexistent.md",
		SkipPromptAssembly: &skip,
	}

	_, err := AssemblePrompt(dir, cfg, stage, "context")
	if err == nil {
		t.Fatal("expected error when prompt file is missing in skip-assembly mode")
	}
}

// ═══════════════════════════════════════════════════════════════════════════
// prompt.go — fragment resolution error path
// ═══════════════════════════════════════════════════════════════════════════

func TestAssemblePrompt_FragmentResolutionError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create the prompt file so the stage prompt resolves
	promptsDir := filepath.Join(dir, "system", "base", "prompts")
	_ = os.MkdirAll(promptsDir, 0755)
	_ = os.WriteFile(filepath.Join(promptsDir, "execute.md"), []byte("Execute"), 0644)

	// Configure fragments to include a nonexistent one
	cfg := config.Defaults()
	cfg.Prompts.Fragments = []string{"nonexistent-fragment.md"}

	stage := config.PipelineStage{
		Name:       "execute",
		PromptFile: "execute.md",
	}

	_, err := AssemblePrompt(dir, cfg, stage, "context")
	if err == nil {
		t.Fatal("expected error for missing fragment in include list")
	}
	if !strings.Contains(err.Error(), "resolving rule fragments") {
		t.Errorf("expected 'resolving rule fragments' in error, got: %v", err)
	}
}
