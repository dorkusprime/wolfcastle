package daemon

import (
	"os/exec"
	"strings"
)

// currentBranch returns the current git branch name. On a freshly
// initialized repo with no commits, HEAD doesn't resolve. In that
// case we fall back to reading .git/HEAD directly which contains
// "ref: refs/heads/main" (or master) even before the first commit.
func currentBranch(repoDir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoDir
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}

	// Fallback: check if this is a git repo with no commits.
	// git rev-parse --git-dir succeeds even on empty repos.
	check := exec.Command("git", "rev-parse", "--git-dir")
	check.Dir = repoDir
	if checkErr := check.Run(); checkErr != nil {
		return "", err // genuinely not a git repo
	}

	// It's a git repo but HEAD can't resolve (no commits yet).
	// Read the symbolic ref directly.
	sym := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	sym.Dir = repoDir
	symOut, symErr := sym.Output()
	if symErr != nil {
		return "main", nil // safe default for empty repos
	}
	return strings.TrimSpace(string(symOut)), nil
}
