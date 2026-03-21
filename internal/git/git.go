// Package git abstracts git operations behind a Provider interface so
// callers can work against a real repository or a test stub. The
// default Service implementation shells out to the git binary.
package git

import (
	"os/exec"
	"strings"
)

// Provider abstracts git operations so callers can work against
// a real repository or a test stub.
type Provider interface {
	CurrentBranch() (string, error)
	HEAD() string
	HasProgress(sinceCommit string) bool
	IsRepo() bool
	IsDirty(excludePaths ...string) bool
	CreateWorktree(path, branch string) error
	RemoveWorktree(path string) error
}

// compile-time check
var _ Provider = (*Service)(nil)

// Service implements Provider by shelling out to the git binary.
type Service struct {
	repoDir string
}

// NewService returns a Service rooted at repoDir.
func NewService(repoDir string) *Service {
	return &Service{repoDir: repoDir}
}

// CurrentBranch returns the current branch name. On a freshly initialized
// repo with no commits, HEAD doesn't resolve; we fall back to reading the
// symbolic ref directly, then to "main" as a safe default.
func (s *Service) CurrentBranch() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = s.repoDir
	out, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(out)), nil
	}

	// Fallback: check if this is a git repo with no commits.
	check := exec.Command("git", "rev-parse", "--git-dir")
	check.Dir = s.repoDir
	if checkErr := check.Run(); checkErr != nil {
		return "", err
	}

	// It's a git repo but HEAD can't resolve (no commits yet).
	sym := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	sym.Dir = s.repoDir
	symOut, symErr := sym.Output()
	if symErr != nil {
		return "main", nil
	}
	return strings.TrimSpace(string(symOut)), nil
}

// HEAD returns the current commit SHA, or an empty string if HEAD
// cannot be resolved (empty repo, not a repo, etc.).
func (s *Service) HEAD() string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = s.repoDir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// IsRepo reports whether repoDir sits inside a git work tree.
func (s *Service) IsRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = s.repoDir
	return cmd.Run() == nil
}

// IsDirty reports whether the working tree has uncommitted changes,
// ignoring any paths that match the given prefixes.
func (s *Service) IsDirty(excludePaths ...string) bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = s.repoDir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Porcelain format: two status chars, a space, then the path.
		path := line
		if len(path) > 3 {
			path = path[3:]
		}
		// Handle renames ("old -> new").
		if idx := strings.Index(path, " -> "); idx >= 0 {
			path = path[idx+4:]
		}
		if !matchesAnyPrefix(path, excludePaths) {
			return true
		}
	}
	return false
}

// HasProgress reports whether work has occurred since sinceCommit:
// either HEAD moved or there are uncommitted changes outside .wolfcastle/.
// If git is unavailable or the directory is not a repo, assumes progress
// was made rather than blocking the pipeline.
func (s *Service) HasProgress(sinceCommit string) bool {
	if !s.IsRepo() {
		return true
	}
	return s.HEAD() != sinceCommit || s.IsDirty(".wolfcastle/")
}

// CreateWorktree adds a new git worktree at path on a new branch.
func (s *Service) CreateWorktree(path, branch string) error {
	cmd := exec.Command("git", "worktree", "add", path, "-b", branch)
	cmd.Dir = s.repoDir
	return cmd.Run()
}

// RemoveWorktree removes the worktree at path.
func (s *Service) RemoveWorktree(path string) error {
	cmd := exec.Command("git", "worktree", "remove", path)
	cmd.Dir = s.repoDir
	return cmd.Run()
}

func matchesAnyPrefix(path string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}
