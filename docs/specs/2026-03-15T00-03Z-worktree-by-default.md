# Worktree-by-Default Execution

> **DRAFT. NOT ACCEPTED.** This spec explores what Wolfcastle would look like if all daemon work happened in isolated git worktrees by default. It does not propose adoption. It maps the terrain so we can decide whether to march.

## Governing ADRs

- ADR-015: Git Branch Behavior and Optional Worktree Isolation

---

## 1. The Status Quo

Today, `wolfcastle start` operates directly in the user's working tree. The daemon reads files, writes files, and commits to whatever branch `HEAD` points at. ADR-015 introduced `--worktree branch-name` as an opt-in escape hatch: Wolfcastle creates a worktree under `.wolfcastle/worktrees/`, does its work there, and cleans up when it stops.

This works. It also has costs:

- The daemon's file writes land in the same tree the user is editing. If both sides touch the same file, someone loses.
- The user must be on the correct branch before starting. Start on `main` by accident, and the daemon commits to `main`.
- There is no boundary between "my work" and "the daemon's work." `git log` is one stream. `git diff` is one mess.
- Running two daemons on two different projects in the same repo is impossible. One working tree, one daemon.

The `--worktree` flag solves all of these, but nobody uses opt-in flags. The question: should isolation be the default?

---

## 2. The Proposal

Every `wolfcastle start` creates a git worktree. The daemon operates exclusively inside that worktree. The user's working tree is never touched.

```
wolfcastle start                          # worktree created automatically
wolfcastle start --node auth-system       # worktree scoped to auth-system
wolfcastle start --no-worktree            # opt out, old behavior
```

The lifecycle:

1. User runs `wolfcastle start`.
2. Wolfcastle creates `.wolfcastle/worktrees/exec-{timestamp}/` and checks out a branch.
3. The daemon runs inside that worktree. All model output, file writes, and commits happen there.
4. On completion (or stop), Wolfcastle merges results back to the source branch per the configured merge strategy.
5. The worktree is removed.

The user's working tree stays exactly as they left it. No surprise commits, no surprise file changes, no surprise branch switches.

---

## 3. Branch Strategy

Every worktree needs a branch. Git does not allow two worktrees to have the same branch checked out. This means the daemon cannot simply use whatever branch the user is on. It needs its own.

### Option A: Ephemeral branch per execution

```
wolfcastle/exec-2026-03-15T00-03Z
```

Created at start, merged back on completion, deleted after merge. The simplest model. Each run is self-contained. The branch name carries no meaning beyond "Wolfcastle was here at this time."

**Upside:** No branch accumulation. No naming decisions. No conflicts between runs.
**Downside:** Branch names are meaningless in `git log`. If the merge fails, the user is staring at `wolfcastle/exec-2026-03-15T00-03Z` and has to figure out what it was doing.

### Option B: Branch per project node

```
wolfcastle/auth-system
wolfcastle/auth-system/login
```

The branch name mirrors the project tree address. Reused across runs. If the branch already exists, the worktree picks it up where it left off.

**Upside:** Meaningful names. Easy to find in `git branch`. Natural mapping to `--node` scoping.
**Downside:** What branch does an unscoped `wolfcastle start` use? The whole tree doesn't have a single address. Reuse means stale state can accumulate.

### Option C: Branch per engineer

```
wolfcastle/wild
wolfcastle/ci-bot
```

One branch per configured engineer identity. Long-lived. All Wolfcastle work for that engineer lands on their branch.

**Upside:** Clean separation in team repos. One branch to review per person.
**Downside:** Serializes all work for one engineer onto one branch. Cannot run two daemons in parallel for the same engineer. Conflicts accumulate across unrelated tasks.

### Option D: Configurable

```yaml
# config.json
{
  "daemon": {
    "worktree": {
      "branch_strategy": "ephemeral" | "node" | "engineer"
    }
  }
}
```

The user picks the strategy that fits their workflow. Ephemeral is the default.

**Upside:** Flexibility.
**Downside:** Three code paths to maintain. Three sets of edge cases. Three sets of documentation. Flexibility is another word for "more things to break."

---

## 4. Merge Behavior

The daemon finishes its work. The branch has commits. Those commits need to get back to the source branch (usually `main` or whatever the user had checked out). How?

