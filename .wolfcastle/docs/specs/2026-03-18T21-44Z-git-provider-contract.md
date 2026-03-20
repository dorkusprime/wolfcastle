# Git Provider Contract

## Overview
Package `internal/git` defines `Provider`, an interface abstracting git repository operations, and `Service`, its implementation backed by the system git binary.

## Provider Interface
- `IsRepo() bool`: reports whether the service's root directory is inside a git work tree.
- `CurrentBranch() (string, error)`: returns the checked-out branch name. On an empty repo (no commits), falls back to `git symbolic-ref --short HEAD`, then to `"main"`.
- `HEAD() string`: returns the current commit SHA as a 40-character hex string. Returns `""` if HEAD cannot be resolved.
- `IsDirty(excludePaths ...string) bool`: reports uncommitted changes exist, filtering out paths matching any of the given prefixes.
- `HasProgress(sinceCommit string) bool`: returns true when HEAD differs from sinceCommit or the tree is dirty (excluding `.wolfcastle/`).
- `CreateWorktree(path, branch string) error`: creates a new worktree at path on a new branch.
- `RemoveWorktree(path string) error`: removes the worktree at path.

## Construction
`NewService(repoDir string) *Service` returns a service rooted at the given directory. All git commands execute with `cmd.Dir` set to this path.

## Error Behavior
- `CurrentBranch` returns an error only when the directory is not a git repository at all. Empty repos produce a valid branch name via fallback.
- `HEAD` never errors; it returns `""` on failure.
- `IsDirty` returns `false` on git errors (conservative: assumes clean).
- `CreateWorktree` and `RemoveWorktree` propagate the git command's error directly.

## Thread Safety
Service holds only an immutable `repoDir` string. All methods are safe for concurrent use; each invocation spawns an independent subprocess.
