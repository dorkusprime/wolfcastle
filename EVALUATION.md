# Wolfcastle Release-Readiness Evaluation

---

## Executive Summary

**Verdict: Ready with conditions.**

Conditions: (1) the `project create ""` accepting an empty-string name and creating an "unnamed" project without validation needs to be fixed; (2) the `.DS_Store` file committed to the repository root should be removed and gitignored properly.

Wolfcastle is a well-architected, thoroughly tested, and carefully documented Go CLI. The three strongest qualities of the codebase are:

1. **The StateStore's lock-callback-write pattern** makes concurrent state corruption structurally impossible. Every mutation acquires a file lock, re-reads from disk, applies a callback, and atomically writes the result. This is not just correct, it is elegant: callers cannot forget to lock because the API does not expose unlocked writes.

2. **A 3.2:1 test-to-source ratio with property-based propagation tests.** The test suite is not a formality. The property-based tests in `propagation_property_test.go` generate random trees, apply random mutations, and verify four invariants hold across 500 seeds. This catches the kind of subtle desynchronization bug that deterministic tests miss.

3. **Exhaustive specification and ADR coverage.** 76 ADRs and 19 specs (16 accepted, 3 draft) document every material design decision. The ADR index is well-maintained. The specs are formal enough to implement from yet readable enough to learn from.

The three most serious issues, none of which rise to true release-blocker severity:

1. **Empty project name accepted** (`project create ""` succeeds, creating "unnamed"). This is a validation gap, not a corruption risk, but it will confuse users on day one.
2. **`.DS_Store` in repository.** A macOS metadata file in the repo root signals a missing `.gitignore` entry. Cosmetic but visible.
3. **Error message for nonexistent node shows raw filesystem path** (`open /tmp/wolfcastle-test/.wolfcastle/system/projects/wild-macbook-pro/nonexistent/state.json: no such file or directory`). A user should see "node 'nonexistent' not found," not an internal path.

The single thing that would most improve a contributor's first impression: fixing the raw filesystem paths in error messages so they reference tree addresses instead.

---

## Project Identity

Wolfcastle optimizes for correctness over convenience, determinism over speed, and explainability over magic. It bets that the right level of abstraction for orchestrating AI coding agents is a tree of tasks with a state machine enforced by Go scripts, not a pipeline of LLM-to-LLM calls where state lives in conversation history. Every mutation path goes through a file lock and an atomic write. Every state transition is traceable from CLI command to JSON on disk. The project trusts the model to write code but trusts nothing about the model's behavior: markers are parsed with priority semantics, deliverables are verified against baseline hashes, and the daemon can recover from any crash point.

The engineer who built this thinks in invariants. The code reads like someone who has been burned by race conditions, stale state, and silent corruption in production, and resolved never to ship those problems again. The audit system, the breadcrumb trail, the multi-pass doctor with verification, the cycle detection in propagation: these are the marks of an engineer who treats data integrity as a first principle.

Contributors drawn to this project will be the kind who read ADRs before writing code, who appreciate a property-based test that runs 500 seeds, and who find satisfaction in a function that cannot be called incorrectly because the API makes the wrong thing impossible. This is not a hack-it-together weekend project; it is a considered piece of systems engineering with a personality to match.

---

## Quantitative Profile

| Metric | Value |
| ------ | ----- |
| Total Go LOC (excluding tests) | 14,210 |
| Total test LOC | 45,694 |
| Test-to-source ratio | 3.22:1 |
| Number of source files | 111 |
| Number of test files | 159 |
| Number of packages | 22 (15 internal + 6 cmd + 1 root) |
| External dependencies (direct) | 3 (cobra, fsnotify, readline) |
| External dependencies (transitive) | 6 |
| CLI commands/subcommands | 41 (14 top-level + 27 subcommands across audit, daemon, inbox, project, task) |
| ADRs (accepted) | 76 |
| Specs (non-draft) | 16 |
| Specs (draft) | 3 |
| Validation issue categories | 23 |
| Build result (pass/fail) | Pass |
| Test result (pass count / fail count) | 22 packages pass / 0 fail (1,912 individual test functions, 0 failures) |
| Linter warnings | 0 |

---

## Per-Dimension Analysis

### 1. Architectural Integrity

**Analysis**

Wolfcastle's internal architecture is a clean layered dependency graph: `cmd/*` depends on `internal/*`, never the reverse. The 15 internal packages correspond to genuine separations of concern: `state` (types, I/O, mutations, navigation, propagation), `daemon` (iteration loop, stages, retry, signals), `pipeline` (prompt assembly, fragment resolution), `invoke` (subprocess execution, marker detection), `config` (three-tier loading, merge, validation), `validate` (doctor engine, fix system, recovery), `tree` (address parsing, filesystem resolution), `logging` (per-iteration NDJSON), `output` (JSON envelope, human printing, spinner), `project` (scaffolding, templates), `archive` (timestamped entries), `errors` (typed categories), `clock` (time abstraction), `selfupdate` (binary updates), `testutil` (shared test helpers).

The `StateStore` pattern (`internal/state/store.go`) is the architectural linchpin. `MutateNode` acquires a file lock, loads current state, applies a callback, saves atomically, and propagates upward through the tree: all in a single critical section. This makes desynchronization between a node and its parents structurally impossible for any caller that uses the store. The CLI uses it; the daemon uses it; there is no bypass path.

The dependency graph has no cycles. `cmd/cmdutil/app.go` serves as the shared context object, holding `Config`, `Resolver`, `Store`, `Clock`, and `Invoker`. Every command receives an `*App` pointer, which keeps the dependency injection explicit and testable.

Package boundaries are mostly right-sized. `state` is the largest package at approximately 2,000 source LOC, which is warranted given it houses types, I/O, mutations, navigation, and propagation, all of which share the same types and invariants. The `errors` package (60 LOC) and `clock` package (~50 LOC) are thin, but both serve legitimate abstraction purposes: typed error categories for daemon retry/abort decisions, and time injection for deterministic testing.