### Option A: Auto-merge on completion

When the daemon stops cleanly (all tasks done or `wolfcastle stop`), it attempts to merge the worktree branch into the source branch. Fast-forward if possible; merge commit if not.

**Upside:** Zero friction. Work lands where it belongs without manual steps.
**Downside:** Merge conflicts halt the process. The user didn't ask for a merge, and now they have one. If the merge produces a bad result, it's already on their branch.

### Option B: PR creation

Wolfcastle creates a pull request from the worktree branch to the source branch. The user (or their team) reviews and merges.

**Upside:** Fits team workflows with review requirements. Clean audit trail. No surprise merges.
**Downside:** Requires GitHub/GitLab configuration. Adds a manual step. Ephemeral branches create ephemeral PRs, which could be noisy.

### Option C: Leave the branch

Wolfcastle stops. The branch stays. The user decides what to do with it. `git merge`, `git cherry-pick`, `git branch -D`, whatever they want.

**Upside:** Maximum user control. No surprises.
**Downside:** Branches accumulate. The user has to remember to do something. The gap between "work done" and "work integrated" grows silently.

### Option D: Configurable (with a sensible default)

```yaml
{
  "daemon": {
    "worktree": {
      "merge_strategy": "auto" | "pr" | "manual"
    }
  }
}
```

Default: `auto` for solo developers, `pr` for team repos (detected via remote configuration or explicit setting).

---

## 5. Conflict Handling

The user's branch moves forward. The worktree branch was forked from an older commit. They diverge. What happens?

### Rebase before each iteration

Before the daemon starts a new iteration, it rebases the worktree branch onto the latest source branch commit. This keeps the worktree close to the source and minimizes final merge pain.

**Risk:** Rebase can fail. Mid-rebase state is messy. The daemon would need to detect rebase conflicts, abort, and either retry or block.

### Merge source into worktree periodically

Same goal, different mechanism. `git merge main` into the worktree branch at iteration boundaries.

**Risk:** Merge commits clutter the branch history. Conflicts still possible. But merge is safer to abort than rebase.

### Abort and recreate

If the worktree has diverged beyond a threshold (configurable commit distance?), tear it down, create a fresh worktree from the current source HEAD, and start over. Any uncommitted progress in the old worktree is lost.

**Risk:** Wasted work. Only viable if iterations are small and cheap.

### Flag and block

Detect divergence. Emit `WOLFCASTLE_BLOCKED`. Wait for the user to resolve it.

**Risk:** The daemon stops working until a human intervenes. Defeats the purpose of automation.

### Recommended combination

Merge source into worktree at iteration boundaries. If the merge has conflicts, block and notify. Don't rebase (too fragile for automated use). Don't abort (too wasteful). Don't silently continue on a diverged branch (too dangerous).

---

## 6. State File Access

Wolfcastle's state lives in `.wolfcastle/` at the repo root. The project tree, config, inbox, PID file, daemon metadata, archives. The daemon needs all of it. But in worktree mode, the daemon's working directory is `.wolfcastle/worktrees/exec-{timestamp}/`, which is a different filesystem root.

### Option A: Symlink `.wolfcastle/` into the worktree

At worktree creation, symlink `.wolfcastle/` from the main repo into the worktree root.

**Upside:** Transparent. All existing code paths that read `.wolfcastle/` just work.
**Downside:** Git worktrees share `.git/` but not working tree files. A symlink in the worktree pointing to the main tree creates a bidirectional dependency. If the user deletes the worktree manually, the symlink breaks. If two worktrees both symlink to the same `.wolfcastle/`, concurrent writes to state files become a race.

### Option B: Keep `.wolfcastle/` in the main tree, pass it explicitly

The daemon already receives `wolfcastleDir` as a parameter (visible in `start.go`). In worktree mode, `wolfcastleDir` continues to point at the main repo's `.wolfcastle/`, while `repoDir` points at the worktree. No symlinks. No copies.

**Upside:** Already how it works. The current `--worktree` implementation does exactly this. State stays in one place. Multiple worktrees can share it (with appropriate locking per the production hardening spec).
**Downside:** Any code that derives paths from `repoDir` expecting to find `.wolfcastle/` there will break. Requires discipline: state access goes through `wolfcastleDir`, file operations go through `repoDir`. Two roots, two responsibilities.

