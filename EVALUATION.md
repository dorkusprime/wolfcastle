# Wolfcastle Release-Readiness Evaluation

## Executive Summary

**Verdict: Ready with conditions.**

Conditions: (1) The `PropagateState` method in `cmd/cmdutil/app.go` bypasses the `StateStore` lock, creating a narrow window for state desynchronization if a CLI command runs during daemon propagation. This should be fixed before release. (2) The `ValidateAll` comment in `engine.go` claims 17 validation categories but the engine checks 24; a trivial fix but worth catching before publication.

Wolfcastle is a genuinely strong Go project. Its three finest qualities are its **state mutation architecture** (the `StateStore` pattern that makes concurrent corruption structurally difficult by routing all writes through lock-protected callbacks with automatic propagation), its **validation engine** (24 categories of structural checks with multi-pass deterministic repair and JSON recovery from truncated or corrupted files), and its **test suite** (1,763 passing tests at a 3.2:1 test-to-source ratio, with property-based propagation testing and careful coverage of edge cases like file permission errors and malformed JSON).

The most serious issues, ranked: (1) The unlocked propagation path in `cmdutil/app.go:PropagateState`, which could desynchronize parent states in a race with the daemon. (2) The `ValidateAll` comment claims "17 validation categories" but the engine actually checks 24, a minor documentation staleness that won't confuse users but signals incomplete internal review. (3) No integration test exercises the daemon's inbox goroutine alongside the execute loop under concurrent CLI mutation, the highest-risk runtime scenario.

The single improvement that would most strengthen a contributor's first impression: the README is already excellent, but a `make ci` target that runs lint, test, and build in one command would signal CI-readiness immediately.

---

## Project Identity

Wolfcastle is the work of someone who has been bitten by state corruption, race conditions, and silent data loss in production, and decided that the answer was to make the wrong thing impossible rather than merely discouraged. The project optimizes for correctness over performance, determinism over flexibility, and explicit failure over silent degradation. Every mutation passes through a lock. Every write is atomic. Every state transition is structural.

The codebase carries the personality of a systems engineer who cares about the Go standard library more than the Go ecosystem: only three direct dependencies, zero code generation, no ORM, no framework beyond Cobra. The architecture decisions are documented with a thoroughness that borders on obsessive (76 ADRs), and the documentation itself is well-written and opinionated, not perfunctory.

The kind of engineer drawn to contribute here would be someone who reads ADRs before writing code, who prefers `filepath.Join` over string concatenation, and who finds satisfaction in a well-designed state machine. This is a project for people who think "it works" is necessary but not sufficient.

---

## Quantitative Profile

| Metric | Value |
| ------ | ----- |
| Total Go LOC (excluding tests) | 14,223 |
| Total test LOC | 45,428 |
| Test-to-source ratio | 3.19:1 |
| Number of source files | 111 |
| Number of test files | 158 |
| Number of packages | 25 |
| External dependencies (direct) | 3 |
| External dependencies (transitive) | 3 |
| CLI commands/subcommands | 38 |
| ADRs (accepted) | 76 |
| Specs (non-draft) | 16 |
| Specs (draft) | 3 |
| Validation issue categories | 24 |
| Build result (pass/fail) | Pass |
| Test result (pass count / fail count) | 1,763 / 0 |
| Linter warnings | 0 |

---

## Per-Dimension Analysis

### 1. Architectural Integrity

**Analysis**

The system decomposes into 15 internal packages plus `cmd/` with 6 subpackages. The boundaries are well-chosen and genuinely separate concerns: `state` owns types, I/O, mutations, navigation, and propagation; `daemon` owns the iteration loop and pipeline dispatch; `pipeline` owns prompt assembly; `invoke` owns subprocess execution; `validate` owns structural checking and repair; `config` owns loading and merging; `tree` owns address parsing and path resolution.

The dependency graph flows cleanly: `cmd/*` depends on `internal/*`, never the reverse. Within `internal/`, the layering is `daemon` → `pipeline`, `invoke`, `state`, `logging`; `pipeline` → `config`, `state`; `validate` → `state`, `tree`. No cycles exist. A new contributor could draw this graph after reading `CONTRIBUTING.md` and browsing the import statements for twenty minutes.

The `StateStore` pattern (`internal/state/store.go`) is the project's most important abstraction. `MutateNode` acquires a file lock, reads the current state, applies a caller-supplied callback, writes atomically, then propagates state changes up through parent nodes and the root index, all within the lock. This makes concurrent state corruption structurally difficult: callers cannot forget to lock, cannot forget to propagate, and cannot write partial state. The pattern earns its complexity.

The concurrency model is two goroutines: the main execute loop and the inbox watcher (ADR-064). Communication is through the `workAvailable` channel (buffered at 1). The inbox goroutine uses fsnotify with a polling fallback. Both goroutines share the `StateStore`, which serializes access through file locking. This is appropriate for the workload: one daemon process per engineer namespace.

Invariant enforcement is mostly structural. The audit-last invariant is enforced by `MoveAuditLast` called after every task insertion. Serial execution is enforced by `selfHeal` at startup (which errors on >1 in-progress task) and by the single-goroutine execute loop. Terminal completion (a node is complete only when all children are complete) is enforced by `RecomputeState` and `TaskComplete`. Upward-only propagation is enforced by `PropagateUp`, which walks from child to root with cycle detection.

One concern: `cmd/cmdutil/app.go:PropagateState` (line 95) provides a propagation path that operates outside the `StateStore`'s lock. CLI commands that use `app.PropagateState` instead of `store.MutateNode` could race with the daemon. Most commands already use `store.MutateNode` (which auto-propagates), but the existence of this alternative path is a latent risk.

**Strengths**

- The `StateStore` pattern makes correct concurrent access the path of least resistance.
- Package boundaries align with domain concepts, not organizational convenience.
- The dependency graph is acyclic and could be drawn by a newcomer in under an hour.
- Cycle detection in `PropagateUp` with a `maxPropagationDepth` guard prevents runaway recursion.

