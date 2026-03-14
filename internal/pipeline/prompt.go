// Package pipeline handles prompt assembly for Wolfcastle's model invocation
// stages. It resolves rule fragments and prompt templates through a three-tier
// merge system (base, custom, local), builds iteration context with node state
// and audit metadata, and assembles the final system prompt by combining rule
// fragments, the script reference, stage-specific prompts, and task context.
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
		// Lightweight stage — stage prompt + iteration context only
		content, err := ResolveFragment(wolfcastleDir, "prompts/"+stage.PromptFile)
		if err != nil {
			return "", err
		}
		var sections []string
		sections = append(sections, content)
		if iterContext != "" {
			sections = append(sections, "# Current Task Context\n\n"+iterContext)
		}
		return strings.Join(sections, "\n\n---\n\n"), nil
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
