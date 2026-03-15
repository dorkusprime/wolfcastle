# Coverage Roadmap: Path to 100%

Current state: **94.2% local (`go tool cover`), estimated ~90%+ Codecov (line + partial branch)**

Codecov's lower number comes from counting partial branches (e.g., `if err != nil` where the error never fires) as uncovered. To reach 100% on Codecov, we need architectural changes.

Total uncovered: ~230 statements across cmd/ and internal/.

---

## Category A: Testable Today (no changes to production code)

**~111 statements, ~35% of all uncovered code**

These are gaps where the test just hasn't been written yet. Standard unit tests with carefully crafted fixtures.

### High impact (10+ lines each)

| Location | Lines | What's uncovered |
|----------|-------|-----------------|
| `cmd/doctor.go` lines 35-111 | ~20 | LoadRootIndex failure, no-fixable-issues path, model-assisted fix reporting |
| `cmd/spec.go` various | ~22 | RequireResolver guards, MkdirAll/WriteFile/SaveNodeState errors, ReadDir errors, dedup logic |
| `cmd/adr_create.go` lines 49-86 | ~8 | stdin reading, file reading error, MkdirAll/WriteFile errors |
| `cmd/archive_add.go` lines 29-68 | ~10 | RequireResolver, empty node, ParseAddress, MkdirAll/WriteFile errors |
| `cmd/navigate.go` lines 27-46 | ~6 | RequireResolver, LoadRootIndex, ParseAddress, FindNextTask errors |
| `internal/daemon/iteration.go` lines 82-198 | ~12 | Pending-filing skip, invoke error, failure escalation (decomposition + hard cap) |
| `internal/daemon/daemon.go` line 110 | ~1 | Multiple in-progress corruption in selfHeal |

### Medium impact (2-5 lines each)

| Location | Lines | What's uncovered |
|----------|-------|-----------------|
| `cmd/audit/approve.go` lines 67-72 | ~5 | Invalid slug from title, CreateProject error, ParseAddress error |
| `cmd/audit/codebase.go` lines 86-95 | ~3 | Unknown scope, no scopes found |
| `cmd/daemon/status.go` lines 47-49, 186-211 | ~5 | LoadRootIndex error, namespace with broken index, no summaries |
| `cmd/daemon/follow.go` lines 45-47 | ~2 | New iteration header on file change |
| `cmd/unblock.go` lines 239-241 | ~3 | loadUnblockPreamble success path (needs unblock.md fixture) |
| `internal/daemon/propagate.go` lines 16-25 | ~2 | Parse error in loadNode/saveNode callbacks |
| `internal/daemon/retry.go` lines 29-55 | ~4 | Context cancellation before and during retry |
| `internal/daemon/stages.go` various | ~4 | Model not found, invoke error, prompt error in file stage |
| `internal/pipeline/fragments.go` lines 79-81 | ~1 | Include list references missing fragment |
| `internal/pipeline/fragments.go` lines 106-107 | ~1 | Invalid Go template syntax |
| `internal/pipeline/prompt.go` lines 21-38 | ~2 | Skip assembly error, fragment resolution error |
| `internal/validate/fix.go` lines 43-61 | ~2 | LoadRootIndex error in loop, zero fixes break |
| `internal/validate/model_fix.go` lines 47-54 | ~2 | Invoke error, JSON parse error |
| `internal/invoke/invoker.go` lines 126-184 | ~4 | Non-ExitError paths, scanner error |
| `internal/invoke/retry.go` lines 91-92 | ~2 | Context done during backoff |
| `internal/state/filelock.go` lines 67-69 | ~1 | Stale lock cleanup triggers retry |
| `internal/testutil/helpers.go` various | ~3 | MarshalIndent error, ReadFile error, Unmarshal error |
| All `cmd/task/*.go` empty-node guards | ~10 | Belt-and-suspenders `--node ""` guards behind MarkFlagRequired |
| All `cmd/audit/*.go` empty-node guards | ~10 | Same pattern |

---

## Category B: Needs Filesystem Tricks (chmod, read-only dirs)

**~122 statements, ~31% of all uncovered code**

These are `if err != nil { return err }` guards after OS calls (MkdirAll, WriteFile, SaveNodeState, etc.). Testable on Unix using permission manipulation, but each test covers only 1-2 lines.

### Pattern

Every `SaveNodeState`, `SaveRootIndex`, `SaveBatch`, `SaveInbox`, `os.WriteFile`, `os.MkdirAll` call has a 2-line error guard. There are approximately 60 of these across the codebase.

### Approach

Create a test helper:
```go
func withReadOnlyDir(t *testing.T, fn func(dir string)) {
    dir := t.TempDir()
    os.Chmod(dir, 0555)
    t.Cleanup(func() { os.Chmod(dir, 0755) })
    fn(dir)
}
```

Then write tests that set up state, make the target directory read-only, and verify the error is returned. Skip on Windows with `runtime.GOOS == "windows"`.

### Key targets (most lines per test)

| Location | Lines | What's uncovered |
|----------|-------|-----------------|
| `internal/state/io.go` atomicWriteJSON | ~8 | MkdirAll, CreateTemp, Write, Close, Rename errors |
| `cmd/audit/approve.go` lines 98-161 | ~15 | MkdirAll, SaveNodeState, WriteFile, SaveBatch, SaveRootIndex errors |
| `cmd/project/create.go` lines 74-154 | ~16 | LoadNodeState, SaveNodeState, MkdirAll, WriteFile, SaveRootIndex errors |
| `internal/project/scaffold.go` various | ~8 | Write errors for .gitignore, base/config.json, custom/config.json, local/config.json, base dirs |
| `internal/logging/logger.go` compressFile | ~5 | io.Copy error, gz.Close error, various cleanup paths |