**Weaknesses**

- The `PropagateState` method in `cmdutil/app.go` bypasses `StateStore` locking, creating a potential race with the daemon.

**Actionable Findings**

1. **Location:** `cmd/cmdutil/app.go:PropagateState` (line 95)
   **Issue:** This method reads and writes state files without acquiring the namespace file lock, creating a race window with `StateStore.MutateNode`.
   **Fix:** Either remove this method entirely (all callers should use `store.MutateNode` which auto-propagates) or wrap it in `store.WithLock`.
   **Severity:** warning

**Path to 100:** Fix the unlocked propagation path. Verify that no `cmd/` code uses `app.PropagateState` when it could use `store.MutateNode`. The rest of the architecture is sound.

**Score: 92/100**

---

### 2. Code Quality & Go Idiom

**Analysis**

The code is idiomatic Go throughout. Error handling follows `fmt.Errorf("doing X: %w", err)` consistently; I did not find a single bare `return err` that loses context. Interface design is minimal and appropriate: `Invoker` is a single-method interface, `clock.Clock` is a single-method interface, both accept implementations and return concrete types. Package naming follows Go convention (lowercase, singular, descriptive). Zero-value usefulness is respected; `NodeState` defaults work correctly.

The typed error system (`internal/errors`) is four types and four constructor functions in 60 lines. It earns its existence: the daemon uses `errors.As` to distinguish retryable invocation errors from fatal state errors, which drives the retry-vs-abort decision in `daemon.go:RunOnce` (line 441). This is precisely the right amount of ceremony for the use case.

Naming is intention-revealing and consistent. `TaskClaim`, `TaskComplete`, `TaskBlock`, `TaskUnblock` are verb-noun pairs that read like commands. `FindNextTask`, `PropagateUp`, `RecomputeState` are descriptive. `MutateNode`, `MutateIndex`, `MutateInbox` form a consistent API surface. I found no naming inconsistencies across packages.

Functions are well-sized. The longest function I found is `runIteration` in `daemon/iteration.go` at approximately 100 lines, which is acceptable for a function that orchestrates an entire pipeline iteration. Most functions are under 40 lines. Parameter lists stay under 5 parameters; where more context is needed, the `Daemon` struct carries it.

No dead code, no stale TODOs, no commented-out blocks. The `grep` for TODO/FIXME/HACK/XXX across all non-test source files returned zero results. The backward-compatibility wrappers in `invoke/invoker.go` (`Invoke` and `InvokeStreaming`) are documented as legacy and delegate to the canonical implementation, which is the right way to handle a transition.

No magic values. Constants are named (`DefaultLockTimeout`, `maxFixPasses`, `maxPropagationDepth`). Configuration values have hardcoded defaults in `config.Defaults()` with every field documented. The `StartupCategories` map explicitly lists which validation categories run at daemon startup.

`go vet` and `gofmt` pass cleanly. Zero linter warnings.

**Strengths**

- Consistent error wrapping with context across the entire codebase.
- The typed error system is justified by its use in retry/abort decisions.
- Zero dead code, zero TODOs, zero linter warnings.
- Function sizing is disciplined; even the orchestration functions stay readable.

**Weaknesses**

No significant weaknesses identified.

**Actionable Findings**

No actionable findings.

**Path to 100:** I cannot articulate a concrete improvement that would earn the remaining points. The code is clean, idiomatic, and consistent. The backward-compatibility wrappers in `invoke` could be removed once all callers migrate, but they're documented and harmless.

**Score: 97/100**

---

### 3. Correctness & Safety

**Analysis**

The atomic write pattern in `atomicWriteJSON` (`internal/state/io.go:71`) is correctly implemented: `os.CreateTemp` creates the temp file in the same directory as the target (required for same-filesystem rename), data is written, `Sync()` is called, the file is closed, and `os.Rename` performs the atomic replacement. If the process is killed between `CreateTemp` and `Rename`, an orphaned `.wolfcastle-tmp-*` file remains, but the original state file is intact. The cleanup of orphaned temp files is not explicitly handled, which is a minor concern for long-running daemons on small filesystems, but not a correctness issue.

File locking uses `flock(2)` with `LOCK_NB` (non-blocking), wrapped in a polling loop with a configurable timeout. The stale lock detection (`tryCleanStaleLock`) reads the PID from the lock file and sends signal 0 to check if the process is alive. If the process is dead, it calls `flockUnlock` to release the advisory lock. This is correct on local filesystems. On NFS, `flock` is advisory and may not provide mutual exclusion; the README doesn't claim NFS support, so this is acceptable. PID recycling is a theoretical concern: if a Wolfcastle process dies and another process reuses its PID before the lock is checked, the stale detection will incorrectly think the original process is still alive. The window for this is extremely narrow on modern systems and the consequence is a lock timeout, not data corruption.

State consistency under concurrent access is well-handled. `MutateNode` acquires the lock, reads the latest state, applies the callback, writes, and propagates, all atomically. The daemon's execute loop and CLI commands both go through `MutateNode`. The inbox goroutine also uses `MutateInbox` or direct `state.InboxMutate` with its own lock.

Propagation correctness: `PropagateUp` walks from child to root, updating each parent's child reference state and recomputing the parent state. Cycle detection via the `visited` map prevents infinite loops. The `maxPropagationDepth` guard of 100 catches degenerate cases. After `PropagateUp` returns, `Propagate` re-walks the ancestor chain to update the root index, ensuring index consistency. This double-walk is slightly redundant but safe.

