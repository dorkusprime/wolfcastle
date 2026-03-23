# ADR-075: Foreground Process Group Reclaim

## Status
Superseded by ADR-076

## Amendment (2026-03-23)

The `TIOCSPGRP` ioctl described here was never implemented. The problem it addresses (Ctrl+C stops working after model invocations) is solved by a different mechanism in ADR-076:

1. Child processes are placed in their own process group via `Setpgid: true` in `internal/invoke/procattr_unix.go`, preventing them from inheriting the daemon's foreground group in the first place.
2. `RestoreTerminal()` in `internal/invoke/terminal_unix.go` forces the terminal back to cooked mode (ISIG, ICANON, ECHO) after every `cmd.Wait()`, restoring signal delivery regardless of what the child did to terminal settings.

This combination is simpler and more portable than the `TIOCSPGRP` approach. It doesn't require opening `/dev/tty` or issuing ioctls, and it handles the broader class of terminal corruption (not just foreground group issues).

## Date
2026-03-16

## Context

The daemon invokes model CLIs as child processes. Some of these processes (notably Claude Code) take over the terminal's foreground process group during execution. This is normal: the child needs foreground access for interactive I/O. The problem arrives after the child exits.

When a child process that held the foreground group terminates, the terminal's foreground process group should revert to the parent. On most systems this happens automatically. But when the child explicitly sets itself as the foreground group via `tcsetpgrp`, the kernel does not always restore the previous group on exit. The daemon's process group is still running, still has the terminal open, but the terminal no longer considers it the foreground group.

The visible symptom: Ctrl+C stops working after a model invocation. The signal goes to whatever process group the terminal thinks is in the foreground (which no longer exists), not to the daemon. The operator has to background the process, `kill` it manually, or close the terminal. For a tool that runs long autonomous sessions, this is a hard stop.

## Decision

After every child process exit, the daemon reclaims the terminal's foreground process group by issuing a `TIOCSPGRP` ioctl on the controlling terminal's file descriptor. This sets the daemon's process group as the terminal's foreground group, regardless of what the child did during its lifetime.

The reclaim is a single syscall: `ioctl(fd, TIOCSPGRP, &pgrp)` where `fd` is `/dev/tty` and `pgrp` is the daemon's own process group ID. It runs unconditionally after `cmd.Wait()` returns, whether the child succeeded or failed.

If the ioctl fails (no controlling terminal, daemon is running detached, fd is invalid), the error is logged at debug level and ignored. The reclaim is best-effort; sessions without a terminal do not need it.

## Consequences

- Ctrl+C works reliably after model invocations. The daemon's signal handling is restored regardless of what the child process did to the foreground group.
- The fix is platform-specific. `TIOCSPGRP` is a POSIX ioctl available on macOS and Linux. The implementation lives behind a build tag; non-Unix platforms get a no-op.
- The syscall runs after every child exit, adding negligible overhead (one ioctl per model invocation).
- Detached or daemonized sessions (no controlling terminal) silently skip the reclaim. The debug log records the skip for anyone investigating terminal behavior.
