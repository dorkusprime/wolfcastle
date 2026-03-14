# ADR-060: Platform-Specific Code via Build Tags

**Status:** Accepted

**Date:** 2026-03-14

## Context

Two subsystems use Unix-only syscalls: the file locking system (`syscall.Flock` in `internal/state/`) and the process group isolation (`syscall.SysProcAttr{Setpgid: true}` in `internal/invoke/`). Both compile fine on macOS and Linux but fail on Windows, breaking cross-compilation.

Wolfcastle targets three platforms (linux, darwin, windows) per ADR-047. Windows support does not need to be feature-complete at this stage, but the binary must compile and run with degraded behavior rather than failing to build.

## Decision

Use Go build tags to provide platform-specific implementations:

### File Locking (`internal/state/`)

| File | Build tag | Behavior |
|------|-----------|----------|
| `filelock.go` | (none) | Platform-independent lock logic: acquire, release, stale detection, WithLock |
| `filelock_unix.go` | `!windows` | `flockExclusive` and `flockUnlock` via `syscall.Flock`. `signalProcess` via `syscall.Signal(0)` |
| `filelock_windows.go` | `windows` | `flockExclusive` and `flockUnlock` are no-ops (return nil). `signalProcess` assumes process is alive |

Windows locking is best-effort. This is acceptable because Wolfcastle's locking is advisory (cooperating processes only), and the single-daemon-per-namespace constraint (enforced by PID file) provides the primary guard against concurrent mutation. The flock is a defense-in-depth layer, not the sole protection.

### Process Group Isolation (`internal/invoke/`)

| File | Build tag | Behavior |
|------|-----------|----------|
| `procattr_unix.go` | `!windows` | Returns `&syscall.SysProcAttr{Setpgid: true}` |
| `procattr_windows.go` | `windows` | Returns `nil` |

On Unix, `Setpgid` puts the child model process in its own process group so SIGTERM propagation from the daemon doesn't leak to the parent shell. On Windows, process group management would require `CREATE_NEW_PROCESS_GROUP` via the Windows API, which is not critical for correctness.

### Pattern

Both subsystems follow the same pattern: platform-independent logic in the main file, platform-specific primitives in `_unix.go` and `_windows.go` files with build tags. The main file calls the primitives through package-level functions (`flockExclusive`, `flockUnlock`, `signalProcess`, `processSysProcAttr`) that are defined in exactly one of the platform files per build.

## Consequences

- Cross-compilation to all five targets (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64) succeeds.
- Windows builds run with degraded locking (no-op) and no process group isolation.
- The degradation is documented and acceptable for the current use case.
- Adding real Windows locking (via `LockFileEx`) later requires only replacing the no-op implementations in `filelock_windows.go`.