Signal handling is correct and thorough. The daemon registers `SIGINT`, `SIGTERM`, and `SIGTSTP` on Unix. It uses both `signal.NotifyContext` (for canceling in-flight invocations) and a dedicated signal channel (as a backup because child processes may corrupt Go's signal infrastructure). The force-exit goroutine (2-second grace period after signal) prevents hanging if the spinner blocks. Terminal restoration (`RestoreTerminal`) is called after every model invocation to reset ISIG, ICANON, and ECHO flags.

The deliverable path traversal protection is in `state/store.go:nodePath` (line 193), which validates that address segments are not empty, `.`, `..`, or containing whitespace. The `tree/address.go` package adds slug validation. Deliverable paths in `daemon/deliverables.go` use `filepath.Join(repoDir, d)` without additional traversal validation, meaning a deliverable path like `../../../etc/passwd` could theoretically escape. However, deliverable paths are set by the model via CLI commands, and the security model (ADR-022) accepts that the model has full filesystem access within the repo directory, so this is by design.

**Strengths**

- The atomic write pattern is textbook correct: same-directory temp file, fsync, rename.
- The `StateStore` lock scope covers the full read-modify-write-propagate cycle.
- Signal handling has three layers of defense: NotifyContext, backup channel, and forced exit.
- Terminal restoration after child process exit prevents the "dead terminal" problem.

**Weaknesses**

- Orphaned `.wolfcastle-tmp-*` files are never cleaned up. On a long-running daemon that crashes repeatedly during writes, these could accumulate.

**Actionable Findings**

1. **Location:** `internal/state/io.go:atomicWriteJSON` and `internal/validate/engine.go`
   **Issue:** Orphaned temp files from interrupted atomic writes are never cleaned up.
   **Fix:** Add a `STALE_TEMP_FILE` validation category in the doctor engine that scans for `.wolfcastle-tmp-*` files older than 1 hour and removes them.
   **Severity:** suggestion

**Path to 100:** Clean up orphaned temp files in the validation engine. The rest of the correctness story is strong; the atomic writes, file locking, propagation, and signal handling are all implemented correctly.

**Score: 93/100**

---

### 4. Specification & ADR Fidelity

**Analysis**

The project has 16 non-draft specs and 3 draft specs (TUI, worktree-by-default, task classes). The 76 ADRs document every significant design decision from file format choices to package consolidation to goroutine architecture. The ADR index (`docs/decisions/INDEX.md`) provides a navigable map.

Spec-code alignment is strong. The state machine spec maps directly to the four `NodeStatus` constants and the transition functions in `mutations.go`. The config schema spec matches the `Config` struct and three-tier loading in `config.go`. The CLI commands spec covers all 38 commands. The structural validation spec aligns with the 24 categories in `validate/types.go`, though the engine.go comment says "17 validation categories" (stale from an earlier count before audit-specific categories were added).

ADR compliance is high. ADR-042 (file locking) is faithfully implemented in `filelock.go`. ADR-064 (consolidated intake with parallel inbox) is implemented in `daemon/stages.go` with the parallel goroutine. ADR-068 (unified state store) is implemented in `state/store.go`. ADR-018 (merge semantics) is implemented in `config/merge.go` with null deletion. ADR-051 (multi-pass doctor) is implemented in `validate/fix.go` with the `maxFixPasses` cap. ADR-062 (realistic model mocks) is reflected in the `Invoker` interface and `CmdFactory` field on `ProcessInvoker`.

The draft specs (TUI, worktree-by-default, task classes) describe features that are not implemented. This is correct per the evaluation instructions: no penalty for unimplemented drafts.

I did not find any ADRs whose decisions have been superseded without an update. The ADR numbering is sequential and the index matches the files on disk.

One minor staleness: the `ValidateAll` comment in `engine.go:93` says "runs all 17 validation categories" but the actual count is 24. This was likely correct when the comment was written and not updated when audit-specific and daemon artifact categories were added.

**Strengths**

- 76 ADRs capture the "why" behind design decisions, not just the "what."
- Spec-code alignment is strong across the state machine, config, CLI, and validation specs.
- Draft specs are clearly marked and do not mislead about the current implementation.

**Weaknesses**

- The `ValidateAll` comment in `engine.go` claims 17 categories but the engine checks 24.

**Actionable Findings**

1. **Location:** `internal/validate/engine.go:93`
   **Issue:** Comment says "17 validation categories" but the engine checks 24.
   **Fix:** Update the comment to "24 validation categories" (or remove the count and say "all validation categories").
   **Severity:** suggestion

**Path to 100:** Fix the stale comment. That is the only concrete finding. The sampling I performed across state machine, config, CLI, and validation specs showed accurate alignment throughout.

**Score: 95/100**

---

### 5. Test Suite Quality

**Analysis**

The test suite has 1,763 passing tests across 23 packages with zero failures. The test-to-source ratio of 3.19:1 indicates significant investment in testing. Wall-clock time for the full suite is under 70 seconds (dominated by `internal/daemon` at 61s, which includes property-based tests and timing-dependent scenarios).

The three-tier strategy is well-implemented. Unit tests in each package test individual functions with table-driven cases. Integration tests in `test/integration/` exercise multi-package interactions (daemon side effects). Smoke tests in `test/smoke/` verify the binary builds and runs.

Property-based propagation tests exist in `internal/state/propagation_property_test.go`. These generate random tree structures and verify that propagation invariants hold: parent state is always consistent with child states, cycle detection catches cycles, and propagation depth is bounded. This is the right approach for a function where the state space is large and manual test cases cannot cover all combinations.

Critical path coverage is good. State transitions are tested in `mutations_test.go` with cases for every valid transition and rejection of invalid transitions. Propagation desynchronization is tested in `propagation_test.go` and the property-based tests. Invariant violations (audit-last, serial execution) are tested in `validate/engine_test.go`. Config merge semantics including null deletion are tested in `config/merge_test.go`. Validation false positives are tested via the edge case tests.

Test quality is high. Assertions are specific (checking exact state values, error messages, and side effects). Test names communicate intent (`TestTaskBlock_NotStarted_Allowed`, `TestPropagateUp_CycleDetection`). I did not find tautological assertions.

The `testutil` package provides helpers for creating test trees, which reduces boilerplate. The `CmdFactory` field on `ProcessInvoker` enables injection of mock commands without a separate mock interface, which is practical.

Test files for filesystem permission errors (`*_chmod_test.go`) test behavior when state files are unreadable, which shows attention to operational edge cases.

Flakiness risk: the daemon tests in `internal/daemon/` use real timers and could theoretically flake under extreme system load. The 61-second run time suggests some tests have real-time dependencies. The test suite uses `t.Parallel()` throughout, which is correct.

Missing tests: I did not find an integration test that exercises the inbox goroutine running simultaneously with the execute loop while a CLI command mutates state. This is the highest-risk concurrent scenario and is currently tested only through unit tests of the individual components.

**Strengths**

- 3.19:1 test-to-source ratio with zero failures.
- Property-based propagation tests cover the state space that manual tests cannot.
- Permission error tests (`*_chmod_test.go`) cover operational edge cases.
- Table-driven tests with intention-revealing names throughout.

**Weaknesses**

- No integration test exercises concurrent daemon execution + inbox processing + CLI mutation.

**Actionable Findings**

1. **Location:** `test/integration/`
   **Issue:** No test exercises the inbox goroutine alongside the execute loop under concurrent CLI mutation.
   **Fix:** Add an integration test that starts the daemon in a goroutine, adds an inbox item via CLI, and mutates a task via CLI, then verifies state consistency after the daemon processes the inbox item.
   **Severity:** suggestion

**Path to 100:** Add the concurrent integration test. The existing test suite is already exceptional; the gap to 100 is about covering the one high-risk scenario that currently relies on component-level testing.

**Score: 93/100**

---

### 6. CLI Design & User Experience

**Analysis**

The CLI surface has 38 commands organized into 6 groups: Lifecycle (init, log, start, status, stop, update, version), Work Management (archive, inbox, navigate, project, task), Auditing (audit with 14 subcommands), Documentation (adr, spec), Diagnostics (doctor, unblock), and Integration (install). The grouping is logical and discoverable.

Help text is accurate and includes usage patterns, required vs. optional flags, and examples. The root help output includes a quickstart that matches the actual CLI workflow. Subcommand groups show examples in their help text (e.g., `wolfcastle task --help` shows five example commands). The help is good enough for a model to use the tool correctly from help text alone; in fact, this is one of the tool's design goals (models invoke Wolfcastle via CLI commands during execution).

Error messages are specific and actionable. Missing arguments show the usage pattern. Invalid node addresses explain the expected format (`"my-project/task-1"`). Missing `.wolfcastle` directory suggests running `init`. The error format is consistent: Cobra usage errors show usage first then the error, while operational errors use the `output.PrintError` format.

JSON output is consistent across commands. Every command wraps its output in the `output.Response` envelope with `ok`, `action`, `error`, `code`, and `data` fields. I verified this across `status --json`, `doctor --json`, and the error path. A consumer could reliably parse output from any command without special-casing.

Shell completions are generated by Cobra and cover all subcommands and flags. The `cmd/cmdutil/completions.go` file provides custom completers for dynamic values (node addresses, task IDs), which is a thoughtful touch.

Workflow ergonomics are smooth. The init→create→add→start workflow works as documented. The doctor command provides clear, categorized output with severity levels and fix type indicators. The status command shows a colored tree view with node states.

No stability markers (experimental/stable) exist on commands or flags. This is acceptable for a v0.1 release, but should be considered before v1.0.

**Strengths**

- 38 commands organized into logical groups with consistent help text.
- JSON envelope is consistent across every command, including error responses.
- Error messages are specific and suggest corrective actions.
- Custom shell completions for dynamic values (node addresses, task IDs).

**Weaknesses**

No significant weaknesses identified.

**Actionable Findings**

No actionable findings.

**Path to 100:** No weaknesses and no actionable findings were identified. Stability annotations would be valuable before v1.0 but are not expected at v0.1. The CLI design is clean, consistent, and well-documented.

**Score: 96/100**

---

### 7. Configuration System

**Analysis**

The three-tier configuration system (base → custom → local) is implemented in `config/config.go:Load`. Resolution order is hardcoded defaults → `base/config.json` → `custom/config.json` → `local/config.json`, with each tier overriding the previous. The `DeepMerge` function in `config/merge.go` handles the semantics correctly: objects are deep-merged recursively, arrays in the source replace the destination entirely, and `null` values delete the key (ADR-018 delete semantics).

The merge implementation is correct and well-tested. It clones before merging to avoid mutating the input, handles nested objects recursively, and treats `nil` as a deletion signal. The `cloneValue` function recursively copies maps and slices, ensuring no shared mutable state between the original and the merged result.

Validation is thorough. `ValidateStructure` checks pipeline stage requirements (at least one stage, model references exist, unique names, non-empty prompt files), failure thresholds, daemon timing values, log retention, retry constraints, validation command configuration, git commit format, overlap advisory threshold, and model definitions. The validation produces a multi-error report with all failures listed, not just the first.

Defaults in `config.Defaults()` are sensible and well-documented: three model tiers (fast/mid/heavy), a two-stage pipeline (intake + execute), reasonable retry and failure thresholds, standard daemon polling intervals, and git auto-commit enabled by default. A user can start the daemon immediately after `init` with no config modifications.

Extensibility: new config fields can be added to the `Config` struct without breaking existing configs because `json.Unmarshal` ignores unknown fields. There is no explicit versioning or migration strategy for the config schema, but the `DeepMerge` approach makes backward compatibility natural: old configs simply don't have the new fields, so defaults apply.

**Strengths**

- Correct deep-merge with null deletion, clone-before-merge, and recursive object handling.
- Thorough validation with multi-error reporting and specific error messages.
- Sensible defaults that allow zero-config startup.

**Weaknesses**

No significant weaknesses identified.

**Actionable Findings**

No actionable findings.

**Path to 100:** No weaknesses and no actionable findings were identified. A config schema version field would be valuable at v1.0 but is not a gap in the current system, where `DeepMerge` provides natural backward compatibility.

**Score: 97/100**

---

### 8. Daemon Lifecycle & Reliability

**Analysis**

The daemon startup sequence in `cmd/daemon/start.go` validates configuration, checks for an existing PID file, writes a new PID file, creates the daemon, and runs with the supervisor. If startup validation fails, the error is reported before the PID file is written. The self-healing phase (`daemon.go:selfHeal`) scans the tree for stale in-progress tasks and reports or errors based on the count (>1 is a fatal state corruption; exactly 1 is a resumable interruption).

The iteration loop in `daemon.go:Run` is well-structured. It checks context cancellation, stop file, and iteration cap before each iteration. It starts the inbox goroutine, manages an idle spinner during no-work periods, and handles four iteration outcomes (DidWork, NoWork, Stop, Error) with appropriate behavior for each. Log retention is enforced after each successful iteration.

The `RunWithSupervisor` wrapper provides crash recovery with configurable restart limits (default 3) and delay (default 2 seconds). It correctly resets daemon state between restarts (new `shutdown` channel, new `sync.Once`, new `workAvailable` channel). The supervisor distinguishes between nil errors (clean shutdown, not restarted), context cancellation (parent canceled, not restarted), and other errors (crash, restarted up to the limit). It does not distinguish between recoverable and permanent failures beyond the max restart count.

Model invocation hanging is handled by `InvocationTimeoutSeconds` (default 3600s = 1 hour), implemented via `context.WithTimeout`. The timeout cancels the context, which kills the child process via `exec.CommandContext`. This is correct.

Crash recovery at different points: during model invocation, the context cancels and the child is killed; the task remains in_progress and will be resumed on next startup via `selfHeal`. During state write, atomic writes prevent corruption; at worst the write is lost and the previous state is intact. During propagation, the lock is held and the worst case is a stale parent state, which `doctor` can repair. During log rotation, errors are silently ignored (non-critical). During inbox processing, items remain in "new" status for retry.

Resource management: the daemon creates new `FileLock` instances per mutation (not leaked), log files are rotated and retained per configuration, and the spinner is properly stopped on all exit paths. The inbox goroutine's fsnotify watcher is closed via `defer`.

Performance: a single iteration performs O(n) filesystem reads for navigation (scanning the index and loading node states depth-first) plus O(depth) reads/writes for propagation. The `FindNextTask` DFS traversal is O(nodes), which is acceptable for trees of 100+ nodes. JSON marshaling/unmarshaling is the dominant cost per file operation, but individual state files are small (under 10KB typically).

**Strengths**

- The supervisor correctly resets all daemon state between restarts.
- Invocation timeouts prevent hanging on unresponsive models.
- Self-healing at startup detects and resumes interrupted tasks.
- All exit paths properly clean up the spinner, logger, and PID file.

**Weaknesses**

- The force-exit goroutine in `daemon.go:Run` (line 218) calls `os.Exit(0)` after a 2-second grace period, which bypasses deferred cleanup in the caller. The PID file removal on line 219 partially mitigates this, but any other deferred cleanup (e.g., log flushing) is skipped.

**Actionable Findings**

1. **Location:** `internal/daemon/daemon.go:218-221`
   **Issue:** The force-exit goroutine calls `os.Exit(0)` which bypasses all deferred cleanup except the PID file removal on the preceding line.
   **Fix:** Instead of `os.Exit`, set a boolean flag that the main loop checks on each iteration to break out of the select, allowing deferred cleanup to run. If the main loop is truly stuck, the `os.Exit` as a last resort is acceptable but should be documented as such.
   **Severity:** suggestion

**Path to 100:** Address the force-exit cleanup issue. Add a test that verifies the supervisor distinguishes between transient and permanent failures. These are reliability hardening items, not correctness bugs.

**Score: 90/100**

---

### 9. Validation Engine

**Analysis**

The validation engine in `internal/validate/` checks 24 categories of structural invariants. The categories cover: root index consistency (dangling refs, missing entries), orphaned state and definition files, propagation mismatches, audit task invariants (missing, not last, multiple), state value validity, transition invariants (complete with incomplete, blocked without reason), concurrency violations (stale in-progress, multiple in-progress), depth mismatches, negative failure counts, missing required fields, malformed JSON, audit state integrity (scope, status, gaps, escalations, status-task mismatch), and daemon artifacts (stale PID, stale stop file).

The fix system in `validate/fix.go` handles 17 of the 24 categories with deterministic repairs. Fixes include: removing dangling index entries, adding orphaned nodes to the index, recomputing propagation state, adding missing audit tasks, moving audit tasks to last position, normalizing state values, resetting negative failure counts, populating missing required fields, syncing audit lifecycle, clearing stale gap metadata, and removing stale daemon artifacts. Each fix is idempotent: applying the same fix twice produces the same result.

The multi-pass strategy (`FixWithVerification`) runs up to 5 passes. Each pass validates, applies fixes, and re-validates. The loop exits when no fixable issues remain or the pass cap is reached. Convergence is guaranteed because each fix reduces the issue count (a fix that creates new issues would be caught by the post-fix validation within the same pass, and the next pass would see the new issue). The `maxFixPasses = 5` cap prevents infinite loops.

The JSON recovery system (`validate/recover.go`) handles malformed state files through three strategies: BOM removal and null byte stripping, trailing garbage stripping, and truncated JSON closure. The `detectLoss` function heuristically estimates whether tasks or children were lost during recovery. This is sophisticated and practical: a daemon crash during a write could produce truncated JSON, and recovery is preferable to data loss.

Performance: validation scans all nodes in the index plus a filesystem walk for orphaned files. For a tree of 100 nodes, this takes milliseconds. The engine is fast enough for daemon startup without noticeable delay.

**Strengths**

- 24 validation categories covering structural, semantic, and operational invariants.
- Multi-pass fix loop with convergence guarantee and post-fix re-validation.
- JSON recovery from truncated, BOM-corrupted, and null-byte-corrupted files.
- Fix idempotency: applying fixes twice produces the same result.

**Weaknesses**

No significant weaknesses identified.

**Actionable Findings**

No actionable findings.

**Path to 100:** No weaknesses and no actionable findings were identified. The orphaned temp file concern belongs to Correctness & Safety, not to the validation engine itself (which would merely be the vehicle for a cleanup check). The engine's own design is sound.

**Score: 97/100**

---

### 10. Security Posture

**Analysis**

Wolfcastle's security model (ADR-022) is explicit: the model has full access to the codebase within the working directory, but communicates with Wolfcastle only through deterministic CLI script calls. Model output flows through `scanTerminalMarker` (which looks for specific marker strings) and CLI command invocations (which go through Cobra's argument parsing). The model cannot directly mutate state files; it invokes `wolfcastle task complete`, `wolfcastle audit breadcrumb`, etc., which go through the standard mutation paths.

