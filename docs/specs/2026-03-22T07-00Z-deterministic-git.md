# Deterministic Git Activity

## Problem

Git operations are currently split between the agent (prompted to `git add` and `git commit` during execution) and the daemon (`autoCommitPartialWork` on failure only). This creates several issues:

- The agent doesn't always follow the commit prompt. State files get missed.
- The daemon only commits on failure, so successful task state changes accumulate uncommitted.
- Users can't disable auto-commits because the agent's git activity is prompt-driven, not config-driven.
- The agent's `git add .` sweeps up the user's manually staged changes.

## Solution

Move all git operations into the daemon. The agent never runs `git add`, `git commit`, or any other git command. The daemon commits deterministically after every task iteration.

### Daemon commit flow

After every task iteration (success or failure), if there are uncommitted changes:

1. `git add .` (stage all changes; `.gitignore` controls what's excluded)
2. `git commit -m "<message>"`

Commit messages use a configurable prefix (default `wolfcastle`) and the task title when available, falling back to the task ID:
- Success: `<prefix>: <title> complete` (or `<task-id> complete` when no prefix/title)
- Failure: `<prefix>: <title> (attempt <N>)` (or `<task-id> partial (attempt <N>)` when no prefix/title)

A commit body with structured metadata (task ID, class, deliverables, latest breadcrumb, failure type) is appended when available.

Skip the commit if the working tree is clean (no staged changes after the add steps).

### Execute prompt changes

Remove the commit phase (Phase H) from the execute prompt. The agent should not run `git add`, `git commit`, or any git commands. Remove the `git add .wolfcastle/` instruction added in PR #137. The daemon handles everything.

The terminal marker (`WOLFCASTLE_COMPLETE`, etc.) remains the agent's signal that it's done. The daemon commits after processing the marker and updating state.

### Config

```json
{
  "git": {
    "auto_commit": true,
    "commit_on_success": true,
    "commit_on_failure": true,
    "commit_state": true,
    "commit_prefix": "wolfcastle",
    "skip_hooks_on_auto_commit": false
  }
}
```

- `auto_commit`: master switch. When `false`, no git operations happen at all. Defaults to `true`.
- `commit_on_success`: commit after successful task completion. Only applies when `auto_commit` is `true`. Defaults to `true`.
- `commit_on_failure`: commit partial work after task failure. Only applies when `auto_commit` is `true`. Defaults to `true`.
- `commit_state`: include `.wolfcastle/` state in commits. Only applies when `auto_commit` is `true`. Defaults to `true`. When `false`, `.wolfcastle/` is unstaged via `git reset HEAD -- .wolfcastle/` before committing, so only code changes are committed.
- `commit_prefix`: string prepended to commit subjects (e.g., `"wolfcastle"` produces `"wolfcastle: <title> complete"`). When empty, the prefix and colon are omitted.
- `skip_hooks_on_auto_commit`: pass `--no-verify` to git commit. Defaults to `false`.

When `auto_commit` is `false`, the fine-grained controls are ignored.

A separate `commitStateFlush` function commits pending `.wolfcastle/` state changes when the daemon goes idle (no tasks, no planning, no archiving). This ensures state from reconciliation or post-processing is persisted even between task iterations.

### Staging area handling

The implementation uses `commitDirect`: a plain `git add .` followed by `git commit`. This stages all changes (`.gitignore` controls exclusions) and commits them in one pass. The user's staging area is not preserved across daemon commits.

Earlier iterations explored two preservation strategies (stash-based and `GIT_INDEX_FILE`). The `GIT_INDEX_FILE` approach was implemented (see ADR-093) but later reversed because it caused phantom diffs visible in `git status`. The current approach accepts that daemon commits will include any user-staged changes, trading staging area purity for simplicity and reliability.

### What changes

| Current | New |
|---------|-----|
| Agent runs `git add` and `git commit` via prompt | Agent never touches git |
| Daemon commits on failure only | Daemon commits on both success and failure |
| State files missed when agent ignores prompt | State files always included (deterministic) |
| No config to disable commits | `git.auto_commit: false` disables everything |
| User's staging area clobbered | Daemon uses `commitDirect` (`git add .` + `git commit`); staging area not preserved but behavior is predictable |

### Migration

- Remove Phase H (Commit) from `execute.md`
- Remove `git add .wolfcastle/` instruction from `execute.md`
- Add commit logic to the success path in `iteration.go` (alongside existing failure path)
- Update `autoCommitPartialWork` to handle both paths
- Add new config fields to `config.Defaults()` and `config.Validate()`
- Update existing tests that assert on agent commit behavior

### Documentation pass

After implementation, review and update all docs that reference git behavior:

- `docs/humans/how-it-works.md`: update the execution protocol to reflect that the daemon commits, not the agent.
- `docs/humans/configuration.md` (and new config pages): document the `git` config section with all new fields.
- `docs/humans/collaboration.md`: update git integration section (auto-commit behavior, branch safety).
- `docs/humans/failure-and-recovery.md`: update partial work preservation to describe the new commit-on-failure behavior.
- `AGENTS.md` and `docs/agents/`: remove any guidance telling agents to commit. Add guidance that agents should NOT run git commands.
- Execute prompt (`execute.md`): verify Phase H is removed and no git instructions remain.
- README: update any references to how commits work.

## What This Does Not Cover

- `git push` (remains manual or CI-driven)
- Branch management (agent may still need to create branches in worktree mode)
- Merge conflict resolution (out of scope for deterministic commits)
