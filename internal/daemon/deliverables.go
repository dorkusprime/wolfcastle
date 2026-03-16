package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// checkDeliverables verifies that all declared deliverables for a task exist
// on disk and are non-empty. Deliverable paths may contain glob characters
// (*, ?, [...]), in which case at least one matching non-empty file must
// exist. Returns the list of missing or empty deliverable paths. A task
// with no deliverables always passes.
func checkDeliverables(repoDir string, ns *state.NodeState, taskID string) []string {
	var missing []string
	for _, t := range ns.Tasks {
		if t.ID == taskID {
			for _, d := range t.Deliverables {
				path := filepath.Join(repoDir, d)
				if isGlob(d) {
					if !globHasMatch(path) {
						missing = append(missing, d)
					}
				} else {
					info, err := os.Stat(path)
					if err != nil || info.Size() == 0 {
						missing = append(missing, d)
					}
				}
			}
			break
		}
	}
	return missing
}

// checkDeliverablesChanged verifies that at least one deliverable has
// changed since the baseline snapshot was taken at claim time. Returns
// true if work was done (any file is new, modified, or the baseline
// was empty). Tasks with no deliverables or no baseline always pass.
func checkDeliverablesChanged(repoDir string, ns *state.NodeState, taskID string) bool {
	for _, t := range ns.Tasks {
		if t.ID != taskID {
			continue
		}
		if len(t.Deliverables) == 0 || len(t.BaselineHashes) == 0 {
			return true // nothing to compare against
		}
		for _, d := range t.Deliverables {
			if isGlob(d) {
				// For globs, check if any match has a different hash
				matches := globRecursive(filepath.Join(repoDir, d))
				for _, m := range matches {
					rel, _ := filepath.Rel(repoDir, m)
					current := hashFile(m)
					baseline, existed := t.BaselineHashes[rel]
					if !existed || current != baseline {
						return true
					}
				}
			} else {
				current := hashFile(filepath.Join(repoDir, d))
				baseline, existed := t.BaselineHashes[d]
				if !existed || current != baseline {
					return true
				}
			}
		}
		return false // everything matches baseline exactly
	}
	return true // task not found, don't block
}

// snapshotDeliverables computes SHA-256 hashes for all deliverable files
// that currently exist on disk. Missing files get the sentinel "missing".
// The result is stored in Task.BaselineHashes at claim time.
func snapshotDeliverables(repoDir string, deliverables []string) map[string]string {
	if len(deliverables) == 0 {
		return nil
	}
	hashes := make(map[string]string)
	for _, d := range deliverables {
		path := filepath.Join(repoDir, d)
		if isGlob(d) {
			matches := globRecursive(path)
			for _, m := range matches {
				rel, _ := filepath.Rel(repoDir, m)
				hashes[rel] = hashFile(m)
			}
			if len(matches) == 0 {
				hashes[d] = "missing"
			}
		} else {
			hashes[d] = hashFile(path)
		}
	}
	return hashes
}

// hashFile returns the hex-encoded SHA-256 of a file, or "missing" if
// the file doesn't exist or can't be read.
func hashFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return "missing"
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "missing"
	}
	return hex.EncodeToString(h.Sum(nil))
}

// isGlob reports whether the path contains glob metacharacters.
func isGlob(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

// globHasMatch returns true if the pattern matches at least one
// non-empty file on disk. Uses recursive walk for ** patterns or
// patterns that should match in subdirectories.
func globHasMatch(pattern string) bool {
	matches := globRecursive(pattern)
	for _, m := range matches {
		info, err := os.Stat(m)
		if err == nil && info.Size() > 0 {
			return true
		}
	}
	return false
}

// globRecursive expands a glob pattern, walking subdirectories when the
// pattern contains path separators with wildcards (e.g., cmd/*.go matches
// cmd/task/add.go). Standard filepath.Glob only matches one directory
// level per *, so this function walks the base directory and applies the
// filename pattern to every file found.
func globRecursive(pattern string) []string {
	// Try standard glob first for simple patterns
	matches, _ := filepath.Glob(pattern)

	// If the pattern has a wildcard in the filename part only (e.g., cmd/*.go),
	// also walk subdirectories to find matching files
	dir, filePattern := filepath.Split(pattern)
	if dir == "" || !isGlob(filePattern) {
		return matches
	}

	// Deduplicate: track what filepath.Glob already found
	seen := make(map[string]bool)
	for _, m := range matches {
		seen[m] = true
	}

	// Walk subdirectories of dir looking for files that match filePattern
	_ = filepath.Walk(strings.TrimSuffix(dir, string(filepath.Separator)), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		matched, _ := filepath.Match(filePattern, info.Name())
		if matched && !seen[path] {
			matches = append(matches, path)
		}
		return nil
	})

	return matches
}