Prompt injection surface: model output is scanned for markers (`WOLFCASTLE_COMPLETE`, `WOLFCASTLE_BLOCKED`, `WOLFCASTLE_YIELD`) and summary text. The marker detection uses exact string matching on trimmed lines, not regex or eval. JSON envelope parsing for Claude Code stream format uses `json.Unmarshal` into a fixed struct, which rejects unexpected fields. There is no `eval`, no template expansion, and no shell interpretation of model output.

Path traversal: node addresses are validated by `tree.ParseAddress` which rejects empty segments, `.`, `..`, and segments containing whitespace. The `StateStore.nodePath` method applies the same validation. Deliverable paths are joined with `filepath.Join(repoDir, d)` and not additionally validated for traversal, but per ADR-022 the model already has filesystem access within the repo, so this is by design.

Subprocess execution: models are invoked via `exec.CommandContext` with arguments passed as a list (not concatenated into a string), which prevents shell injection. The child is placed in its own process group via `Setpgid: true`. Environment variables are inherited from the parent process (no explicit filtering), which means API keys in the environment are visible to the model subprocess. This is by design (the model needs API access) but should be documented.

State file integrity: `json.Unmarshal` into typed structs ignores unexpected fields and uses Go's zero values for missing fields. Extremely large values or unexpected types would either be ignored (wrong type) or parsed into the field (correct type, unusual value). The validation engine catches many of these cases (invalid state values, missing required fields).