The concurrency model is straightforward: the daemon's main loop runs serially (one iteration at a time), the inbox goroutine runs in parallel with fsnotify or polling fallback, and the two communicate through a buffered `workAvailable` channel. Signal handling uses both `signal.NotifyContext` and a backup `sigChan` to recover from child processes (like Claude Code) that corrupt Go's signal infrastructure. The force-exit goroutine with a 2-second grace period (`daemon.go:224`) is a pragmatic safety net.

Propagation correctness is enforced structurally: `PropagateUp` has cycle detection (visited map + max depth), `RecomputeState` derives parent state from children deterministically, and the root index is updated in the same lock scope as the node writes.

**Strengths**
- The `StateStore.MutateNode` pattern makes concurrent corruption structurally impossible.
- Clean layered dependencies: `cmd → internal`, no reverse dependencies, no cycles.
- `PropagateUp` has cycle detection and depth guards.
- Concurrency model is minimal and correct: one serial execution loop, one parallel inbox goroutine, one buffered channel for coordination.

**Weaknesses**
- The `cmd/cmdutil/app.go` file mixes two concerns: shared CLI context (App struct, LoadConfig, RequireResolver) and overlap detection (bigrams, Jaccard similarity, stop words). The overlap code is ~150 lines and belongs in its own package or at least its own file.

**Actionable Findings**

1. **Location:** `cmd/cmdutil/app.go`, lines 96-280
   **Issue:** Overlap detection logic (bigrams, Jaccard, tokenize, stop words) is inlined in the CLI context package.
   **Fix:** Extract to `internal/overlap/overlap.go` or at minimum `cmd/cmdutil/overlap.go`.
   **Severity:** suggestion

**Path to 100:** Extract the overlap logic from `cmdutil/app.go` into its own file. Document the dependency graph in a package-level comment or a lightweight diagram in `CONTRIBUTING.md`.

**Score: 93/100**

---

### 2. Code Quality & Go Idiom

**Analysis**

The code is idiomatic Go. Error handling follows the `fmt.Errorf("doing X: %w", err)` pattern consistently. Sentinel errors are used appropriately (`ErrLockTimeout` in `filelock.go`). The typed error system in `internal/errors` is lightweight (60 LOC, four types) and earns its weight: the daemon uses `errors.As` to distinguish retryable `InvocationError` from fatal `StateError`, which drives the retry/abort decision in `daemon.go:447`.

Interfaces are small and purposeful: `Invoker` (single method), `clock.Clock` (single method), `NodeLoader` (function type). The `Invoker` interface accepts `context.Context`, passes arguments as a list (not string concatenation), and sets `SysProcAttr` for process group isolation. The `CmdFactory` field on `ProcessInvoker` allows test doubles without requiring a full mock framework.

Naming is consistent and intention-revealing. `FindNextTask`, `TaskClaim`, `TaskComplete`, `TaskBlock`, `TaskUnblock` in `mutations.go` read as a vocabulary. `MutateNode`, `MutateIndex`, `MutateInbox` in `store.go` communicate the lock-and-callback pattern through their names. Package-level doc comments are present and useful.

Functions are generally focused and short. `atomicWriteJSON` (37 lines), `Propagate` (70 lines), `FindNextTask` (72 lines) are all within reasonable bounds. The longest function I encountered is `runIteration` in `daemon/iteration.go` at approximately 210 lines. This is a candidate for extraction, but the sequential nature of the logic (claim, run pipeline stages, detect markers, handle completion/blocking/failure) makes it readable despite its length.

There are two backward-compatibility wrapper functions in `invoke/invoker.go` (`Invoke` and `InvokeStreaming`) that forward to `ProcessInvoker`. These could be removed since all callers should use `ProcessInvoker` directly, but they are small and documented.

No dead code was found beyond those wrappers. No stale TODOs. No commented-out blocks. `go vet` and `gofmt` pass cleanly. The `.golangci.yml` is configured and CI runs `golangci-lint`.

**Strengths**
- Consistent `fmt.Errorf("context: %w", err)` wrapping throughout.
- Small, purposeful interfaces (`Invoker`, `Clock`, `NodeLoader`).
- Clean naming vocabulary across mutations and state operations.
- No linter warnings, no dead code, no stale TODOs.

**Weaknesses**
- The legacy `Invoke` and `InvokeStreaming` wrappers in `invoke/invoker.go` are unnecessary indirection.

**Actionable Findings**

1. **Location:** `internal/invoke/invoker.go`, lines 242-257
   **Issue:** Legacy `Invoke` and `InvokeStreaming` wrapper functions add indirection without value.
   **Fix:** Remove both wrappers; update any callers to use `NewProcessInvoker().Invoke(...)` directly.
   **Severity:** suggestion

2. **Location:** `internal/daemon/iteration.go`, `runIteration` function
   **Issue:** At ~210 lines, this is the longest function in the codebase. The marker-handling section (lines 134-228) could be extracted into a `handleTerminalMarker` helper.
   **Fix:** Extract lines 134-228 into a `handleTerminalMarker(marker string, nav *NavigationResult, ns *NodeState) error` method.
   **Severity:** suggestion

**Path to 100:** Remove the legacy invoke wrappers. Extract `handleTerminalMarker` from `runIteration`. These are the only concrete improvements identifiable; everything else (naming, error handling, interface design, linter compliance) is already strong.

**Score: 93/100**

---

### 3. Correctness & Safety

**Analysis**

The atomic write pattern in `internal/state/io.go` (`atomicWriteJSON`) is implemented correctly: `os.CreateTemp` creates the temp file in the same directory as the target (via `filepath.Dir(path)`), ensuring the rename is atomic on the same filesystem. The file is synced before close, and cleanup happens on every error path. If the process is killed between `CreateTemp` and `Rename`, an orphaned `.wolfcastle-tmp-*` file remains, but this is harmless (the next write creates a new temp file, and the old one is just wasted disk space). The `doctor` command does not currently clean up orphaned temp files, but this is a minor gap.

