# Git Provider Contract

## Overview
Package `internal/git` defines `Provider`, an interface abstracting git repository operations, and `Service`, its implementation backed by the system git binary.

## Provider Interface
- `IsRepo() bool`: reports whether the service's root directory is inside a git work tree.
- `CurrentBranch() (string, error)`: returns the checked-out branch name. On an empty repo (no commits), falls back to `git symbolic-ref --short HEAD`, then to `"main"`.
- `HEAD() string`: returns the current commit SHA as a 40-character hex string. Returns `""` if HEAD cannot be resolved.
- `IsDirty(excludePaths ...string) bool`: reports uncommitted changes exist, filtering out paths matching any of the given prefixes.
- `HasProgress(sinceCommit string) bool`: returns true when HEAD differs from sinceCommit or the tree is dirty (excluding `.wolfcastle/`).
- `HasProgressScoped(sinceCommit string, scopeFiles []string) bool`: returns true when any file in the working tree's uncommitted changes matches a `scopeFiles` entry. Each entry is matched against paths extracted from `git status --porcelain`. Entries ending with `"/"` act as directory prefixes: a scope of `"internal/"` matches `"internal/foo.go"`, `"internal/bar/baz.go"`, etc. All other entries require an exact path match. The `sinceCommit` parameter is accepted for signature symmetry with `HasProgress` but is deliberately unused; in parallel execution mode, sibling worktrees commit to the same branch, so HEAD moves from commits the current node didn't make, rendering HEAD comparison unreliable. When git is unavailable or the directory is not a repository, returns `true` (conservative: assumes progress was made rather than blocking the pipeline). For renames, the new (destination) path is the one matched against scope entries.
- `CreateWorktree(path, branch string) error`: creates a new worktree at path on a new branch.
- `RemoveWorktree(path string) error`: removes the worktree at path.

## Construction
`NewService(repoDir string) *Service` returns a service rooted at the given directory. All git commands execute with `cmd.Dir` set to this path.

## Error Behavior
- `CurrentBranch` returns an error only when the directory is not a git repository at all. Empty repos produce a valid branch name via fallback.
- `HEAD` never errors; it returns `""` on failure.
- `IsDirty` returns `false` on git errors (conservative: assumes clean).
- `HasProgress` returns `true` on git errors (conservative: assumes progress was made).
- `HasProgressScoped` returns `true` on git errors or when the directory is not a repository, matching the same conservative stance as `HasProgress`.
- `CreateWorktree` and `RemoveWorktree` propagate the git command's error directly.

## Thread Safety
Service holds only an immutable `repoDir` string. All methods are safe for concurrent use; each invocation spawns an independent subprocess.
