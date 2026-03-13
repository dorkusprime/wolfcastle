package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/output"
)

// checkOverlap scans other engineers' project descriptions and uses the
// configured overlap advisory model to detect potential scope overlap with
// the newly created project. This is purely informational — failures are
// silently ignored (ADR-027).
func checkOverlap(projectName, description string) {
	if cfg == nil || resolver == nil {
		return
	}

	model, ok := cfg.Models[cfg.OverlapAdvisory.Model]
	if !ok {
		return
	}

	// Collect project descriptions from other engineers' namespaces
	projectsRoot := filepath.Join(wolfcastleDir, "projects")
	entries, err := os.ReadDir(projectsRoot)
	if err != nil {
		return
	}

	var descriptions []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Skip our own namespace
		if entry.Name() == resolver.Namespace {
			continue
		}
		// Walk this engineer's namespace for .md project description files
		nsDir := filepath.Join(projectsRoot, entry.Name())
		collectDescriptions(nsDir, entry.Name(), &descriptions)
	}

	if len(descriptions) == 0 {
		return
	}

	// Build the prompt
	var prompt strings.Builder
	prompt.WriteString(fmt.Sprintf("New project: %s\n", projectName))
	prompt.WriteString(fmt.Sprintf("Description:\n%s\n\n", description))
	prompt.WriteString("Existing projects from other engineers:\n\n")
	for _, d := range descriptions {
		prompt.WriteString(d)
		prompt.WriteString("\n---\n\n")
	}
	prompt.WriteString("Do any of these existing projects overlap in scope with the new project? If so, which ones and how? Be concise.\n")

	// Invoke the model with a reasonable timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	repoDir := filepath.Dir(wolfcastleDir)
	result, err := invoke.Invoke(ctx, model, prompt.String(), repoDir)
	if err != nil {
		return
	}
	if result.ExitCode != 0 {
		return
	}

	response := strings.TrimSpace(result.Stdout)
	if response != "" {
		output.PrintHuman("")
		output.PrintHuman("Overlap Advisory:")
		output.PrintHuman("  %s", response)
	}
}

// collectDescriptions recursively finds .md files in a namespace directory
// and appends their contents to the descriptions slice.
func collectDescriptions(dir, namespace string, descriptions *[]string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			collectDescriptions(fullPath, namespace, descriptions)
			continue
		}
		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}
		content := strings.TrimSpace(string(data))
		if content != "" {
			header := fmt.Sprintf("[%s] %s", namespace, entry.Name())
			*descriptions = append(*descriptions, header+"\n"+content)
		}
	}
}