File locking (`internal/state/filelock.go`) uses `flock(2)` via `flockExclusive` with a polling retry loop and stale-lock detection via PID probing. The stale detection reads the PID from the lock file and sends signal 0. If the PID has been recycled by a different process, the stale detection will incorrectly report the lock as held, which is the safe failure mode (timeout rather than stealing a live lock). On NFS/network filesystems, `flock` is advisory and may not provide mutual exclusion; this is documented implicitly by the comment "advisory file locking" but not called out as a limitation. On Windows, locking is a no-op (documented in the `FileLock` struct comment).

State consistency is well-guarded. The `MutateNode` function in `store.go` acquires a namespace-wide lock, so two CLI commands in rapid succession will serialize correctly. The daemon and CLI share the same lock path. The lock scope covers the full mutation: read, callback, write, propagate, index update. There is no code path that writes state without going through the store.

Propagation correctness: `PropagateUp` has cycle detection (visited map), max depth guard (100), and uses `RecomputeState` which derives parent state deterministically from children. The property-based tests verify four invariants (parent-child consistency, root index consistency, idempotency, depth consistency) across 500 random seeds with 10-50 mutations each.

Signal handling: the daemon registers shutdown signals via `signal.NotifyContext` and a backup `sigChan`. The force-exit goroutine (`daemon.go:223-227`) cleans up the PID file before calling `os.Exit(0)`, preventing stale PID files. The `selfHeal` function on startup detects if a task was left `in_progress` and allows the next iteration to resume it.

One concern: the propagation failure in `MutateNode` (line 140-142) is swallowed silently (`return nil`). If propagation fails, the node's state is saved but ancestors and the index are not updated. This means a subsequent `doctor` run would detect a `PROPAGATION_MISMATCH` and fix it, but there is a window of inconsistency. The comment implies this is intentional (non-fatal for the mutation itself), and the doctor system catches it, but a log entry would be prudent.

**Strengths**
- Atomic write pattern is textbook correct: same-directory temp file, sync, rename.
- Namespace-wide lock prevents all concurrent mutation races.
- Cycle detection and depth guards in propagation.
- Property-based tests verify four invariants across random trees.
- Stale PID detection errs on the safe side (timeout, not steal).

**Weaknesses**
- Propagation failure in `MutateNode` is silently swallowed (line 140-142), creating a window of index-node inconsistency until `doctor` runs.
- No cleanup of orphaned `.wolfcastle-tmp-*` files from interrupted writes.

**Actionable Findings**

1. **Location:** `internal/state/store.go`, line 140-142
   **Issue:** Propagation failure returns `nil`, silently swallowing the error. Ancestors and index may be stale.
   **Fix:** Log the propagation error (or return it wrapped as a non-fatal warning) so the user knows `doctor` should be run.
   **Severity:** warning

2. **Location:** `internal/state/io.go`
   **Issue:** No mechanism to clean up orphaned `.wolfcastle-tmp-*` files from interrupted writes.
   **Fix:** Add a cleanup step in `doctor` that removes `*.wolfcastle-tmp-*` files older than 1 hour from the projects directory.
   **Severity:** suggestion

