package pipeline

import (
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/config"
)

// AssemblePrompt builds the complete system prompt for a pipeline stage.
// Assembly order: rule fragments, script reference, stage prompt, iteration context.
func AssemblePrompt(wolfcastleDir string, cfg *config.Config, stage config.PipelineStage, iterContext string) (string, error) {
	if stage.ShouldSkipPromptAssembly() {
		// Lightweight stage — only its own prompt
		content, err := ResolveFragment(wolfcastleDir, "prompts/"+stage.PromptFile)
		if err != nil {
			return "", err
		}
		return content, nil
	}

	var sections []string

	// 1. Rule fragments
	fragments, err := ResolveAllFragments(wolfcastleDir, "rules", cfg.Prompts.Fragments, cfg.Prompts.ExcludeFragments)
	if err != nil {
		return "", fmt.Errorf("resolving rule fragments: %w", err)
	}
	if len(fragments) > 0 {
		sections = append(sections, "# Project Rules\n\n"+strings.Join(fragments, "\n\n"))
	}

	// 2. Script reference
	scriptRef, err := ResolveFragment(wolfcastleDir, "prompts/script-reference.md")
	if err == nil {
		sections = append(sections, scriptRef)
	}

	// 3. Stage prompt
	stagePrompt, err := ResolveFragment(wolfcastleDir, "prompts/"+stage.PromptFile)
	if err != nil {
		return "", err
	}
	sections = append(sections, stagePrompt)

	// 4. Iteration context
	if iterContext != "" {
		sections = append(sections, "# Current Task Context\n\n"+iterContext)
	}

	return strings.Join(sections, "\n\n---\n\n"), nil
}
