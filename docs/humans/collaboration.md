# Collaboration

## Engineer Namespacing

Multiple engineers work on the same repo simultaneously. No merge conflicts. No coordination overhead. Each engineer's project tree lives in its own namespace:

```
.wolfcastle/system/projects/
  wild-macbook/          <- your tree
  dave-workstation/      <- Dave's tree
  sarah-laptop/          <- Sarah's tree
```

Each engineer reads and writes only their own namespace. Everyone can see everyone else's work (the `projects/` directory is committed), but nobody steps on anyone else's [state](how-it-works.md#distributed-state).

[`wolfcastle status`](cli.md#commands) shows your tree. `wolfcastle status --all` aggregates across all engineers at runtime. No shared index file. No merge conflicts.

### Overlap Advisory

When you create a new project, Wolfcastle optionally scans other engineers' active projects and alerts you if scope overlaps. Read-only. Informational. No blocking, no state changes.

```json
{
  "overlap_advisory": {
    "enabled": true,
    "model": "fast"
  }
}
```

## Git Integration

### Default Behavior

Wolfcastle commits to your current branch. No branch creation. No branch management. At the start of each iteration and before every commit, Wolfcastle verifies the current branch matches the branch recorded at startup. If someone switched branches underneath it, [the daemon](how-it-works.md#the-daemon) blocks immediately. It does not commit to the wrong branch.

### Worktree Isolation

For those who want separation:

```
wolfcastle start --worktree feature/auth
```

Wolfcastle creates a git worktree in `.wolfcastle/worktrees/`, checks out the specified branch (or creates it from HEAD), and runs all work inside the worktree. Your working directory is never touched. Review the work when you're ready. Merge it when you're satisfied. The worktree is cleaned up on stop or completion.

Node scoping and worktree isolation compose:

```
wolfcastle start --worktree feature/auth --node backend/auth
```

Isolated branch. Focused subtree.

## Specs

Living specifications that travel with the work:

```
wolfcastle spec create --node backend/auth "Authentication Protocol"
wolfcastle spec link --node backend/auth/oauth oauth-spec.md
wolfcastle spec list --node backend/auth
```

Specs live in the committed `docs/specs/` directory with ISO 8601 timestamp filenames. Each node's [`state.json`](how-it-works.md#distributed-state) references the specs relevant to it. Only referenced specs are injected into the [model's context](how-it-works.md#seven-phase-execution) for that node. Multiple nodes can reference the same spec.

## Logging

Logs are NDJSON, one self-contained JSON record per line. Each daemon iteration produces its own log file:

```
.wolfcastle/system/logs/0001-20260312T18-45Z.jsonl
.wolfcastle/system/logs/0002-20260312T18-47Z.jsonl
```

`wolfcastle log` finds the latest file and tails it, watching for new files as iterations advance. (`follow` still works as an alias.)

```json
{
  "logs": {
    "max_files": 100,
    "max_age_days": 30,
    "compress": true
  }
}
```

Query with `jq` for quick filters. Point DuckDB at the directory for SQL over your entire log history.

## Archive

When a project completes, it graduates to the archive. Each archive entry is a self-contained Markdown file:

```
.wolfcastle/archive/2026-03-12T18-45Z-auth-implementation-complete.md
```

Contents: model-written summary, chronological [breadcrumbs](audits.md#breadcrumbs), [audit results](audits.md#the-audit-system), and metadata (node path, timestamp, engineer, branch). Archive filenames are unique by construction. Append-only. Merge-conflict-proof.
