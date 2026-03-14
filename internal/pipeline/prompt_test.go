package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
)

func setupPromptDir(t *testing.T, dir string) {
	t.Helper()
	setupTiers(t, dir)

	// Write a rule fragment
	_ = os.WriteFile(filepath.Join(dir, "base", "rules", "rule1.md"), []byte("Rule one content"), 0644)

	// Write script reference
	_ = os.WriteFile(filepath.Join(dir, "base", "prompts", "script-reference.md"), []byte("Script reference"), 0644)

	// Write stage prompt
	_ = os.WriteFile(filepath.Join(dir, "base", "prompts", "execute.md"), []byte("Execute prompt"), 0644)

	// Write a lightweight stage prompt
	_ = os.WriteFile(filepath.Join(dir, "base", "prompts", "expand.md"), []byte("Expand prompt"), 0644)
}

func TestAssemblePrompt_IncludesRuleFragments(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupPromptDir(t, dir)

	cfg := config.Defaults()
	stage := config.PipelineStage{Name: "execute", Model: "heavy", PromptFile: "execute.md"}

	result, err := AssemblePrompt(dir, cfg, stage, "")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "Rule one content") {
		t.Error("expected prompt to include rule fragments")
	}
}

func TestAssemblePrompt_IncludesStagePrompt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupPromptDir(t, dir)

	cfg := config.Defaults()
	stage := config.PipelineStage{Name: "execute", Model: "heavy", PromptFile: "execute.md"}

	result, err := AssemblePrompt(dir, cfg, stage, "")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "Execute prompt") {
		t.Error("expected prompt to include stage prompt")
	}
}

func TestAssemblePrompt_IncludesIterationContext(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupPromptDir(t, dir)

	cfg := config.Defaults()
	stage := config.PipelineStage{Name: "execute", Model: "heavy", PromptFile: "execute.md"}

	result, err := AssemblePrompt(dir, cfg, stage, "task context here")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "task context here") {
		t.Error("expected prompt to include iteration context")
	}
	if !strings.Contains(result, "# Current Task Context") {
		t.Error("expected prompt to include context header")
	}
}

func TestAssemblePrompt_SkipPromptAssembly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupPromptDir(t, dir)

	cfg := config.Defaults()
	skip := true
	stage := config.PipelineStage{
		Name:               "expand",
		Model:              "fast",
		PromptFile:         "expand.md",
		SkipPromptAssembly: &skip,
	}

	result, err := AssemblePrompt(dir, cfg, stage, "task context here")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(result, "Expand prompt") {
		t.Error("expected stage prompt in output")
	}
	if strings.Contains(result, "Rule one content") {
		t.Error("skip_prompt_assembly should exclude rule fragments")
	}
	if strings.Contains(result, "Script reference") {
		t.Error("skip_prompt_assembly should exclude script reference")
	}
	if !strings.Contains(result, "task context here") {
		t.Error("skip_prompt_assembly should still include iteration context")
	}
}
