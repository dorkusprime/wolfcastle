package daemon

import (
	"os"
	"os/exec"
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

// gitHEAD returns the current HEAD commit SHA, or empty string if
// git is unavailable or the directory isn't a repo.
func gitHEAD(repoDir string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// checkGitProgress returns true if either: (1) the git working tree
// has uncommitted changes outside .wolfcastle/, or (2) HEAD has moved
// since beforeHEAD (the model committed new work). Models are
// instructed to commit, so a clean working tree with a new commit
// still counts as progress.
func checkGitProgress(repoDir string, beforeHEAD string) bool {
	// Check for new commits first (handles the common case where
	// the model committed its changes and the tree is clean).
	if beforeHEAD != "" {
		currentHEAD := gitHEAD(repoDir)
		if currentHEAD != "" && currentHEAD != beforeHEAD {
			return true
		}
	}

	// Check for uncommitted changes outside .wolfcastle/
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err != nil {
		// If git isn't available or this isn't a repo, assume progress
		// was made rather than blocking the pipeline.
		return true
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		path := line
		if len(path) > 3 {
			path = path[3:]
		}
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = path[idx+4:]
		}
		if !strings.HasPrefix(path, ".wolfcastle/") && !strings.HasPrefix(path, ".wolfcastle\\") {
			return true
		}
	}
	return false
}

// isGlob reports whether the path contains glob metacharacters.
func isGlob(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

// globHasMatch returns true if the pattern matches at least one
// non-empty file on disk. Uses recursive walk for patterns whose
// filename part contains wildcards.
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
// filename part contains wildcards (e.g., cmd/*.go matches cmd/task/add.go).
// Standard filepath.Glob only matches one directory level per *, so this
// function walks the base directory and applies the filename pattern to
// every file found.
func globRecursive(pattern string) []string {
	matches, _ := filepath.Glob(pattern)

	dir, filePattern := filepath.Split(pattern)
	if dir == "" || !isGlob(filePattern) {
		return matches
	}

	seen := make(map[string]bool)
	for _, m := range matches {
		seen[m] = true
	}

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
