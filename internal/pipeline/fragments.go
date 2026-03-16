package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

// Tiers defines the resolution order for three-tier file merging.
// Listed from lowest to highest priority; later entries override earlier
// ones for same-named files. Defined once to prevent drift between
// ResolveFragment, ResolveAllFragments, and any other tier-aware code.
var Tiers = []string{"system/base", "system/custom", "system/local"}

// ResolveFragment finds a file through the three-tier merge.
// Returns the content from the most specific tier that has it.
func ResolveFragment(wolfcastleDir, filename string) (string, error) {
	// Walk tiers from most specific to least, return on first match.
	for i := len(Tiers) - 1; i >= 0; i-- {
		path := filepath.Join(wolfcastleDir, Tiers[i], filename)
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("reading %s from tier %s: %w", filename, Tiers[i], err)
		}
	}
	return "", fmt.Errorf("fragment %q not found in any tier", filename)
}

// ResolveAllFragments finds all rule fragments across all tiers.
// Same-named files in higher tiers replace lower tiers.
func ResolveAllFragments(wolfcastleDir string, subdir string, include, exclude []string) ([]string, error) {
	// Collect all filenames across tiers. Iterating lowest-to-highest
	// means later map writes (higher tiers) overwrite earlier ones.
	files := make(map[string]string) // filename -> full path

	for _, tier := range Tiers {
		dir := filepath.Join(wolfcastleDir, tier, subdir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			files[e.Name()] = filepath.Join(dir, e.Name())
		}
	}

	// Filter by include/exclude
	excludeSet := make(map[string]bool)
	for _, e := range exclude {
		excludeSet[e] = true
	}

	var names []string
	if len(include) > 0 {
		names = include
	} else {
		for name := range files {
			names = append(names, name)
		}
		sort.Strings(names)
	}

	var contents []string
	for _, name := range names {
		if excludeSet[name] {
			continue
		}
		path, ok := files[name]
		if !ok {
			if len(include) > 0 {
				return nil, fmt.Errorf("fragment %q specified in include list not found in any tier", name)
			}
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading fragment %q: %w", name, err)
		}
		contents = append(contents, string(data))
	}

	return contents, nil
}

// ResolvePromptTemplate loads a prompt file via the three-tier system and
// optionally executes it as a Go text/template. If ctx is nil the raw
// content is returned without template execution.
func ResolvePromptTemplate(wolfcastleDir, promptFile string, ctx any) (string, error) {
	raw, err := ResolveFragment(wolfcastleDir, "prompts/"+promptFile)
	if err != nil {
		return "", err
	}
	if ctx == nil {
		return raw, nil
	}
	tmpl, err := template.New(promptFile).Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parsing prompt template %s: %w", promptFile, err)
	}
	var buf strings.Builder
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("executing prompt template %s: %w", promptFile, err)
	}
	return buf.String(), nil
}
