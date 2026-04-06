# Use separate GIT_INDEX_FILE for daemon commits

## Status
Superseded

## Date
2026-03-22

## Context
The daemon's git add/commit cycle can clobber files the user has manually staged. Two approaches were considered for preserving the user's staging area.

## Options Considered
1. **Stash-based**: `git stash --keep-index` before commit, `git stash pop` after. Fragile: conflicts with existing stashes, fails on edge cases (empty stash, merge conflicts during pop), and temporarily modifies the working tree.
2. **Separate GIT_INDEX_FILE**: Create a temp index seeded from HEAD via `git read-tree`, stage and commit through it, then delete it. The user's `.git/index` is never read or written.

## Decision
Separate GIT_INDEX_FILE. It avoids all stash edge cases and guarantees the user's index is byte-identical before and after the daemon commit. The only downside is cosmetic: `git status` may show stale entries because the default index references the pre-commit tree while HEAD has moved forward. This is data-safe and self-corrects the next time the user stages or commits.

## Superseded
This decision was reversed after implementation. The separate `GIT_INDEX_FILE` approach caused phantom diffs visible in `git status` that proved more disruptive than expected. The current implementation uses `commitDirect`, which runs a plain `git add .` followed by `git commit` against the default index. Staging area preservation was dropped in favor of simplicity and predictable behavior. No replacement ADR was written because the reversal returned to the pre-existing default behavior.

## Consequences
- The original consequences (user staging area preserved, transient `git status` noise) no longer apply.
- The current `commitDirect` approach accepts that daemon commits may include user-staged changes, but the behavior is deterministic and free of index-related edge cases.