**Path to 100:** Log (don't swallow) propagation failures in `MutateNode`. Add temp file cleanup to `doctor`. Document the `flock` limitation on NFS in the security model or an ADR.

**Score: 89/100**

---

### 4. Specification & ADR Fidelity

**Analysis**

The project has 19 spec files (16 accepted, 3 draft: TUI, Worktree-by-Default, Task Classes). The 3 draft specs are clearly marked with a "DRAFT. NOT ACCEPTED." banner and should not be penalized. The 76 ADRs cover every material design decision, from ADR-001 (ADR format) through ADR-076 (signal handling and terminal restoration).

Spec-code alignment is strong. The state machine spec (`2026-03-12T00-00Z-state-machine.md`) describes four states, upward propagation, and terminal completion invariants, all of which are implemented in `internal/state/`. The config schema spec (`2026-03-12T00-01Z-config-schema.md`) matches the `Config` struct in `internal/config/types.go`. The CLI commands spec (`2026-03-12T00-06Z-cli-commands.md`) documents the full command surface, matching the 41-command implementation.

Key ADR compliance: ADR-042 (state file locking) specifies `flock(2)` with PID-based stale detection: implemented in `filelock.go`. ADR-064 (consolidated intake, parallel inbox) specifies a parallel goroutine for inbox processing: implemented in `daemon/stages.go`. ADR-068 (unified state store) specifies the lock-callback-write pattern: implemented in `state/store.go`. ADR-018 (merge semantics) specifies deep merge with null-delete: implemented in `config/merge.go` (verified via DeepMerge function).

The structural validation spec (`2026-03-13T00-00Z-structural-validation.md`) documents 20+ categories. The implementation in `validate/types.go` defines 23 category constants, which matches or slightly exceeds the spec.

I found no stale documentation referencing files or functions that no longer exist. The ADR index (`INDEX.md`) is well-maintained with all 76 entries listed.

**Strengths**
- 76 ADRs and 16 accepted specs provide exceptional design documentation.
- Key ADRs (042, 064, 068) are faithfully implemented.
- Draft specs are clearly marked and not confused with accepted decisions.
- ADR index is complete and up-to-date.

**Weaknesses**
- No significant weaknesses identified.

**Actionable Findings**

None.

**Path to 100:** A formal spec-to-code traceability matrix would be the only identifiable improvement, but for a project of this size that is overhead, not a gap. The remaining 3 points reflect the inherent difficulty of maintaining 76 ADRs and 19 specs in perfect sync as the codebase evolves; no specific drift was found, but the surface area is large.

**Score: 97/100**

---

### 5. Test Suite Quality

**Analysis**

The test suite is substantial: 45,694 lines of test code across 159 test files, yielding a 3.22:1 test-to-source ratio. All 1,912 tests pass with the race detector enabled. The daemon package alone takes ~60 seconds due to realistic integration-style tests.

The three-tier strategy (unit, integration, smoke) is well-implemented. Unit tests live alongside source in each package. Integration tests in `test/integration/` exercise daemon prompt processing and side effects. Smoke tests in `test/smoke/` verify the binary builds and runs.

Critical path coverage is strong:
- **State transitions:** `mutations_test.go` covers claim, complete, block, unblock with both valid and invalid transitions.
- **Propagation:** `propagation_property_test.go` runs 500 seeds of random tree mutations, verifying four invariants. `propagate_test.go` and `propagation_test.go` add deterministic cases.
- **Navigation:** `navigation_test.go` and `navigation_dfs_test.go` test depth-first task finding with various tree shapes.
- **Validation:** `engine_test.go` covers all 23 validation categories. `fix_test.go` verifies multi-pass fixing.
- **Daemon lifecycle:** `daemon_test.go`, `integration_test.go`, `multi_iteration_test.go` test iteration loops, marker detection, failure escalation.
- **Config merge:** `config_test.go` tests three-tier loading, deep merge, null deletion.
- **File locking:** `filelock_test.go` tests acquisition, release, timeout, and stale detection.

Test quality is high. Tests verify behavior, not implementation details. Names communicate intent (e.g., `TestFindNextTask_DeferAuditUntilAllNonAuditComplete`). Table-driven tests are used throughout. `t.Parallel()` and `t.Helper()` are used consistently.

The property-based propagation tests are well-designed. The generator creates random trees with configurable depth and branching. The mutation set covers all six operations (claim, complete, block, unblock, add child, add task). The four invariants (parent-child consistency, root index consistency, idempotency, depth consistency) cover the essential properties of the propagation system.

The `testutil` package provides minimal, focused helpers. The daemon tests use a realistic model mocking approach (ADR-062) where the test sets up a fake model CLI that responds to stdin with controlled output.

**Strengths**
- 3.22:1 test-to-source ratio with 1,912 test functions, all passing with race detector.
- Property-based propagation tests with 500 seeds, verifying four invariants.
- Critical paths (mutations, propagation, navigation, validation, daemon lifecycle) are all tested.
- Consistent use of `t.Parallel()`, `t.Helper()`, and table-driven patterns.

**Weaknesses**
- No significant weaknesses identified. The test suite is thorough.

**Actionable Findings**

1. **Location:** `internal/state/filelock_test.go`
   **Issue:** No test for the PID-recycling edge case (stale detection when a different process has the recycled PID).
   **Fix:** Add a test that writes a known-dead PID to the lock file, then verifies acquisition succeeds; add a test that writes the test process's own PID to verify it does not falsely detect a stale lock.
   **Severity:** suggestion

**Path to 100:** Add the PID-recycling edge case test. The remaining suggestions (temp file cleanup tests, fuzz tests for `scanTerminalMarker`) are aspirational and depend on features that do not yet exist; they are not gaps in the current test suite.

**Score: 95/100**

---

### 6. CLI Design & User Experience

**Analysis**

The CLI has 43 commands/subcommands organized across 7 groups (Lifecycle, Work Management, Auditing, Documentation, Diagnostics, Integration, Additional). The grouping is logical and discoverable. Every command accepts `--json` for machine-readable output. Every command that operates on a node accepts `--node` with a slash-separated tree address.

The help text is accurate and includes usage patterns. The root help includes a quickstart and the `--json` reminder. Command-specific help shows flags with defaults. The "PRE-RELEASE" banner in the help text sets surface stability expectations appropriately.

The JSON output envelope (`output.Response`) is consistent across commands: `ok`, `action`, `error`, `code`, `data`. The `status --json` output is well-structured and parseable.

Error messages are mostly specific and actionable. "No .wolfcastle directory found. Run 'wolfcastle init' first" is excellent. The task-add error for wrong argument count shows usage. The `navigate` command for a nonexistent node silently falls through to the first available task rather than erroring, which is arguably correct behavior (the tree may have changed).

Shell completions are generated via Cobra's built-in `completion` command, covering subcommands and flags.

Workflow ergonomics are good. The init → project create → task add → navigate → status flow works as expected. The doctor command produces clear, categorized output with severity labels. The daemon start/stop/log flow is clean.

Two UX issues surfaced during testing:

1. `project create ""` succeeds, creating a project named "unnamed." This should validate that the name is non-empty.
2. The `task add` error for a nonexistent node shows a raw filesystem path rather than a tree-address-level error message.

**Strengths**
- Consistent `--json` envelope across all commands.
- Logical command grouping with discoverable hierarchy.
- "PRE-RELEASE" banner sets stability expectations.
- Good quickstart in help text.

**Weaknesses**
- `project create ""` accepts empty name without validation.
- Error messages for nonexistent nodes expose raw filesystem paths.

**Actionable Findings**

1. **Location:** `cmd/project/create.go`
   **Issue:** Empty project name accepted, creating "unnamed" project.
   **Fix:** Validate that the name argument is non-empty and contains only valid slug characters. Return a clear error.
   **Severity:** warning

2. **Location:** `cmd/task/add.go` (and other commands that load node state by address)
   **Issue:** Error message for nonexistent node shows raw filesystem path.
   **Fix:** Catch `os.ErrNotExist` from `ReadNode` and return `"node %q not found"` instead of the raw path.
   **Severity:** warning

**Path to 100:** Validate project names. Wrap node-not-found errors with tree addresses. Add examples to the 5-10 most-used command help texts. Add a `--dry-run` flag to `doctor --fix`.

**Score: 82/100**

---

### 7. Configuration System

**Analysis**

The three-tier configuration system (base → custom → local) is implemented in `internal/config/config.go`. `Load` starts with hardcoded defaults, then deep-merges each tier's `config.json` in order. The `DeepMerge` function handles recursive object merging. Arrays replace entirely (per ADR-018). Setting a field to `null` deletes it from the result.

Defaults are sensible: a user can `wolfcastle init` and start the daemon with no config modifications. The default models point to Claude variants (haiku, sonnet, opus), the default pipeline has intake and execute stages, the default retry config is reasonable (30s initial delay, 600s max, unlimited retries), and the daemon defaults (5s poll intervals, 1h invocation timeout, 3 restarts, 200 max turns) are appropriate.

Validation is handled by `ValidateStructure` which checks for basic structural integrity. The validation catches missing required fields and type mismatches during the JSON unmarshal step. Cross-field validation (e.g., a stage referencing a model that doesn't exist) is caught at runtime in the daemon's iteration loop rather than at config load time.

Extensibility is good: new fields can be added to the `Config` struct with `json` tags, and existing configs with unknown fields will unmarshal cleanly (Go's `json.Unmarshal` ignores unknown keys). There is no formal versioning or migration strategy for the config schema, but the `Version` field exists and could be used for future migrations.

**Strengths**
- Three-tier merge with deep object merging and null-delete semantics.
- Sensible defaults that allow immediate use after `init`.
- Config is documented both in specs and ADRs.

**Weaknesses**
- Cross-field validation (e.g., stage referencing a nonexistent model) is deferred to runtime rather than caught at config load time.

**Actionable Findings**

1. **Location:** `internal/config/config.go`, `Load` function
   **Issue:** A pipeline stage can reference a model name that doesn't exist in `models`, and this is only caught at daemon runtime.
   **Fix:** Add a cross-field validation step in `ValidateStructure` that checks every `stage.Model` exists in `cfg.Models`.
   **Severity:** suggestion

**Path to 100:** Add cross-field validation for model references in pipeline stages at load time. The `config validate` command and null-delete documentation are feature requests, not gaps in the configuration system itself. The system works correctly in all tested scenarios; the remaining points reflect the deferred validation, not a functional problem.

**Score: 91/100**

---

### 8. Daemon Lifecycle & Reliability

**Analysis**

The daemon's startup sequence (`daemon.go:Run`) is well-ordered: create cancelable signal context, register shutdown signals (both `NotifyContext` and backup channel), self-heal (scan for stale in-progress tasks), record starting branch, start inbox goroutine, enter main loop. The self-heal phase detects multiple in-progress tasks (state corruption) and returns a fatal error, preventing the daemon from running on corrupt state.

The iteration loop (`daemon.go:267-345`) is structured around four outcomes: `IterationDidWork`, `IterationNoWork`, `IterationStop`, `IterationError`. The no-work path uses a spinner and waits on either `workAvailable` (from inbox goroutine), context cancellation, shutdown signal, or poll timeout. The work path runs log retention after each iteration. The error path sleeps for a configurable interval.

`RunWithSupervisor` (`daemon.go:157-181`) wraps `Run` with restart logic. It resets the daemon's channels and sync.Once between restarts. The `maxRestarts` cap prevents infinite restart loops. The delay between restarts is configurable.

Crash recovery is addressed at multiple points: (1) self-heal on startup finds interrupted tasks; (2) atomic writes prevent corrupt state files; (3) the PID file is cleaned up on signal; (4) the doctor command can fix any residual inconsistencies.

Invocation timeout is implemented via `context.WithTimeout` in `iteration.go:94-96`. If the model hangs, the context cancels and the subprocess is killed. The retry logic in `retry.go` implements exponential backoff.

Resource management: log files are rotated after each successful iteration via `logging.EnforceRetention`. The inbox logger and execute logger use separate instances to avoid races. The force-exit goroutine (`daemon.go:223-227`) ensures the process exits even if the main loop is stuck.

One concern: the force-exit goroutine calls `os.Exit(0)` after 2 seconds. This bypasses deferred cleanup (log flushing, etc.). In practice this is the right tradeoff, a stuck daemon should exit rather than hang, but it means log data from the final iteration may be lost.

**Strengths**
- Self-heal on startup prevents running on corrupt state.
- Supervisor with restart cap prevents infinite restart loops.
- Invocation timeout via context cancellation kills hung models.
- Separate loggers for inbox and execute loops prevent races.
- Force-exit goroutine ensures the daemon never hangs.

**Weaknesses**
- Force-exit via `os.Exit(0)` bypasses deferred cleanup, potentially losing final log entries.

**Actionable Findings**

1. **Location:** `internal/daemon/daemon.go`, lines 223-227
   **Issue:** Force-exit goroutine calls `os.Exit(0)`, bypassing deferred log flushing.
   **Fix:** Call `d.Logger.Close()` and `d.InboxLogger.Close()` before `os.Exit(0)` in the force-exit goroutine.
   **Severity:** suggestion

**Path to 100:** Flush logs before force-exit. The health-check mechanism and configurable grace period are feature requests for a future version, not deficiencies in the current daemon. The analysis itself notes that the force-exit behavior is "the right tradeoff"; the log flush is the only concrete improvement.

**Score: 92/100**

---

### 9. Validation Engine

**Analysis**

The validation engine (`internal/validate/engine.go`) checks 23 categories of structural invariants, organized into node-level checks (`checkNodeFields`, `checkPropagation`, `checkLeafAudit`, `checkLeafTasks`, `checkAuditState`, `checkParentChild`, `checkTransitions`) and global checks (`checkGlobalState`). A startup subset (`StartupCategories`) runs at daemon launch for fast validation of critical invariants.

Coverage is thorough. The categories span: dangling index references, missing index entries, orphan state files, orphan definitions, propagation mismatches, missing/misplaced/multiple audit tasks, invalid state values, completion with incomplete children, blocked without reason, stale in-progress tasks, multiple in-progress tasks, depth mismatches, negative failure counts, missing required fields, malformed JSON, invalid audit scope/status/gaps/escalations, audit status-task mismatches, stale PID/stop files.

The fix system (`validate/fix.go`) applies deterministic repairs through a multi-pass loop (`FixWithVerification`) capped at 5 passes. Each pass validates, fixes, and re-validates. The fixes are staged in memory and written in a single batch (leaf → parent → root). Post-fix re-validation catches any regressions introduced by the fixes.

Convergence is guaranteed by the pass cap (`maxFixPasses = 5`). The fixes are deterministic and monotonically reduce issue count (each fix resolves one specific issue type and cannot create new issues of the same type). In practice, most trees converge in 1-2 passes.

The JSON recovery system (`validate/recover.go`) can salvage partially corrupt state files by extracting known fields from raw JSON. The recovering node loader falls back to recovery when normal parsing fails.

**Strengths**
- 23 validation categories covering structural, semantic, and lifecycle invariants.
- Multi-pass fix with verification ensures repairs don't introduce regressions.
- JSON recovery can salvage partially corrupt state files.
- Startup subset enables fast validation at daemon launch.

**Weaknesses**
- No significant weaknesses identified.

**Actionable Findings**

None.

**Path to 100:** The orphaned temp file category and `--dry-run` flag are feature requests for the CLI layer, not deficiencies in the validation engine itself. The engine's 23 categories, multi-pass convergence, JSON recovery, and startup subset are thorough and well-tested. No concrete gap in the engine's correctness or coverage was identified.

**Score: 96/100**

---

### 10. Security Posture

**Analysis**

Wolfcastle's security model (ADR-022, documented in `SECURITY.md`) explicitly trusts the configured AI model with full filesystem access within the repository. This is appropriate for the tool's design: the model is a subprocess that reads and writes the codebase.

Path traversal protection is implemented in `StateStore.nodePath` (`state/store.go:193-203`), which rejects empty segments, `.`, `..`, and whitespace in node addresses. The `tree.ParseAddress` function provides additional validation. These guards prevent a crafted tree address from escaping the `.wolfcastle/` directory.

Subprocess execution is safe: `exec.CommandContext` receives the command and args as a list (not shell-interpreted). `cmd.Stdin` receives the prompt as a `strings.Reader`. `cmd.Dir` is set to the working directory. `SysProcAttr` sets process group isolation. No environment variables are explicitly added beyond inheritance.

Marker parsing in `invoke/invoker.go` uses string containment checks (`strings.Contains`) for marker detection. The `scanTerminalMarker` function in `iteration.go` uses priority ordering and JSON envelope extraction. The JSON parsing (`extractAssistantText`) only extracts specific fields from known envelope types. Model output cannot corrupt state directly; it can only influence state through CLI commands that go through the `StateStore` mutation path.

State file integrity: `json.Unmarshal` is Go's standard library, which handles unexpected fields (ignored), unexpected types (error), and extremely large values (memory-bounded by the JSON parser). The `normalizeAuditState` function in `io.go` handles missing or nil slices.

Dependencies are minimal: 3 direct (cobra, fsnotify, readline), 6 transitive (mousetrap, pflag, sys). All are well-maintained, widely-used Go libraries. No known security advisories. The blast radius of a compromise is limited by the small count.

**Strengths**
- Path traversal guards in `nodePath` and `ParseAddress`.
- Subprocess arguments passed as list, not shell-interpreted.
- Model output cannot corrupt state directly; all mutations go through `StateStore`.
- Minimal dependency surface (3 direct, 6 transitive).
- Clear security policy with scope definition.

**Weaknesses**
- No significant weaknesses identified for a v0.1 release.

**Actionable Findings**

None.

**Path to 100:** Add `govulncheck` to CI for proactive dependency vulnerability scanning. Document the `flock` advisory locking limitation on NFS in the security model. These are process improvements, not code deficiencies; the security posture of the code itself has no identified gaps.

**Score: 94/100**

---

### 11. Build, Distribution & First-Run Experience

**Analysis**

Clone-to-running requires two commands: `git clone` and `make build`. The Makefile is well-structured with correct targets: `build`, `test`, `lint`, `vet`, `fmt`, `ci`, `install`, `clean`, `build-all`, `build-linux`, `build-darwin`, `build-windows`, `help`. The `ci` target runs `lint test build` in sequence. Cross-compilation works for Linux (amd64, arm64), macOS (amd64, arm64), and Windows (amd64).

`ldflags` version injection is correct: `-X github.com/dorkusprime/wolfcastle/cmd.Version=$(VERSION)` with `git describe --tags --always --dirty`. The binary is self-contained with no runtime files needed (embedded templates via `base/` directory).

The `help` target documents all available targets. CI is already set up with GitHub Actions (`ci.yml`): multi-version Go matrix, lint, cross-compile, smoke tests, integration tests, and Codecov integration. GoReleaser is configured (`.goreleaser.yml`) for automated releases. Homebrew tap is referenced in the README.

First-run experience is good: `wolfcastle init` creates the `.wolfcastle/` directory, `wolfcastle start` launches the daemon, `wolfcastle status` shows the tree. The README's quickstart is accurate and minimal.

The repository contains a `.DS_Store` file that should be removed and added to `.gitignore`.

**Strengths**
- One-command build (`make build`).
- Complete CI pipeline with lint, test, race detection, cross-compile, coverage.
- GoReleaser and Homebrew tap for distribution.
- Self-contained binary with embedded templates.

**Weaknesses**
- `.DS_Store` committed to repository.

**Actionable Findings**

1. **Location:** Repository root `.DS_Store`
   **Issue:** macOS metadata file committed to repository.
   **Fix:** Remove `.DS_Store` from git tracking (`git rm --cached .DS_Store`) and add `.DS_Store` to `.gitignore`.
   **Severity:** warning

**Path to 100:** Remove `.DS_Store`. The `make coverage` and `make release-check` suggestions are convenience improvements, not gaps. The build system, CI pipeline, distribution story (GoReleaser + Homebrew), and first-run experience are all strong. The `.DS_Store` is the only concrete issue.

**Score: 92/100**

---

### 12. Documentation Quality

**Analysis**

The README is excellent. It explains what Wolfcastle is in the first sentence ("You give Wolfcastle a goal. It breaks that goal into pieces. Then it breaks those pieces."), includes a quickstart that works, explains the project tree, daemon, configuration, failure recovery, audits, collaboration, and CLI in clear prose. Badge strip provides CI status, coverage, Go report card, and license. The README would give an experienced Go developer a clear picture of the project's value proposition in under 30 seconds.

Spec quality is high. The state machine spec (~40K) is formally structured with state diagrams, transition rules, and propagation semantics. The CLI commands spec (~128K) documents every command with usage, flags, and examples. The structural validation spec documents all categories with examples and fix strategies.

ADR quality is strong. ADRs capture alternatives considered and tradeoffs (e.g., ADR-056 evaluates Cobra vs. custom CLI framework). The index is well-maintained. Superseded ADRs link to replacements.

Inline documentation is good: package-level doc comments on every package, function-level comments on public APIs, type-level comments on key types. The `daemon.go` package comment even lists the file layout following ADR-045.

The `CONTRIBUTING.md` provides a package map, step-by-step guides for adding commands and validation checks, test expectations, and PR process. A new contributor could add a CLI command by following the 7-step guide.

The `AGENTS.md` file provides architectural context for coding agents working in the repository.

The docs/ directory contains extensive human-readable documentation (`docs/humans/`) covering how-it-works, configuration, audits, failure recovery, collaboration, and CLI references.

**Strengths**
- README is clear, complete, and engagingly written.
- Specs are formally structured and implementable.
- ADRs capture the "why" with alternatives considered.
- CONTRIBUTING.md provides actionable onboarding guides.

**Weaknesses**
- No significant weaknesses identified.

**Actionable Findings**

None.

**Path to 100:** A CHANGELOG entry template and troubleshooting section would be nice additions, but neither represents a gap in the existing documentation. The README, specs, ADRs, CONTRIBUTING guide, and inline comments are all strong. No concrete deficiency was identified.

**Score: 96/100**

---

### 13. Open-Source Readiness

**Analysis**

License: MIT, present as `LICENSE` file, referenced in the README badge strip and `go.mod` module path. All source files are consistent with MIT licensing (no conflicting headers).

Contributing guide: `CONTRIBUTING.md` covers package structure, adding commands, adding validation checks, test expectations, and PR process.

Issue templates: Bug report and feature request templates in `.github/ISSUE_TEMPLATE/`.

Code of conduct: `CODE_OF_CONDUCT.md` present.

Changelog: `CHANGELOG.md` present with release notes.

Security policy: `SECURITY.md` with clear reporting instructions, scope, and supported versions.

Dependency hygiene: 3 direct dependencies, all pinned in `go.mod`, `go.sum` committed. All dependencies are well-maintained (Cobra, fsnotify, readline).

Secrets and credentials: No hardcoded secrets, API keys, or PII found in source files. The default config references `claude` as a model command, which is a public CLI tool name.

Leftover artifacts: `.DS_Store` in the repository root (noted in Build dimension). No other generated files, temporary files, or IDE configuration found. `.gitignore` covers the binary, `dist/`, `coverage.out`, and `.wolfcastle/`.

CI includes CodeQL analysis (`.github/workflows/codeql.yml`) for automated security scanning.

**Strengths**
- Complete OSS scaffolding: LICENSE, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, CHANGELOG.
- Issue templates for bugs and features.
- CI with CodeQL security scanning.
- Minimal, well-maintained dependencies.
- No secrets or PII in source.

**Weaknesses**
- `.DS_Store` committed (cross-referenced from Build dimension).

**Actionable Findings**

None beyond the `.DS_Store` finding already captured in dimension 11.

**Path to 100:** Remove `.DS_Store` (cross-referenced from dimension 11). `CODEOWNERS` and release-drafter are optional tooling for a project that may initially have a single maintainer. Every standard OSS artifact (LICENSE, CONTRIBUTING, CODE_OF_CONDUCT, SECURITY, CHANGELOG, issue templates, CI, CodeQL) is present and well-crafted.

**Score: 95/100**

---

## Summary Scorecard

| Dimension | Weight | Score (/100) | Notes |
| --------- | ------ | ------------ | ----- |
| Architectural Integrity | 12% | 93 | StateStore pattern is exemplary; overlap logic placement is minor |
| Code Quality & Go Idiom | 10% | 93 | Idiomatic throughout; two suggestion-level findings, no real weaknesses |
| Correctness & Safety | 14% | 89 | Atomic writes and locking are correct; swallowed propagation error is a real warning |
| Specification & ADR Fidelity | 6% | 97 | 76 ADRs, 16 accepted specs, no findings, no drift detected |
| Test Suite Quality | 12% | 95 | 3.2:1 ratio, property-based tests, all passing with race detector |
| CLI Design & User Experience | 8% | 82 | Good structure; empty name validation and raw path errors need fixing |
| Configuration System | 5% | 91 | Three-tier merge works correctly; cross-field validation deferred to runtime |
| Daemon Lifecycle & Reliability | 10% | 92 | Solid lifecycle management; force-exit log flush is the only concrete gap |
| Validation Engine | 5% | 96 | 23 categories, multi-pass fix, JSON recovery; no engine-level gaps found |
| Security Posture | 5% | 94 | Clean subprocess execution; minimal dependency surface; no code-level gaps |
| Build, Distribution & First-Run | 5% | 92 | One-command build, full CI, GoReleaser; .DS_Store is the only issue |
| Documentation Quality | 5% | 96 | README is excellent; specs, ADRs, and CONTRIBUTING are thorough; no gaps found |
| Open-Source Readiness | 3% | 95 | Every standard artifact present; .DS_Store is the only blemish |
| **Weighted Total** | **100%** | **92.2** | |

---

## Weighting Rationale

| Dimension | Weight | Rationale |
| --------- | ------ | --------- |
| Architectural Integrity | 12% | Architecture determines long-term maintainability; poor structure is expensive to fix later. |
| Code Quality & Go Idiom | 10% | First impression for Go developers browsing the repo; directly affects contributor onboarding. |
| Correctness & Safety | 14% | Highest weight because data corruption or race conditions would destroy user trust permanently. |
| Specification & ADR Fidelity | 6% | Important for contributor understanding but less critical than runtime correctness. |
| Test Suite Quality | 12% | Tests are the primary safety net for contributors; poor tests erode confidence in the codebase. |
| CLI Design & User Experience | 8% | The CLI is the primary interface; bad UX drives users away before they evaluate the internals. |
| Configuration System | 5% | Important but self-contained; config bugs are easy to fix in isolation. |
| Daemon Lifecycle & Reliability | 10% | The daemon runs unattended for hours; reliability failures waste user time and trust. |
| Validation Engine | 5% | Critical for self-repair but used infrequently; bugs here are less visible than daemon bugs. |
| Security Posture | 5% | Important but the trust model is explicit; most attacks require local access anyway. |
| Build, Distribution & First-Run | 5% | Gate to entry; but easy to fix (unlike architectural problems). |
| Documentation Quality | 5% | Important for first impression but easily improved post-release. |
| Open-Source Readiness | 3% | Scaffolding items (templates, CODEOWNERS) are easy to add; weighted lightly. |

---

## Release Blockers

No release blockers identified.

The two most serious issues (empty project name validation and `.DS_Store`) are both firmly in the "warning" category: they affect user experience and first impressions but cannot cause data loss, corruption, or incorrect behavior.

---

## Release Warnings

1. **What:** `project create ""` accepts an empty name, creating an "unnamed" project.
   **Why:** A user who accidentally submits an empty name gets a confusing tree entry. A model invoking the command with empty input creates noise in the project tree.
   **Fix:** In `cmd/project/create.go`, validate that the name argument (after slug normalization) is non-empty. Return `"project name cannot be empty"`.
   **Verification:** Run `wolfcastle project create ""` and verify it returns an error.

2. **What:** `.DS_Store` committed to repository root.
   **Why:** macOS metadata file in an open-source repo signals carelessness to the first contributor who runs `git status`.
   **Fix:** `git rm --cached .DS_Store` and add `.DS_Store` to `.gitignore`.
   **Verification:** Confirm `.DS_Store` is not in `git ls-files`.

3. **What:** Error messages for nonexistent nodes expose raw filesystem paths.
   **Why:** `"open /tmp/wolfcastle-test/.wolfcastle/system/projects/wild-macbook-pro/nonexistent/state.json: no such file or directory"` is an internal implementation detail, not a user-facing error.
   **Fix:** In `StateStore.ReadNode` (or at the command level), catch `os.ErrNotExist` and return `"node %q not found"` with the tree address.
   **Verification:** Run `wolfcastle task add "foo" --node nonexistent` and verify the error message uses the tree address.

4. **What:** Propagation failure in `MutateNode` is silently swallowed.
   **Why:** If propagation fails, the node is saved but ancestors and index may be stale until `doctor` runs. A user running commands in rapid succession could see inconsistent status output.
   **Fix:** Log the propagation error. Consider returning it as a wrapped warning.
   **Verification:** Unit test that simulates a propagation failure and verifies the error is logged.

---

## Commendations

1. **`StateStore.MutateNode` in `internal/state/store.go`:** The lock-callback-write-propagate pattern makes concurrent state corruption structurally impossible. Callers cannot forget to lock because the API does not expose unlocked writes. This is the single best design decision in the codebase.

2. **Property-based propagation tests in `internal/state/propagation_property_test.go`:** Generating random trees with configurable depth and branching, applying 10-50 random mutations, and verifying four invariants across 500 seeds catches the class of subtle desynchronization bugs that no deterministic test suite would find. The generator is realistic, the mutations cover the full operation set, and the invariants are well-chosen.

3. **The 76-ADR decision trail:** Every material design decision is documented with context, alternatives considered, and tradeoffs accepted. ADR-056 (Cobra evaluation) even documents the decision to use a framework vs. roll a custom CLI. This level of decision documentation is rare in open-source projects and will pay dividends for every future contributor.

4. **The validation engine's multi-pass fix with verification (`internal/validate/fix.go`):** Staging fixes in memory, applying them in a single batch, and re-validating after each pass is exactly the right approach. The 5-pass cap guarantees termination, and the post-fix re-validation catches regressions. The JSON recovery fallback for corrupt state files is a thoughtful touch.

5. **Terminal marker scanning with priority semantics (`internal/daemon/iteration.go:scanTerminalMarker`):** Parsing model output for markers is fraught with edge cases (prompt echo, JSON stream envelopes, multiple markers in one output). The priority ordering (COMPLETE > BLOCKED > YIELD) with JSON envelope extraction handles the real-world scenarios where models echo their instructions or produce intermediate output before the final marker.

---

## Improvement Roadmap

### Before release

No blockers.

### First 30 days

- Release Warning #1: Validate empty project names (effort: small, impact: high)
- Release Warning #2: Remove `.DS_Store` (effort: small, impact: medium)
- Release Warning #3: User-facing error messages for nonexistent nodes (effort: small, impact: high)
- Release Warning #4: Log propagation failures in `MutateNode` (effort: small, impact: medium)
- Extract overlap detection from `cmd/cmdutil/app.go` to its own file (effort: small, impact: low)
- Add cross-field config validation for model references in pipeline stages (`internal/config/config.go`; effort: small, impact: medium)

### First 90 days

- Remove legacy `Invoke`/`InvokeStreaming` wrappers from `internal/invoke/invoker.go` (effort: small, impact: low)
- Extract `handleTerminalMarker` from `daemon/iteration.go:runIteration` (effort: small, impact: low)
- Add orphaned temp file cleanup to `doctor` (effort: small, impact: low)
- Add `--dry-run` flag to `doctor --fix` (effort: medium, impact: medium)
- Add PID-recycling edge case tests to `filelock_test.go` (effort: small, impact: low)
- Add `govulncheck` to CI for dependency vulnerability scanning (effort: small, impact: medium)
- Consider adding `CODEOWNERS` file and release-drafter workflow (effort: small, impact: low)
