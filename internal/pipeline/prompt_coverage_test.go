package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
)

func TestAssemblePrompt_LightweightStage(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create a prompt file for the lightweight stage
	promptsDir := filepath.Join(dir, "system", "base", "prompts")
	_ = os.MkdirAll(promptsDir, 0755)
	_ = os.WriteFile(filepath.Join(promptsDir, "navigate.md"),
		[]byte("Navigate to next task"), 0644)

	cfg := config.Defaults()
	skip := true
	stage := config.PipelineStage{
		Name:               "navigate",
		PromptFile:         "navigate.md",
		SkipPromptAssembly: &skip,
	}

	result, err := AssemblePrompt(dir, cfg, stage, "context here")
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("expected non-empty assembled prompt")
	}
}

func TestAssemblePrompt_FullStageWithRules(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create prompts and rules
	stagesDir := filepath.Join(dir, "system", "base", "prompts", "stages")
	_ = os.MkdirAll(stagesDir, 0755)
	_ = os.WriteFile(filepath.Join(stagesDir, "execute.md"),
		[]byte("Execute the task"), 0644)

	rulesDir := filepath.Join(dir, "system", "base", "rules")
	_ = os.MkdirAll(rulesDir, 0755)
	_ = os.WriteFile(filepath.Join(rulesDir, "safety.md"),
		[]byte("Always validate input"), 0644)

	cfg := config.Defaults()
	stage := config.PipelineStage{
		Name:       "execute",
		PromptFile: "stages/execute.md",
	}

	result, err := AssemblePrompt(dir, cfg, stage, "task context")
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Error("expected non-empty assembled prompt")
	}
}

func TestResolvePromptTemplate_MissingTemplate(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	_, err := ResolvePromptTemplate(dir, "nonexistent.md", nil)
	if err == nil {
		t.Error("expected error for missing template")
	}
}

func TestResolvePromptTemplate_WithData(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	promptsDir := filepath.Join(dir, "system", "base", "prompts")
	_ = os.MkdirAll(promptsDir, 0755)
	_ = os.WriteFile(filepath.Join(promptsDir, "test.md"),
		[]byte("Hello {{.Name}}!"), 0644)

	result, err := ResolvePromptTemplate(dir, "test.md", map[string]any{"Name": "World"})
	if err != nil {
		t.Fatal(err)
	}
	if result != "Hello World!" {
		t.Errorf("expected 'Hello World!', got %q", result)
	}
}
