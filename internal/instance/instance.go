// Package instance manages the wolfcastle instance registry at
// ~/.wolfcastle/instances/. Each running daemon registers itself
// as a file in this directory, enabling discovery and routing
// when multiple daemons run concurrently in separate worktrees.
package instance

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Entry represents a registered wolfcastle daemon instance.
type Entry struct {
	PID       int       `json:"pid"`
	Worktree  string    `json:"worktree"`
	Branch    string    `json:"branch"`
	StartedAt time.Time `json:"started_at"`
}

// registryDir returns the path to the instance registry directory.
// Uses RegistryDirOverride if set (for testing), otherwise
// ~/.wolfcastle/instances/.
func registryDir() (string, error) {
	if RegistryDirOverride != "" {
		return RegistryDirOverride, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolving home directory: %w", err)
	}
	return filepath.Join(home, ".wolfcastle", "instances"), nil
}

// RegistryDirOverride allows tests to redirect the registry to a
// temp directory. Empty string means use the default.
var RegistryDirOverride string

// Register creates an instance file for the current daemon.
// The worktree path is resolved via EvalSymlinks before slugifying.
func Register(worktree, branch string) error {
	resolved, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		return fmt.Errorf("resolving worktree path: %w", err)
	}

	dir, err := registryDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating registry directory: %w", err)
	}

	entry := Entry{
		PID:       os.Getpid(),
		Worktree:  resolved,
		Branch:    branch,
		StartedAt: time.Now().UTC(),
	}
	data, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling instance entry: %w", err)
	}

	path := filepath.Join(dir, Slug(resolved)+".json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("writing instance file: %w", err)
	}
	return nil
}

// Deregister removes the instance file for the given worktree.
// The worktree path is resolved via EvalSymlinks before slugifying.
// Returns nil if the file doesn't exist (idempotent).
func Deregister(worktree string) error {
	resolved, err := filepath.EvalSymlinks(worktree)
	if err != nil {
		// Worktree may have been removed already; try the raw path.
		resolved = worktree
	}

	dir, err := registryDir()
	if err != nil {
		return err
	}

	path := filepath.Join(dir, Slug(resolved)+".json")
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing instance file: %w", err)
	}
	return nil
}

// List returns all live instances in the registry. Stale entries
// (dead PIDs) are removed automatically.
func List() ([]Entry, error) {
	dir, err := registryDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading registry directory: %w", err)
	}

	var live []Entry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var entry Entry
		if err := json.Unmarshal(data, &entry); err != nil {
			continue
		}
		if !isProcessRunning(entry.PID) {
			_ = os.Remove(filepath.Join(dir, e.Name()))
			continue
		}
		live = append(live, entry)
	}
	return live, nil
}

// Resolve finds the instance that owns the given directory. Returns
// the matching entry, or an error if zero or multiple instances match.
// When multiple instances match (nested worktrees), the longest
// worktree path wins (most specific match).
func Resolve(dir string) (*Entry, error) {
	resolved, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return nil, fmt.Errorf("resolving directory: %w", err)
	}

	instances, err := List()
	if err != nil {
		return nil, err
	}

	var matches []Entry
	for _, inst := range instances {
		if isSubpath(resolved, inst.Worktree) {
			matches = append(matches, inst)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no running instance found for %s", dir)
	}
	if len(matches) == 1 {
		return &matches[0], nil
	}

	// Longest worktree path wins (most specific match).
	best := matches[0]
	for _, m := range matches[1:] {
		if len(m.Worktree) > len(best.Worktree) {
			best = m
		}
	}
	return &best, nil
}

// Slug converts an absolute path to a filename-safe string.
// Path separators become hyphens, leading hyphens are stripped,
// and the result is lowercased.
func Slug(path string) string {
	s := strings.ToLower(path)
	s = strings.ReplaceAll(s, string(filepath.Separator), "-")
	s = strings.TrimLeft(s, "-")
	return s
}

// isSubpath returns true if child equals parent or is a subdirectory
// of parent, with a path separator boundary check.
func isSubpath(child, parent string) bool {
	if child == parent {
		return true
	}
	prefix := parent + string(filepath.Separator)
	return strings.HasPrefix(child, prefix)
}