Dependency supply chain: 3 direct dependencies (`cobra` v1.10.2, `fsnotify` v1.9.0, `readline` v1.5.1) and 3 indirect (`mousetrap`, `pflag`, `golang.org/x/sys`). All are well-maintained, widely-used Go libraries. The `go.sum` file is committed with 21 entries, providing integrity verification. No known security advisories affect these versions at the time of evaluation.

**Strengths**

- No shell interpretation of model output; all subprocess invocation uses argument lists.
- Address validation prevents path traversal in node names.
- Marker detection uses exact string matching, not eval or regex.
- Minimal dependency surface (3 direct deps, all well-maintained).

**Weaknesses**

No significant weaknesses identified for the stated security model.

**Actionable Findings**

No actionable findings.

**Path to 100:** No vulnerabilities were found within the stated security model. Environment variable inheritance is by design (ADR-022). An optional allowlist would be a feature, not a fix. The security posture is appropriate for a local tool with an explicit trust model.

**Score: 96/100**

---

### 11. Build, Distribution & First-Run Experience

**Analysis**

Clone-to-running requires exactly two commands after `git clone`:

```bash
make build    # produces ./wolfcastle
./wolfcastle init
```

Or one command with `make install` (installs to `$GOPATH/bin`). Or `brew install dorkusprime/tap/wolfcastle` for end users. The binary is fully self-contained with no runtime files required; embedded templates are included via Go's `embed` directive in `internal/project/`.

