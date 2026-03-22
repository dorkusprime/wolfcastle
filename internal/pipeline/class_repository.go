package pipeline

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/dorkusprime/wolfcastle/internal/config"
)

// ClassRepository owns task class prompt resolution. It wraps a
// PromptRepository for file access and maintains a goroutine-safe map of
// configured class definitions. The class map is populated via Reload after
// config is loaded; all other methods read from this map under a shared lock.
type ClassRepository struct {
	prompts *PromptRepository
	mu      sync.RWMutex
	classes map[string]config.ClassDef
}

// NewClassRepository creates a ClassRepository that delegates prompt
// resolution to the given PromptRepository. The internal class map starts
// empty; callers must call Reload to populate it before Resolve, List, or
// Validate will return meaningful results.
func NewClassRepository(prompts *PromptRepository) *ClassRepository {
	return &ClassRepository{
		prompts: prompts,
		classes: make(map[string]config.ClassDef),
	}
}

// Reload replaces the internal class map with the provided definitions.
// Goroutine-safe: acquires a write lock on mu. The daemon calls this once
// at startup after loading config, and again if config is reloaded.
func (r *ClassRepository) Reload(classes map[string]config.ClassDef) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.classes = classes
}

// Resolve returns the behavioral prompt content for a class key. The key
// must exist in the configured class map. Resolution follows a one-level
// fallback chain: try the exact key's prompt file, then strip the last
// segment (after "/" or "-") and try the parent key's file.
func (r *ClassRepository) Resolve(key string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if _, ok := r.classes[key]; !ok {
		return "", fmt.Errorf("classes: key %q is not a configured class", key)
	}

	// Try exact key, then parent fallback. Track which key resolved for
	// subdirectory assembly.
	var content string
	var resolvedKey string

	c, err := r.prompts.ResolveRaw("prompts/classes", key+".md")
	if err == nil {
		content = c
		resolvedKey = key
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("classes: resolve %q: %w", key, err)
	} else {
		parent := parentKey(key)
		if parent == "" {
			return "", fmt.Errorf("classes: no prompt file for %q and no parent key to fall back to", key)
		}
		c, err = r.prompts.ResolveRaw("prompts/classes", parent+".md")
		if err == nil {
			content = c
			resolvedKey = parent
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("classes: resolve fallback %q: %w", parent, err)
		} else {
			return "", fmt.Errorf("classes: no prompt file for %q or fallback %q", key, parent)
		}
	}

	// Assemble subdirectory assets from prompts/classes/{resolvedKey}/.
	// Missing or unreadable subdirectory is not an error; the main file
	// content is always sufficient on its own.
	fragments, _ := r.prompts.ListFragments("prompts/classes/"+resolvedKey, nil, nil)
	if len(fragments) > 0 {
		content = content + "\n" + strings.Join(fragments, "\n")
	}

	return content, nil
}

// List returns all configured class keys, sorted lexicographically.
// Returns an empty slice (not nil) if no classes are loaded.
func (r *ClassRepository) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	keys := make([]string, 0, len(r.classes))
	for k := range r.classes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Validate checks every configured class key for a resolvable prompt file.
// Returns the sorted list of class keys whose prompts are missing from all
// tiers (including fallback). Returns an empty slice if every class resolves.
func (r *ClassRepository) Validate() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var missing []string
	for key := range r.classes {
		if !r.canResolve(key) {
			missing = append(missing, key)
		}
	}
	sort.Strings(missing)
	if missing == nil {
		return []string{}
	}
	return missing
}

// canResolve checks whether the exact key or its parent key resolves to a
// prompt file. Must be called with mu held (at least read-locked).
func (r *ClassRepository) canResolve(key string) bool {
	_, err := r.prompts.ResolveRaw("prompts/classes", key+".md")
	if err == nil {
		return true
	}
	parent := parentKey(key)
	if parent == "" {
		return false
	}
	_, err = r.prompts.ResolveRaw("prompts/classes", parent+".md")
	return err == nil
}

// parentKey strips the last segment from a class key. It tries "/" first
// (hierarchical keys like "typescript/react"), then "-" (hyphenated keys
// like "lang-go"). Returns "" if the key has no separator.
func parentKey(key string) string {
	if i := strings.LastIndex(key, "/"); i > 0 {
		return key[:i]
	}
	if i := strings.LastIndex(key, "-"); i > 0 {
		return key[:i]
	}
	return ""
}
