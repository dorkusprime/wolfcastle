package pipeline

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/project"
)

func setupPromptDir(t *testing.T, dir string) {
	t.Helper()
	setupTiers(t, dir)

	// Write a rule fragment
	_ = os.WriteFile(filepath.Join(dir, "system", "base", "rules", "rule1.md"), []byte("Rule one content"), 0644)

	// Write script reference
	_ = os.WriteFile(filepath.Join(dir, "system", "base", "prompts", "script-reference.md"), []byte("Script reference"), 0644)

	// Write stage prompt
	_ = os.WriteFile(filepath.Join(dir, "system", "base", "prompts", "execute.md"), []byte("Execute prompt"), 0644)

	// Write a lightweight stage prompt
	_ = os.WriteFile(filepath.Join(dir, "system", "base", "prompts", "expand.md"), []byte("Expand prompt"), 0644)
}

// setupEmbeddedPrompts writes the real embedded templates to a temp dir
// so tests can verify that production prompts contain expected content.
func setupEmbeddedPrompts(t *testing.T, dir string) {
	t.Helper()
	setupTiers(t, dir)

	// Extract real embedded templates
	err := project.WriteBasePrompts(dir) //nolint:staticcheck // testing deprecated function
	if err != nil {
		t.Fatalf("writing embedded prompts: %v", err)
	}
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

// Tests below verify that assembling with real embedded templates produces
// prompts with the right stage-specific content. This catches the case where
// the wrong prompt file is wired to the wrong stage.

func TestAssemblePrompt_ExecuteStageContainsTerminalMarkers(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupEmbeddedPrompts(t, dir)

	cfg := config.Defaults()
	stage := cfg.Pipeline.Stages[1] // execute with AllowedCommands

	result, err := AssemblePrompt(dir, cfg, stage, "task context")
	if err != nil {
		t.Fatal(err)
	}

	required := []string{
		"WOLFCASTLE_COMPLETE",
		"WOLFCASTLE_YIELD",
		"WOLFCASTLE_BLOCKED",
		"execution agent",
	}
	for _, s := range required {
		if !strings.Contains(result, s) {
			t.Errorf("execute prompt missing %q", s)
		}
	}

	// Execute prompt must NOT contain intake-specific markers
	if strings.Contains(result, "WOLFCASTLE_INTAKE_COMPLETE") {
		t.Error("execute prompt should not contain WOLFCASTLE_INTAKE_COMPLETE")
	}
	if strings.Contains(result, "STOP after creating projects") {
		t.Error("execute prompt should not contain intake-specific instructions")
	}

	// Execute prompt must NOT contain commands outside its AllowedCommands.
	// Note: project create, adr create, spec create/link are allowed.
	forbidden := []string{
		"### wolfcastle task claim",
		"### wolfcastle task complete",
		"### wolfcastle navigate",
		"### wolfcastle inbox add",
		"### wolfcastle archive add",
	}
	for _, s := range forbidden {
		if strings.Contains(result, s) {
			t.Errorf("execute prompt should not contain %q", s)
		}
	}
}

func TestAssemblePrompt_IntakeStageContainsProjectCreation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupEmbeddedPrompts(t, dir)

	cfg := config.Defaults()
	stage := cfg.Pipeline.Stages[0] // intake with AllowedCommands

	result, err := AssemblePrompt(dir, cfg, stage, "inbox context")
	if err != nil {
		t.Fatal(err)
	}

	required := []string{
		"wolfcastle project create",
		"wolfcastle task add",
		"WOLFCASTLE_INTAKE_COMPLETE",
		"STOP after creating projects and tasks",
	}
	for _, s := range required {
		if !strings.Contains(result, s) {
			t.Errorf("intake prompt missing %q", s)
		}
	}

	// Intake prompt must NOT contain execute-specific instructions
	if strings.Contains(result, "WOLFCASTLE_COMPLETE") {
		t.Error("intake prompt should not contain WOLFCASTLE_COMPLETE")
	}
	if strings.Contains(result, "WOLFCASTLE_YIELD") {
		t.Error("intake prompt should not contain WOLFCASTLE_YIELD")
	}

	// Intake prompt must NOT contain commands outside its AllowedCommands
	forbidden := []string{
		"### wolfcastle task claim",
		"### wolfcastle task complete",
		"### wolfcastle task block",
		"### wolfcastle audit breadcrumb",
		"### wolfcastle navigate",
		"### wolfcastle inbox add",
		"### wolfcastle archive add",
		"### wolfcastle adr create",
	}
	for _, s := range forbidden {
		if strings.Contains(result, s) {
			t.Errorf("intake prompt should not contain %q", s)
		}
	}
}

func TestAssemblePrompt_StagesProduceDifferentPrompts(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	setupEmbeddedPrompts(t, dir)

	cfg := config.Defaults()

	intake, err := AssemblePrompt(dir, cfg,
		config.PipelineStage{Name: "intake", Model: "mid", PromptFile: "intake.md"},
		"inbox items")
	if err != nil {
		t.Fatal(err)
	}

	execute, err := AssemblePrompt(dir, cfg,
		config.PipelineStage{Name: "execute", Model: "heavy", PromptFile: "execute.md"},
		"task context")
	if err != nil {
		t.Fatal(err)
	}

	if intake == execute {
		t.Error("intake and execute prompts should not be identical")
	}
}