---

## Category C: Needs Interface Extraction

**~60 statements, ~15% of all uncovered code**

These are functions that call external systems through concrete types. To test them, the dependency needs to be behind an interface.

| Location | Lines | Dependency | Refactoring needed |
|----------|-------|-----------|-------------------|
| `cmd/audit/codebase.go` runCodebaseAudit | ~50 | `invoke.Invoke` (concrete) | Accept an `invoke.Invoker` parameter or read it from `App` |
| `cmd/doctor.go` model-assisted fixes | ~10 | `validate.TryModelAssistedFix` calls `invoke.Invoke` | Same: inject Invoker |

### Approach

The `invoke.Invoker` interface already exists (`internal/invoke/invoker.go`). The fix:

1. Add an `Invoker invoke.Invoker` field to `cmdutil.App`
2. Initialize it with `invoke.NewProcessInvoker()` in `cmd/root.go`
3. Pass `app.Invoker` to `runCodebaseAudit` and `TryModelAssistedFix`
4. Tests inject a mock invoker that returns canned responses

This single refactoring unlocks 60 lines of coverage and makes the audit and doctor commands fully testable.

---

## Category D: Needs Refactoring (mixed testable logic + untestable I/O)

**~22 statements, ~6% of all uncovered code**

| Location | Lines | Problem | Refactoring needed |
|----------|-------|---------|-------------------|
| `internal/daemon/daemon.go` RunWithSupervisor | ~10 | `time.Sleep(delay)` is not injectable | Add `SleepFunc` field to Daemon (pattern already used in `invoke.RetryInvoker`) |
| `internal/logging/logger.go` WatchForNewFiles | ~5 | `time.After` not injectable | Accept a clock or polling function |
| `cmd/unblock.go` runInteractiveUnblock | ~7 | readline + invoke mixed together | Extract conversation loop from terminal I/O |

### Approach for RunWithSupervisor

```go
type Daemon struct {
    // ... existing fields
    SleepFunc func(time.Duration)  // defaults to time.Sleep
}
```

Tests set `SleepFunc` to a no-op. This makes the restart loop, max-restart detection, and state reset all testable.

---

## Category E: Inherently Untestable

**~77 statements, ~19% of all uncovered code**

These cannot be unit tested by any reasonable means.

| Location | Lines | Why untestable |
|----------|-------|---------------|
| `cmd/root.go` Execute | ~3 | Calls `os.Exit(1)` |
| `cmd/unblock.go` runInteractiveUnblock | ~49 | readline requires a real terminal |
| `cmd/daemon/start.go` startBackground | ~22 | Forks a child process via os.Executable |
| `cmd/daemon/start.go` createWorktree | ~14 | Runs `git worktree add` |
| `cmd/daemon/start.go` cleanupWorktree | ~6 | Runs `git worktree remove` |
| `internal/selfupdate/updater.go` Apply (stub) | ~3 | Dead code by design (stub never reaches update path). This puts the package at 75%, below the 85% per-package threshold, but it's accepted: the uncovered lines are the error-return and post-download paths that only execute when a real update backend replaces the stub. |
| Various `os.FindProcess` error guards | ~5 | Never fails on Unix |
| Various `json.Marshal` on known types | ~3 | Never fails with serializable structs |
| `embed.FS.ReadFile` error guard | ~1 | Compiled-in files never fail to read |
| `filepath.Rel` error guards | ~2 | Never fails on same-volume Unix paths |

### Approach

Accept these as the coverage ceiling. At 77 lines out of ~3600 total, they represent a hard floor of ~97.9% maximum achievable unit test coverage. The remaining ~2.1% can only be covered by integration tests (which don't contribute to the Codecov unit coverage profile).

---

## Blocking Issue

~~Resolved.~~ The duplicate `TestCompressFile_EmptyFile` was renamed to `TestCompressFile_EmptyFile_ErrorPath` in `logger_errorpath_test.go`. No blocking issues remain.

---

## Priority Order (by Codecov impact)

### Phase 1: Fix blocking issue + Category A quick wins
**Status: Complete.** Logging duplicate fixed, Category A tests written. Gained ~3% local coverage.

### Phase 2: Interface extraction for Invoker
**Status: Complete.** `Invoker` field added to `App`, `runCodebaseAudit` and `TryModelAssistedFix` accept it, mock invoker tests written.

### Phase 3: Filesystem error path tests
**Status: Complete.** chmod-based error path tests written across all major Category B items (state/io, logging, scaffold, validate/fix, daemon/stages, audit, project, task, inbox).

### Phase 4: Daemon refactoring
**Estimated gain: +0.5% on Codecov (~96% to ~96.5%)**
- Add SleepFunc to Daemon
- Test RunWithSupervisor restart loop
- Time: 1 hour

### Phase 5: Accept the ceiling
**Maximum achievable: ~97.9% on `go tool cover`, ~95-96% on Codecov**
- The remaining ~77 lines are Category E (os.Exit, readline, process forking, platform guards)
- These can only be covered by integration/E2E tests outside the unit coverage profile
