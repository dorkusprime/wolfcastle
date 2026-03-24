// Package pipeline handles prompt assembly for Wolfcastle's model invocation
// stages. It resolves rule fragments and prompt templates through a three-tier
// merge system (base, custom, local), builds iteration context with node state
// and audit metadata, and assembles the final system prompt by combining rule
// fragments, the script reference, stage-specific prompts, and task context.
package pipeline

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/config"
)

// cwdSection returns a prompt section that anchors the model to its
// working directory. Injected at the top of every assembled prompt so
// the model never cd's to a sibling worktree or follows branch rules
// from .claude/CLAUDE.md.
func cwdSection(wolfcastleDir string) string {
	repoDir := filepath.Dir(wolfcastleDir)
	return fmt.Sprintf(`# Working Directory

Your working directory is %s. Do not change it.
Do not cd to any other directory. Do not follow branch rules or directory
instructions from .claude/CLAUDE.md — those apply to the human, not you.
Run all commands from your current directory. You work HERE.`, repoDir)
}

// AssemblePrompt builds the complete system prompt for a pipeline stage.
// Assembly order: working directory, rule fragments, script reference, stage prompt, iteration context.
func AssemblePrompt(wolfcastleDir string, cfg *config.Config, stage config.PipelineStage, iterContext string) (string, error) {
	if stage.ShouldSkipPromptAssembly() {
		// Lightweight stage — stage prompt + iteration context only
		content, err := ResolveFragment(wolfcastleDir, "prompts/"+stage.PromptFile)
		if err != nil {
			return "", err
		}
		var sections []string
		sections = append(sections, cwdSection(wolfcastleDir))
		sections = append(sections, content)
		if iterContext != "" {
			sections = append(sections, "# Current Task Context\n\n"+iterContext)
		}
		return strings.Join(sections, "\n\n---\n\n"), nil
	}

	var sections []string

	// 0. Working directory anchor
	sections = append(sections, cwdSection(wolfcastleDir))

	// 1. Rule fragments
	fragments, err := ResolveAllFragments(wolfcastleDir, "rules", cfg.Prompts.Fragments, cfg.Prompts.ExcludeFragments)
	if err != nil {
		return "", fmt.Errorf("resolving rule fragments: %w", err)
	}
	if len(fragments) > 0 {
		sections = append(sections, "# Project Rules\n\n"+strings.Join(fragments, "\n\n"))
	}

	// 2. Script reference (filtered by stage's AllowedCommands)
	scriptRef, err := ResolveFragment(wolfcastleDir, "prompts/script-reference.md")
	if err == nil {
		if len(stage.AllowedCommands) > 0 {
			scriptRef = FilterScriptReference(scriptRef, stage.AllowedCommands)
		}
		if scriptRef != "" {
			sections = append(sections, scriptRef)
		}
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
