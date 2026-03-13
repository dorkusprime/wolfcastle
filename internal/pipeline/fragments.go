package pipeline

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ResolveFragment finds a file through the three-tier merge.
// Returns the content from the most specific tier that has it.
func ResolveFragment(wolfcastleDir, filename string) (string, error) {
	// Check local/ first (most specific)
	for _, tier := range []string{"local", "custom", "base"} {
		path := filepath.Join(wolfcastleDir, tier, filename)
		data, err := os.ReadFile(path)
		if err == nil {
			return string(data), nil
		}
	}
	return "", os.ErrNotExist
}

// ResolveAllFragments finds all rule fragments across all tiers.
// Same-named files in higher tiers replace lower tiers.
func ResolveAllFragments(wolfcastleDir string, subdir string, include, exclude []string) ([]string, error) {
	// Collect all filenames across tiers
	files := make(map[string]string) // filename -> tier path

	for _, tier := range []string{"base", "custom", "local"} {
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
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		contents = append(contents, string(data))
	}

	return contents, nil
}