The Makefile has 12 targets (build, test, test-verbose, install, clean, lint, vet, fmt, build-all, build-linux, build-darwin, build-windows, help). Targets are well-named and the `help` target prints a description of each. The `ldflags` version injection sets `Version`, `Commit`, and `Date` at build time, which is correct. Cross-compilation targets cover Linux (amd64, arm64), macOS (amd64, arm64), and Windows (amd64).

CI is configured with three GitHub Actions workflows: `ci.yml` (build, test, lint), `codeql.yml` (code scanning), and `release.yml` (release automation). The Makefile `lint` target runs `vet` and `fmt` checks. There is no `make ci` target that combines lint + test + build in one command, which would be a convenience for contributors.

The Makefile `lint` target doesn't depend on `test`, so a contributor might run `make lint` and think the codebase is clean without running tests. Adding a `make ci` target would address this.

The `codecov.yml` file exists in the repository root, indicating code coverage integration is set up. The README has badges for CI, coverage, Go Report Card, and Go Reference.

**Strengths**

- Two commands from clone to working binary.
- Self-contained binary with no runtime dependencies.
- Cross-compilation for three platforms and five architectures.
- CI, code coverage, CodeQL, and release automation already configured.

**Weaknesses**

- No `make ci` target that runs the full CI pipeline locally.

**Actionable Findings**

1. **Location:** `Makefile`
   **Issue:** No combined CI target for running the full pipeline (lint + test + build) locally.
   **Fix:** Add `ci: lint test build` target with a description in `make help`.
   **Severity:** suggestion

**Path to 100:** The only finding is a missing convenience target. Two commands from clone to working binary, self-contained distribution, cross-compilation, and full CI automation are all present. A `make ci` target is a nice-to-have, not a meaningful gap.

