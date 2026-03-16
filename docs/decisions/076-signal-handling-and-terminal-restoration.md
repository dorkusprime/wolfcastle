# ADR-076: Signal Handling and Terminal Restoration After Model Invocation

## Status
Accepted

## Date
2026-03-16

## Context
Claude Code (and potentially other model CLIs) puts the terminal in raw mode during execution for its readline/input handling. When the child process exits, it may not restore the terminal to cooked mode. In cooked mode, the kernel generates SIGINT when the user presses Ctrl+C. In raw mode, Ctrl+C produces a literal `^C` character and no signal is delivered.

Go's `signal.NotifyContext` catches the first signal and cancels a context. After that, subsequent signals get default handling (process termination). But if no signal is delivered (because the terminal is in raw mode), neither mechanism fires.

Additionally, `signal.NotifyContext` cancels the context on the first signal, and any goroutine waiting on `ctx.Done()` exits. If a backup signal channel's goroutine also listens on `ctx.Done()`, it exits when NotifyContext fires first, leaving nobody to handle subsequent signals.

## Decision

Three-layer defense:

1. **Terminal restoration**: After every child process exits, explicitly set `ISIG | ICANON | ECHO` on the terminal via `SYS_IOCTL` with platform-specific constants (`TIOCGETA`/`TIOCSETA` on macOS, `TCGETS`/`TCSETS` on Linux). This ensures the terminal generates signals from Ctrl+C regardless of what the child did.

2. **Backup signal channel**: A dedicated `signal.Notify` channel runs alongside `signal.NotifyContext`. Its goroutine listens on `d.shutdown` (not `ctx.Done()`) so it survives after NotifyContext fires. This catches signals that NotifyContext missed.

3. **Spinner Stop() timeout**: The spinner's `Stop()` method waits for the animation goroutine to exit, but the goroutine may be blocked in `fmt.Fprintf` to stdout. A 200ms timeout prevents `Stop()` from blocking the shutdown path indefinitely.

## Consequences

- Ctrl+C works reliably after any number of model invocations.
- The terminal is always in cooked mode between invocations.
- New dependency on `golang.org/x/term` is avoided; raw syscalls use only `syscall` and platform-specific ioctl constants.
- The 200ms spinner timeout means the animation line may not always be cleanly erased on rapid shutdown. Acceptable tradeoff.
- Platform-specific files: `terminal_unix.go` (shared logic), `terminal_darwin.go` and `terminal_linux.go` (ioctl constants), `terminal_windows.go` (no-op).
