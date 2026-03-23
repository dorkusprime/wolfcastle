# Coverage Roadmap

Weighted statement coverage: **93.3%** (as of 2026-03-22 audit).
Codecov target: **90%**. All `internal/` packages above 85%. All `cmd/` packages above 65%.

## Category A: Testable today

Functions below 80% that can be covered with straightforward unit tests.

| Function | File | Coverage | Notes |
|---|---|---|---|
| `printNodeTree` | `cmd/daemon/status.go:296` | 63.5% | Missing branches for edge-case node states |
| `nodeGlyph` | `cmd/daemon/status.go:505` | 54.5% | Untested glyph variants |
| `taskGlyph` | `cmd/daemon/status.go:531` | 54.5% | Untested glyph variants |
| `runJSONMode` | `cmd/daemon/follow.go:169` | 66.7% | Missing error path coverage |
| `watchStatus` | `cmd/daemon/status.go:641` | 73.9% | Needs additional state transitions |
| `HasProgress` | `internal/git/git.go:117` | 66.7% | Missing branch for no-diff case |
| `writeBasePrompts` | `internal/project/scaffold_service.go:254` | 75.0% | Missing error-path test |
| `FixWithVerificationRepo` | `internal/validate/fix.go:42` | 69.6% | Needs more fix-scenario tests |
| `WriteTier` | `internal/config/repository.go:207` | 66.7% | Error path not exercised |
| `ApplyMutation` | `internal/config/repository.go:224` | 71.4% | Missing merge-conflict test |

## Category B: Filesystem tricks (chmod-based)

Covered with `os.Chmod` tests on Unix, skipped on Windows via `runtime.GOOS` guards.

| Function | File | Coverage | Notes |
|---|---|---|---|
| `compressFile` | `internal/logging/logger.go:333` | 62.5% | gzip write/close errors |
| `AtomicWriteFile` | `internal/state/io.go:73` | 61.9% | Rename and temp-file errors |
| `signalProcess` | `internal/state/filelock_unix.go:18` | 75.0% | Platform-specific |
| `checkOrphanedTempFiles` | `internal/validate/engine.go:726` | 75.0% | Temp-file cleanup paths |

## Category C: Interface extraction needed

Functions tightly coupled to OS, process, or daemon lifecycle that need mockable
interfaces to fully test.

| Function | File | Coverage | Notes |
|---|---|---|---|
| `newStartCmd` | `cmd/daemon/start.go:23` | 74.1% | Cobra wiring, hard to unit-test fully |
| `checkDirtyTree` | `cmd/daemon/start.go:325` | 19.2% | Needs git interface injection |
| `Run` | `internal/daemon/daemon.go:367` | 78.7% | Long-running loop, tested via integration |
| `commitStateFlush` | `internal/daemon/iteration.go:638` | 16.7% | Deep daemon internals |
| `AcquireGlobalLock` | `internal/daemon/lock.go:43` | 62.5% | File-locking race conditions |

## Category E: Inherently untestable

These functions interact with the OS, terminal, or process lifecycle in ways
that make unit testing impractical or meaningless. Accepted as-is.

| Function | File | Coverage | Reason |
|---|---|---|---|
| `main` | `main.go:14` | 0% | Entry point; calls `cmd.Execute()` |
| `Execute` | `cmd/root.go:121` | 0% | Cobra root execute; tested via integration |
| `confirmContinue` | `cmd/daemon/start.go:367` | 0% | Interactive readline prompt |
| `IsBoolFlag` | `cmd/daemon/follow.go:33` | 0% | Cobra pflag interface method |
| `followJSON` | `cmd/daemon/follow.go:214` | 0% | Real-time log tailing with fsnotify |
| `RunInbox` | `internal/daemon/daemon.go:356` | 0% | Full daemon lifecycle |
| `IsTerminal` | `internal/output/spinner.go:193` | 75% | Checks os.Stdout file descriptor |
| `RestoreTerminal` | `internal/invoke/terminal_unix.go:15` | 66.7% | Raw ioctl syscalls |
