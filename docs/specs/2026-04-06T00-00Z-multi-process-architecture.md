# Multi-Process Architecture

## Status
Draft

## Problem

Wolfcastle enforces a single global daemon lock, allowing only one instance at a time. This conflicts with worktree-heavy workflows where a developer runs multiple branches concurrently. Switching worktrees requires `wolfcastle init --force`, and there's no way to tell which worktree the running daemon belongs to. CLI commands like `wolfcastle status` and `wolfcastle inbox add` have no routing mechanism when multiple instances could exist.

## Design

### Instance registry

Each running daemon registers itself in `~/.wolfcastle/instances/`. One file per instance, named by slugifying the symlink-resolved worktree path (e.g., `users-wild-repository-wolfcastle-feat-auth.json`). The file contains:

```json
{
  "pid": 12345,
  "worktree": "/Users/wild/repository/wolfcastle/feat/auth",
  "branch": "feat/auth",
  "started_at": "2026-04-06T00:00:00Z"
}
```

On startup: resolve the worktree path with `filepath.EvalSymlinks`, slugify it, write the instance file. On clean shutdown: remove it. On read (by any CLI command): check PID liveness, remove stale entries automatically.

No shared registry file. No user-level lock needed for writes. Each instance owns its own file; the only contention is a new daemon in the same worktree overwriting a crashed daemon's stale file, which is the correct behavior.

The slug function converts a resolved path to a filename-safe string: replace path separators with `-`, strip leading `-`, lowercase. Example: `/Users/wild/repository/wolfcastle/feat/auth` becomes `users-wild-repository-wolfcastle-feat-auth.json`.

### Single source of truth for daemon PID

The instance registry replaces the per-worktree PID file (`wolfcastle.pid`). The registry entry contains the PID, worktree path, branch, and start time. All commands that need to know whether a daemon is running (status, stop, start's "already running" check) read the registry, not a local PID file.

The PID file inside `.wolfcastle/system/` is removed. No backward compatibility concern; there are no external users yet.

### CLI routing

When a CLI command needs to talk to a daemon (status, stop, inbox add, log, etc.):

1. If `--instance` is set, use the specified instance directly.
2. Resolve CWD with `filepath.EvalSymlinks`.
3. Scan `~/.wolfcastle/instances/` for live instances whose `worktree` path matches. A match means the resolved CWD equals the worktree path or is a subdirectory of it, with a path separator boundary check (so `/repo/feat/auth` does not match CWD `/repo/feat/auth-v2/src`).
4. If multiple candidates match, use the longest worktree path (most specific match).
5. If exactly one match after longest-prefix: route to it.
6. If zero matches: error with "No running instance found. Start one with `wolfcastle start`."
7. If still ambiguous after longest-prefix (shouldn't happen with the boundary check, but as a safety net): present a selector in interactive mode, or error with the candidate list in non-interactive mode (`--json`, piped stdout, no TTY).

The `--instance` flag is available on commands that talk to a running daemon: start, stop, status, log, inbox. It is not a global persistent flag.

### Stopping instances

`wolfcastle stop` follows the same routing as other daemon commands: CWD match by default, `--instance` for explicit override, selector when ambiguous.

Additionally, `wolfcastle stop --all` stops every live instance in the registry. It iterates the registry, checks PID liveness, and signals each running daemon.

### Per-worktree daemon lock

The daemon lock moves from a global lock in `~/.wolfcastle/` to a per-worktree lock inside `.wolfcastle/`. Two daemons in two worktrees run concurrently without blocking each other. The lock only prevents duplicate daemons in the same worktree.

The `WOLFCASTLE_LOCK_DIR` env var remains as a testing escape hatch.

### Worktree-aware startup

When `wolfcastle start` runs in a directory with a `.wolfcastle/` that has tracked content but missing gitignored tiers:

1. Detect missing tiers (base, local, logs directory).
2. Regenerate the base tier from embedded templates.
3. Generate the local tier with identity only (`config.DetectIdentity()`). Non-identity local overrides are not recovered; users who want persistent overrides across worktrees should use the custom tier, which is tracked by git.
4. Create missing directories (logs, projects namespace).
5. Register in `~/.wolfcastle/instances/`.
6. Start the daemon.

No `init --force` required. The tiers are derived artifacts. `wolfcastle init` remains for first-time repo setup (creating `.wolfcastle/` from scratch in a repo that has never had wolfcastle).

Detection of "worktree with missing tiers" vs "repo that needs init": if `.wolfcastle/` exists (with any tracked content), `start` ensures tiers are populated. If `.wolfcastle/` doesn't exist at all, `start` tells you to run `init`. The git worktree status (`git rev-parse --git-dir` vs `--git-common-dir`) can optionally inform the error message but is not required for the logic.

### Path resolution

All path comparisons use `filepath.EvalSymlinks` to resolve symlinks before comparison or slugification. Both the daemon (on registration) and the CLI (on CWD lookup) resolve to the canonical path first.

## Scope

### In scope
- Instance registry in `~/.wolfcastle/instances/`
- CWD-based routing with longest-prefix matching and selector fallback
- `--instance` flag on daemon-talking commands
- `wolfcastle stop --all`
- Per-worktree daemon lock (replace global lock)
- Remove PID file (registry is single source of truth)
- Worktree-aware tier regeneration on startup (identity-only local tier)
- Stale instance cleanup via PID check
- Non-interactive fallback (error with candidates instead of selector)

### Out of scope
- TUI (uses the registry but is a separate feature)
- Cross-machine coordination (instances are local to one machine)
- Recovering non-identity local config overrides across worktrees

## Affected packages

- `internal/daemon/lock.go`: per-worktree lock, remove global lock
- `internal/daemon/repository.go`: remove PID file operations (WritePID, ReadPID, RemovePID, IsAlive)
- `internal/daemon/daemon.go`: register/deregister on startup/shutdown
- `cmd/daemon/start.go`: worktree-aware tier regeneration, registry integration
- `cmd/daemon/stop.go`: routing via registry, `--all` flag
- `cmd/daemon/status.go`: routing via registry
- `cmd/cmdutil/app.go`: instance resolution from CWD
- New: `internal/instance/` package for registry operations (register, deregister, discover, resolve, clean stale, slug)

## Migration

No external users exist. The global lock file (`~/.wolfcastle/daemon.lock`) and PID file (`wolfcastle.pid`) are removed. The `~/.wolfcastle/instances/` directory is created on first daemon start. Existing `.wolfcastle/` directories in worktrees work without changes; the daemon populates missing tiers automatically on start.