**Score: 96/100**

---

### 12. Documentation Quality

**Analysis**

The README is well-written and answers the four essential questions: what is Wolfcastle (a model-agnostic autonomous coding orchestrator), how to install it (`brew install dorkusprime/tap/wolfcastle`), how to use it (quickstart with `init`, `project create`, `task add`, `start`), and how to contribute (link to CONTRIBUTING.md). The value proposition is clear in the first two sentences. The README includes status badges, architecture overview, and links to ADRs and specs.

Spec quality is high. The specs I reviewed (state machine, config schema, tree addressing, pipeline stage contract, CLI commands, structural validation) are well-structured with clear sections for purpose, behavior, constraints, and examples. A contributor could implement a feature from the spec alone.

ADR quality is exceptional. The ADRs capture alternatives considered, tradeoffs accepted, and the reasoning behind decisions. They are findable via the index. I did not find ADRs that reference nonexistent code or describe superseded behavior without an update.

CONTRIBUTING.md provides a clear map of the 15 internal packages with their responsibilities, step-by-step guides for adding a CLI command and adding a validation check, test expectations (race detector, parallel tests, table-driven), and the PR process. This is exactly what a new contributor needs.

AGENTS.md (for coding agents) provides architecture context and code modification guides. The `docs/humans/` and `docs/agents/` directories contain additional documentation.

Inline documentation: package-level doc comments are present on all packages. Complex functions have comments explaining the "why" (e.g., the dual marker detection comment in `invoke/invoker.go:211`). Godoc-worthy documentation is present throughout.

**Strengths**

- README answers all four essential questions with a working quickstart.
- ADRs capture "why" decisions were made, not just "what" was decided.
- CONTRIBUTING.md provides concrete step-by-step guides for common tasks.
- Inline comments explain the "why" on complex functions.

**Weaknesses**

No significant weaknesses identified.

**Actionable Findings**

No actionable findings.

**Path to 100:** No weaknesses and no actionable findings were identified. The documentation is thorough across README, 76 ADRs, 16 specs, CONTRIBUTING.md, AGENTS.md, and inline comments. A troubleshooting section and godoc examples would be welcome additions but their absence is not a gap in a project with this level of documentation.

**Score: 97/100**

---

### 13. Open-Source Readiness

**Analysis**

License: MIT license in `LICENSE` file, referenced in the README badge. The copyright line says "2026 dorkusprime," which is consistent with the module path.

Contributing guide: `CONTRIBUTING.md` exists with package structure, step-by-step guides for adding commands and validation checks, test expectations, and PR process. This is sufficient for a first contributor.

Issue templates: bug report and feature request templates exist in `.github/ISSUE_TEMPLATE/`.

Code of conduct: Present in `CODE_OF_CONDUCT.md`, adapted from Contributor Covenant 2.1. Brief but covers the essentials.

Changelog: `CHANGELOG.md` exists with an "0.1.0 (unreleased)" section that covers core, CLI, pipeline, safety, and documentation changes. This is a good start.

Dependency hygiene: 3 direct dependencies, all necessary (Cobra for CLI, fsnotify for inbox watching, readline for interactive unblock sessions). All are pinned to specific versions in `go.mod`. `go.sum` is committed with 21 entries.

Secrets scan: No hardcoded secrets, API keys, internal URLs, or PII found in source files, test fixtures, or documentation. The default model commands reference `claude` as the CLI command, which is a public tool name, not a secret.

Leftover artifacts: `.DS_Store` exists at the repository root. It is in `.gitignore` but not committed (verified: `git ls-files` shows 0 matches). The `wolfcastle` binary is gitignored. No generated files or IDE-specific configuration is committed.

**Strengths**

- Complete open-source infrastructure: license, contributing guide, issue templates, code of conduct, changelog.
- Minimal, pinned dependencies with `go.sum` committed.
- No secrets or PII in the codebase.

**Weaknesses**

No significant weaknesses identified.

**Actionable Findings**

No actionable findings.

**Path to 100:** No weaknesses and no actionable findings were identified. `SECURITY.md` is a best-practice for mature projects but not expected at v0.1. The project ships with license, contributing guide, code of conduct, issue templates, changelog, and clean dependency hygiene.

**Score: 97/100**

---

## Summary Scorecard

| Dimension | Weight | Score (/100) | Notes |
| --------- | ------ | ------------ | ----- |
| Architectural Integrity | 12% | 92 | Unlocked propagation path in cmdutil is the only concern |
| Code Quality & Go Idiom | 10% | 97 | Exceptionally clean, idiomatic, zero dead code |
| Correctness & Safety | 14% | 93 | Atomic writes, file locking, signal handling all correct |
| Specification & ADR Fidelity | 6% | 95 | Strong alignment; one stale comment |
| Test Suite Quality | 12% | 93 | 1,763 tests, 3.19:1 ratio, property-based tests |
| CLI Design & User Experience | 8% | 96 | 38 commands, consistent JSON envelope, good help |
| Configuration System | 5% | 97 | Correct deep merge, thorough validation, sensible defaults |
| Daemon Lifecycle & Reliability | 10% | 90 | Solid supervisor, good crash recovery; force-exit bypasses cleanup |
| Validation Engine | 5% | 97 | 24 categories, multi-pass fix, JSON recovery |
| Security Posture | 5% | 96 | No shell injection, minimal deps, explicit security model |
| Build, Distribution & First-Run | 5% | 96 | Two-command build, self-contained binary, CI configured |
| Documentation Quality | 5% | 97 | Excellent README, 76 ADRs, clear contributing guide |
| Open-Source Readiness | 3% | 97 | Complete OSS infrastructure, clean dependency hygiene |
| **Weighted Total** | **100%** | **94.4** | |

---

## Weighting Rationale

