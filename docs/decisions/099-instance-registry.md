# File-per-instance registry

## Status
Accepted

## Date
2026-04-06

## Context

With per-worktree daemon locks, there is no single file that answers "which wolfcastle daemons are running?" CLI commands like `status`, `stop`, and `inbox add` need to discover the right daemon to talk to when multiple instances exist.

## Options Considered

1. **Shared registry file.** One JSON file listing all instances. Simple to read, but requires locking for concurrent writes (two daemons starting simultaneously in different worktrees).

2. **File per instance.** Each daemon creates its own file in `~/.wolfcastle/instances/`, named by a slug of the worktree path. No write contention: each daemon writes only its own file. Discovery reads the directory listing.

3. **Socket per instance.** Each daemon opens a Unix domain socket. CLI commands connect to communicate. More complex, enables bidirectional communication, but adds network programming and platform concerns (Windows named pipes).

## Decision

File per instance. Each daemon writes `~/.wolfcastle/instances/<slug>.json` on startup and removes it on shutdown. The slug is derived from the symlink-resolved worktree path (path separators replaced with `-`, lowercased) to ensure deterministic filenames regardless of how the worktree is accessed.

Stale entries (crashed daemons) are detected by PID liveness check on read and cleaned automatically. A new daemon in the same worktree overwrites the crashed daemon's file, which is correct behavior.

## Consequences

- No write contention between concurrent daemon startups.
- Discovery is a directory listing plus PID liveness check per entry.
- The registry is the single source of truth for daemon PID, replacing the per-worktree PID file (`wolfcastle.pid`).
- Stale entries persist until a wolfcastle command reads the registry. This is acceptable; the cleanup cost is one `kill -0` per entry, and the entries are small files.
- The `~/.wolfcastle/instances/` directory is created on first daemon start.