### Option C: State directory outside the repo

Move `.wolfcastle/` to `~/.wolfcastle/repos/{repo-hash}/` or similar.

**Upside:** Clean separation. No symlinks. No confusion about which tree owns the state.
**Downside:** Breaks the "everything in the repo" model. `git clone` no longer gets you a working Wolfcastle setup. Portability drops. Discoverability drops.

**Assessment:** Option B is already implemented and working. The others solve problems that don't exist yet.

---

## 7. Multiple Daemons

If each daemon gets its own worktree, can multiple daemons run in parallel on different project nodes?

In theory, yes. Each daemon gets:
- Its own worktree directory
- Its own branch
- Its own execution loop

They share:
- `.wolfcastle/` state directory (needs locking)
- The git object store (worktrees share `.git/`)
- Model API rate limits

The state directory is the bottleneck. Today's PID file assumes one daemon. The node selection algorithm assumes one daemon traversing the tree. The inbox processor assumes one consumer.

Parallel daemons would require:
- Per-daemon PID files (`wolfcastle.{worktree-id}.pid`)
- Node claiming with locks (so two daemons don't grab the same task)
- Inbox partitioning or shared queue with atomic dequeue
- Separate log streams per daemon
- TUI awareness of multiple active daemons

This is a real feature, but it's a separate spec. Worktree-by-default enables it without requiring it.

---

## 8. Performance

Creating a worktree is fast. `git worktree add` creates a new working directory with a checkout of the specified branch. For a repo with 1,000 files, this takes milliseconds. For a repo with 100,000 files, it takes seconds. For a monorepo with millions of files, it could take minutes.

Costs per `wolfcastle start`:
- One `git worktree add` (checkout time proportional to repo size)
- Disk space for the working tree (proportional to repo size, though git shares objects)
- One `git worktree remove` on stop (fast, just deletes the directory)

For most repos this is negligible. For very large repos, the startup cost might be noticeable. A `--no-worktree` escape hatch handles this, but it shouldn't need to be the answer for large repos.

Possible mitigations for large repos:
- Sparse checkout in the worktree (only check out files relevant to the scoped node)
- Persistent worktree reuse (don't create/destroy every time; reuse if the branch matches)
- Background worktree preparation (create the worktree before the user asks for it)

---

## 9. User Experience

### What the user sees

```
$ wolfcastle start
Operating in worktree: .wolfcastle/worktrees/exec-20260315T0003Z (branch: wolfcastle/exec-20260315T0003Z)
Daemon deployed. Hunting.

$ wolfcastle status
Daemon: hunting (PID 48291)
Worktree: .wolfcastle/worktrees/exec-20260315T0003Z
Branch: wolfcastle/exec-20260315T0003Z (3 commits ahead of main)
Current target: auth-system/login/task-implement-oauth

$ wolfcastle stop
Daemon stopped.
Merging wolfcastle/exec-20260315T0003Z into main... done (fast-forward).
Cleaned up worktree.
```

### Inspecting daemon work

The user can `cd` into the worktree to inspect files. They can `git log` the branch. They can open the worktree in their editor. The worktree is a normal directory with a normal git checkout. Nothing magical.

```
$ cd .wolfcastle/worktrees/exec-20260315T0003Z
$ git log --oneline -5
$ code .
```

### TUI integration

The TUI spec (v0.7) already plans worktree management. With worktree-by-default, this moves earlier in the roadmap. The TUI would show:

- Which worktree the daemon is operating in
- Branch status relative to source (ahead/behind)
- File changes in the worktree
- Merge status on completion

---

## 10. Backward Compatibility

This is a breaking change. Users who expect `wolfcastle start` to commit directly to their current branch will get different behavior. Their working tree will stay clean, which might be disorienting if they're watching for file changes.

### Migration path

1. **v0.next:** Add `--no-worktree` flag. Default behavior unchanged. Deprecation notice when running without `--worktree`.
2. **v0.next+1:** Flip the default. `wolfcastle start` uses worktrees. `--no-worktree` restores old behavior.
3. **v1.0:** `--no-worktree` remains supported indefinitely. No removal planned.

### Config override

```yaml
{
  "daemon": {
    "worktree": {
      "enabled": true,
      "branch_strategy": "ephemeral",
      "merge_strategy": "auto"
    }
  }
}
```

Set `"enabled": false` to restore direct-tree behavior permanently for a project.

---

## 11. Non-Git Repos

Worktrees are a git feature. If the repo isn't a git repo, worktree-by-default cannot apply. Wolfcastle already detects git presence for branch verification (ADR-015). The same check gates worktree creation.

For non-git repos: fall back to direct-tree execution silently. No error. No warning. Just the old behavior.

---

## 12. Submodules

Git worktrees and submodules have a complicated relationship. `git worktree add` does not recursively initialize submodules. If the repo uses submodules, the worktree will have empty submodule directories.

Options:
- Run `git submodule update --init --recursive` after worktree creation. Correct, but slow for repos with many submodules.
- Only initialize submodules referenced by the scoped node. Requires knowing which files live in submodules, which the project tree doesn't currently track.
- Document the limitation and let users opt out with `--no-worktree` if submodules are load-bearing.

None of these are great. Submodule support would need its own investigation.

---

## 13. Interaction with Intake Stage

The intake model runs CLI commands (`wolfcastle project create`, `wolfcastle task add`) to create project structure from inbox items. These commands modify files in `.wolfcastle/projects/`, which lives in the main tree, not the worktree.

This is fine as long as the CLI commands write to `wolfcastleDir` (the main tree's `.wolfcastle/`) rather than deriving paths from the current working directory. The current implementation already does this. But it's a subtle invariant that would need to be tested and documented.

The intake model also reads repository context (file listings, code snippets) to understand what to build. In worktree mode, this context comes from the worktree, which is a checkout of the worktree branch. If the worktree branch has diverged from the user's branch, the intake model sees different code than the user sees. This could be confusing or could be exactly right, depending on perspective.

---

## 14. Open Questions

These are unresolved and would need answers before this spec could become a decision.

1. **Should the TUI show worktree status by default, or only when worktrees are active?** If worktrees are always active, the TUI always shows worktree info. This simplifies the TUI but adds visual noise for users who don't care about the underlying mechanism.

2. **What happens if the user manually deletes the worktree directory while the daemon is running?** The daemon's working directory vanishes. Every file operation fails. The daemon should detect this (inotify on the worktree root, or periodic existence check) and shut down cleanly rather than logging hundreds of "file not found" errors.

3. **Can a user run `wolfcastle start` while another daemon is already running in a different worktree?** This is the multi-daemon question from section 7. Worktree-by-default makes it mechanically possible but does not make the state management safe.

4. **How does `wolfcastle follow` work when the daemon is in a worktree?** Follow currently tails the log and shows status. It doesn't need to know about the worktree. But should it show worktree-specific information? File diffs in the worktree? Branch status?

5. **Should `wolfcastle start --node X` automatically name the branch after the node?** This is an ergonomic shortcut that blurs the line between Options A and B in the branch strategy section. It would be convenient. It would also create implicit coupling between the project tree structure and git branch names.

6. **What is the cleanup policy for failed runs?** If the daemon crashes, the worktree stays on disk. Who cleans it up? `wolfcastle doctor`? A startup check? A TTL-based garbage collector?

---

## 15. Summary of Tradeoffs

| Dimension | Direct tree (current) | Worktree-by-default (proposed) |
|---|---|---|
| Isolation | None. Daemon and user share everything. | Full. Daemon operates in its own checkout. |
| Setup cost | Zero. | One `git worktree add` per run. |
| Merge cost | Zero (already on the branch). | One merge per run. Conflicts possible. |
| Parallel daemons | Impossible. | Mechanically possible (state management TBD). |
| User mental model | "Wolfcastle commits to my branch." | "Wolfcastle works somewhere else and brings results back." |
| Failure modes | Daemon corrupts user's working tree. | Merge conflicts on completion. Orphaned worktrees on crash. |
| Backward compatibility | Current behavior. | Breaking change. Needs opt-out. |
| Non-git repos | Works. | Falls back to direct tree. |
| Submodules | Works. | Needs investigation. |
| Large repos | No overhead. | Checkout time on worktree creation. |

The direct-tree model is simpler. The worktree model is safer. Whether the safety is worth the complexity depends on how often users actually get burned by the direct-tree model, and how well the merge/conflict machinery can be made to work without human intervention.