| Dimension | Weight | Rationale |
| --------- | ------ | --------- |
| Architectural Integrity | 12% | Structural decisions are the hardest to change later; wrong architecture cascades into every other dimension. |
| Code Quality & Go Idiom | 10% | First impression for Go developers; affects maintainability and contributor willingness. |
| Correctness & Safety | 14% | Highest weight because a state management tool that corrupts state is worse than no tool at all. |
| Specification & ADR Fidelity | 6% | Important for maintainability but lower weight because specs are supplementary to the code. |
| Test Suite Quality | 12% | Contributors need confidence that their changes won't break things; tests are the primary trust signal. |
| CLI Design & User Experience | 8% | Models and humans both interact through the CLI; bad UX blocks adoption. |
| Configuration System | 5% | Important but relatively self-contained; configuration bugs are localized and fixable. |
| Daemon Lifecycle & Reliability | 10% | The daemon is the product's core runtime; unreliable daemons destroy user trust. |
| Validation Engine | 5% | Essential safety net but not the primary value proposition; good validation is table stakes. |
| Security Posture | 5% | Security matters but the threat model is bounded (local tool, trusted user, model access by design). |
| Build, Distribution & First-Run | 5% | Must work on first try but is easy to fix if wrong; low ongoing maintenance burden. |
| Documentation Quality | 5% | Important for adoption but documentation can be improved incrementally without code changes. |
| Open-Source Readiness | 3% | Lightweight infrastructure that can be added in a day; doesn't affect code quality. |

---

## Release Blockers

No release blockers identified.

The unlocked propagation path in `cmdutil/app.go` is a warning, not a blocker, because the race window is narrow (requires a CLI command to call `PropagateState` at the exact moment the daemon is mid-propagation for the same node) and the consequence is a stale parent state that `doctor` can repair.

---

## Release Warnings

1. **What:** `cmd/cmdutil/app.go:PropagateState` (line 95) reads and writes state files without acquiring the namespace file lock.
   **Why:** If a CLI command (e.g., `wolfcastle task complete`) uses this method while the daemon is mid-propagation, parent states could desynchronize. The `doctor` command can repair this, but the user would see inconsistent `status` output until they run `doctor`.
   **Fix:** Route all propagation through `StateStore.MutateNode` (which auto-propagates under lock) or wrap `PropagateState` in `store.WithLock`. Verify that no `cmd/` code calls `PropagateState` directly.
   **Verification:** Run the test suite after changes; add a test that simulates concurrent propagation and CLI mutation.

2. **What:** The `ValidateAll` comment in `internal/validate/engine.go:93` says "17 validation categories" but the engine checks 24.
   **Why:** A contributor reading the code would be confused by the discrepancy. Minor but signals incomplete review.
   **Fix:** Update the comment to match the actual count.
   **Verification:** `grep -n "17 validation" internal/validate/engine.go` returns no matches after fix.

---

## Commendations

1. **The `StateStore` mutation pattern** (`internal/state/store.go:MutateNode`, lines 99-145). By routing all state changes through a lock-protected callback with automatic propagation, the pattern makes concurrent corruption structurally difficult. Callers get correct locking, atomic writes, and upward propagation without having to remember any of it. Other projects that manage distributed state files should study this design.

2. **The JSON recovery system** (`internal/validate/recover.go`). The three-strategy approach (sanitize, strip trailing garbage, close truncated JSON) with data loss detection (`detectLoss`) transforms what would be a fatal "corrupt state" error into a recoverable situation with clear reporting of what was lost. This is production-grade defensive programming.

3. **The test-to-source ratio of 3.19:1 with property-based propagation tests** (`internal/state/propagation_property_test.go`). A project that tests its state propagation with random tree generation and invariant verification demonstrates a level of testing discipline that most open-source projects never achieve. The property-based tests cover the state space that manual test cases cannot.

4. **The dual marker detection system** (`internal/invoke/invoker.go:detectLineMarker` for streaming, `internal/daemon/iteration.go:scanTerminalMarker` for post-execution). The streaming detector provides immediate awareness during model execution, while the post-execution scanner uses priority ordering (COMPLETE > BLOCKED > YIELD) to resolve ambiguity when models echo their own prompt instructions. The comment at line 211 explains why both exist.

5. **76 Architecture Decision Records** that capture not just what was decided but why, including alternatives considered and tradeoffs accepted. This is the most thorough ADR practice I have seen in a project of this size.

---

## Improvement Roadmap

### Before release

No blockers. This tier is empty.

### First 30 days

- **Warning #1:** Fix the unlocked `PropagateState` path in `cmdutil/app.go`. (Effort: small. Impact: medium.)
- **Warning #2:** Update the stale category count comment in `validate/engine.go`. (Effort: trivial. Impact: low.)
- Add a `make ci` target that runs `lint test build`. (Effort: trivial. Impact: medium. File: `Makefile`.)
- Add `SECURITY.md` with vulnerability reporting instructions. (Effort: small. Impact: medium. File: `SECURITY.md`.)
- Add an integration test for concurrent daemon + inbox + CLI mutation. (Effort: medium. Impact: high. File: `test/integration/`.)

### First 90 days

- Add a `STALE_TEMP_FILE` validation category for orphaned `.wolfcastle-tmp-*` files. (Effort: small. Impact: low. Files: `internal/validate/types.go`, `internal/validate/engine.go`.)
- Replace the force-exit `os.Exit(0)` in `daemon.go:218` with a cooperative shutdown mechanism. (Effort: medium. Impact: medium. File: `internal/daemon/daemon.go`.)
- Add config schema versioning for future backward-compatibility management. (Effort: medium. Impact: medium. Files: `internal/config/config.go`, `internal/config/types.go`.)
- Add optional environment variable allowlisting for model subprocesses. (Effort: medium. Impact: medium. File: `internal/invoke/invoker.go`.)
- Consider adding stability annotations to commands before v1.0. (Effort: medium. Impact: high. Files: `cmd/` help text.)
