# ADR-014: Serial Execution with Node Scoping

## Status
Accepted

## Date
2026-03-12

## Context
Ralph was strictly serial: one task per iteration, one daemon, one branch. Parallel execution via git worktrees is a proven pattern (Cursor, Claude Code, Nx), but managing worktree lifecycle, merge-back, conflict resolution, and state synchronization across worktrees adds significant complexity and risk to Wolfcastle.

We considered making parallel execution a configurable feature but concluded that the overhead of safe automation is too high for an opt-in feature, and manual configuration that requires the user to do the heavy lifting shouldn't pretend to be a product feature.

## Decision

### Serial by Default, Serial Only
Wolfcastle executes one task at a time. There is no concurrency setting and no parallel execution machinery. (Note: single-instance worktree isolation via `--worktree` is a separate concern: see ADR-015. This ADR rejects *multi-instance parallel orchestration*, not worktrees as an isolation mechanism.)

### Node Scoping
`wolfcastle start` accepts an optional `--node` flag to scope execution to a specific subtree:

```
wolfcastle start                                    # full tree, depth-first
wolfcastle start --node attunement-tree              # only this project
wolfcastle start --node attunement-tree/fire-impl    # only this sub-project
```

This exists because subtree scoping is useful in its own right: focusing work, testing a specific project, resuming after a partial run.

### Parallel is a User Adventure
Nothing in Wolfcastle prevents a user from creating their own git worktrees, running separate `wolfcastle start --node` instances scoped to non-overlapping subtrees, and merging the results. But Wolfcastle does not advertise, configure, manage, or support this workflow. Each Wolfcastle instance operates on its own engineer namespace. Running separate instances in separate worktrees works as long as they target non-overlapping namespaces, but Wolfcastle does not coordinate across instances. The user takes full responsibility for non-overlap, merge conflicts, and cleanup.

## Consequences
- Zero complexity in Wolfcastle for parallel execution
- `--node` scoping is a simple, useful feature independent of parallelism
- Users who want parallelism can achieve it with standard git tooling. Wolfcastle doesn't get in the way
- No state synchronization problem to solve across worktrees
- Documentation may include a brief note acknowledging the possibility without endorsing it
