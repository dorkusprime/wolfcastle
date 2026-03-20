package pipeline

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/dorkusprime/wolfcastle/internal/tierfs"
)

// Tiers returns the three-tier directory paths relative to the wolfcastle
// root, in resolution order (base, custom, local). Derived from tierfs,
// the single source of truth for tier names (ADR-063).
//
// Deprecated: prefer tierfs.SystemTierPaths() or a tierfs.Resolver directly.
// Kept for call sites that still need the "system/base" style paths.
var Tiers = tierfs.SystemTierPaths()

// ResolveFragment finds a file through the three-tier merge.
// Returns the content from the most specific tier that has it.
func ResolveFragment(wolfcastleDir, filename string) (string, error) {
	r := tierfs.New(filepath.Join(wolfcastleDir, tierfs.SystemPrefix))
	data, err := r.Resolve(filename)
	if err != nil {
		if isNotExist(err) {
			return "", fmt.Errorf("fragment %q not found in any tier", filename)
		}
		return "", fmt.Errorf("reading %s: %w", filename, err)
	}
	return string(data), nil
}

// ResolveAllFragments finds all rule fragments across all tiers.
// Same-named files in higher tiers replace lower tiers.
func ResolveAllFragments(wolfcastleDir string, subdir string, include, exclude []string) ([]string, error) {
	r := tierfs.New(filepath.Join(wolfcastleDir, tierfs.SystemPrefix))
	allFiles, err := r.ResolveAll(subdir)
	if err != nil {
		return nil, fmt.Errorf("resolving fragments in %s: %w", subdir, err)
	}

	// Convert []byte values to string map for filtering
	files := make(map[string]string, len(allFiles))
	for name, data := range allFiles {
		files[name] = string(data)
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
		content, ok := files[name]
		if !ok {
			if len(include) > 0 {
				return nil, fmt.Errorf("fragment %q specified in include list not found in any tier", name)
			}
			continue
		}
		contents = append(contents, content)
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

// isNotExist checks whether an error wraps os.ErrNotExist.
func isNotExist(err error) bool {
	return errors.Is(err, os.ErrNotExist)
}
