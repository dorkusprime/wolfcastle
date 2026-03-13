# ADR-015: Git Branch Behavior and Optional Worktree Isolation

## Status
Accepted

## Date
2026-03-12

## Context
Wolfcastle commits to the repo as it works. By default it should commit to whatever branch the user is on, but this creates a risk: if the user switches branches while Wolfcastle is running, commits could land on the wrong branch. Additionally, some users may prefer Wolfcastle to work in isolation without touching their current branch at all.

## Decision

### Default: Commit to Current Branch
By default, Wolfcastle works on the user's current branch. No branch creation or management.

### Branch Verification
At the start of each iteration and before every commit, Wolfcastle verifies the current branch matches the branch recorded at startup. If the branch has changed (user switched branches), Wolfcastle emits `WOLFCASTLE_BLOCKED` and stops. This is a deterministic check via `git rev-parse --abbrev-ref HEAD`, not model-dependent.

### Optional Worktree Isolation
`wolfcastle start` accepts an optional `--worktree` flag:

```
wolfcastle start --worktree feature/skill-system
```

When provided, Wolfcastle:
1. Creates a git worktree (in `.wolfcastle/worktrees/`)
2. Checks out the specified branch (or creates it from HEAD if it doesn't exist)
3. Runs all work inside the worktree
4. Cleans up the worktree on stop/completion, leaving the branch intact

The user's working directory is never touched. They can review and merge the branch at their leisure.

### Combined with Node Scoping
Both flags compose naturally:

```
wolfcastle start --worktree feature/fire --node attunement-tree/fire-impl
```

This runs only the specified subtree, in an isolated worktree, on a dedicated branch.

## Consequences
- Default behavior is simple — no git magic, commits go where the user expects
- Branch verification prevents silent corruption from mid-run branch switches
- `--worktree` gives users safe isolation without Wolfcastle managing parallel execution
- Worktree cleanup is Wolfcastle's responsibility since it created it
- `.wolfcastle/worktrees/` is gitignored (already covered by ADR-009's wildcard)
- Users get a clean review/merge workflow: run Wolfcastle, inspect the branch, merge when ready
