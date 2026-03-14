# Wolfcastle Implementation Evaluation Report

## Build Status

| Implementation | `go build ./...` | `go vet ./...` | `go test ./...` | Notes |
|----------------|:-----------------:|:--------------:|:---------------:|-------|
| Claude | Pass | Pass | 134 pass, 0 fail | Tests across 8 packages (archive, config, logging, output, pipeline, project, state, tree) |
| Gemini | Pass | Pass | 4 pass, 0 fail | Tests in 1 package only (state) |
| Codex | Pass | Pass | 91 pass, 0 fail | All tests in single root package; 8.6s runtime (integration-style tests) |

## Per-Implementation Scores

### Implementation A: Claude

| Dimension | Weight | Score (1-5) | Notes |
|-----------|:------:|:-----------:|-------|
| Spec & ADR Compliance | 20% | 4 | All critical state machine specs correct. Doctor only covers ~7 of 17 issue categories. All ADR compliance points met. |
| Code Quality | 25% | 4 | Excellent package separation (cmd/ + 11 internal packages). 134 tests. Cobra CLI. Injectable interfaces for PropagateUp. |
| Completeness | 25% | 4 | 25+ CLI commands. All core subsystems implemented. Validation engine has fewer issue types than spec'd. |
| Algorithm Correctness | 30% | 4 | recompute_parent correct for all 5 cases. Navigation correct (in_progress first, skips blocked/complete). Config merge correct. |
| **Weighted Total** | | **4.00** | |

#### Strengths
- **Best package architecture**: 11 internal packages with clear separation of concerns (state, pipeline, config, daemon, invoke, logging, tree, archive, inbox, project, output). Each package has a focused responsibility.
- **Most testable design**: `PropagateUp` in `internal/state/propagation.go:57` takes injectable `loadParent`, `saveParent`, and `getParentAddr` functions — the state machine can be tested without touching the filesystem.
- **Comprehensive test suite**: 134 tests covering state transitions, propagation (all 5 recompute cases), config merge (deep merge, arrays, null deletion), logging retention, prompt assembly, archive generation, address validation, navigation, and output envelopes.
- **Correct process group management**: `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` in `internal/invoke/invoker.go:40` per ADR-013.
- **Clean prompt assembly**: `internal/pipeline/prompt.go` implements the four-layer assembly (rules → script ref → stage prompt → iteration context) clearly and concisely.

#### Weaknesses
- **Validation engine is incomplete**: `cmd/doctor.go` only checks ~7 issue categories (orphan_index, state_mismatch, type_mismatch, missing_audit, audit_position, invalid_state, orphan_parent, orphan_child, orphan_state). Missing: MULTIPLE_AUDIT_TASKS, STALE_IN_PROGRESS, MULTIPLE_IN_PROGRESS, DEPTH_MISMATCH, NEGATIVE_FAILURE_COUNT, MISSING_REQUIRED_FIELD, MALFORMED_JSON, and more from the 17-category spec.
- **No model-assisted fix strategy**: Doctor only supports deterministic fixes. The spec calls for deterministic, model-assisted, and manual fix strategies.
- **Daemon lacks explicit self-healing**: No startup phase that scans for stale in_progress tasks and resumes them. Navigation finds in_progress tasks first (implicit self-healing), but no explicit self-heal step per ADR-020.
- **PID file management split across packages**: PID file logic is in `internal/daemon/pid.go` and `cmd/start.go`, making it harder to understand the full lifecycle.

#### Critical Deviations from Spec
- Doctor validation engine implements 7 of 17 required issue categories.
- No explicit self-healing phase on daemon startup (ADR-020 requires scanning for stale in_progress tasks).
- No atomic fix application with rollback on failure — fixes are applied individually without post-fix re-validation.

#### Wrong vs Missing
**Wrong**: Nothing — all implemented logic is correct.
**Missing/Stubbed**:
- 10 of 17 validation issue categories
- Model-assisted doctor fixes
- Explicit self-healing startup phase (implicit via navigation ordering)
- Atomic fix application with rollback
- Decomposition (leaf → orchestrator conversion) is referenced but not fully wired in the daemon

---

### Implementation B: Gemini

| Dimension | Weight | Score (1-5) | Notes |
|-----------|:------:|:-----------:|-------|
| Spec & ADR Compliance | 20% | 3.5 | All 17 validation issue types defined. recompute_parent correct. Navigation doesn't explicitly skip blocked nodes. |
| Code Quality | 25% | 2 | Only 4 tests. Many SaveNodeState/SaveRootState calls ignore errors. Dead code in log rotation. |
| Completeness | 25% | 3 | 18 CLI commands. Good validation engine. Fewer audit commands. Missing spec/install commands. |
| Algorithm Correctness | 30% | 3 | recompute_parent correct. Config merge correct. Navigation spec deviation (blocked nodes). |
| **Weighted Total** | | **2.85** | |

#### Strengths
- **Most complete validation issue taxonomy**: `internal/doctor/doctor.go:17-38` defines all 17 issue categories as typed constants (ROOTINDEX_DANGLING_REF, ROOTINDEX_MISSING_ENTRY, ORPHAN_STATE, ORPHAN_DEFINITION, PROPAGATION_MISMATCH, MISSING_AUDIT_TASK, AUDIT_NOT_LAST, MULTIPLE_AUDIT_TASKS, INVALID_STATE_VALUE, INVALID_TRANSITION_COMPLETE_WITH_INCOMPLETE, INVALID_TRANSITION_BLOCKED_WITHOUT_REASON, STALE_IN_PROGRESS, MULTIPLE_IN_PROGRESS, DEPTH_MISMATCH, NEGATIVE_FAILURE_COUNT, MISSING_REQUIRED_FIELD, MALFORMED_JSON).
- **Model-assisted doctor fixes**: `internal/doctor/doctor.go:510-572` implements a `tryModelAssistedFix` function that invokes the configured doctor model to resolve ambiguous state conflicts. This is the only implementation that actually shells out to a model for structural repair.
- **Explicit self-healing**: `internal/daemon/daemon.go:114-185` has a dedicated `selfHeal` method that scans the entire tree for in_progress tasks on startup, detects corruption (multiple in_progress), and resumes interrupted tasks.
- **Three fix strategy types**: Deterministic, model-assisted, and user-guided — matching the spec exactly.
- **Dynamic orchestrator audit**: `internal/state/state.go:254-260` automatically creates audit tasks on orchestrators when all children are complete, ensuring audit coverage without manual intervention.

#### Weaknesses
- **Critically under-tested**: Only 4 tests in the entire codebase (all in `internal/state/state_test.go`). No tests for config merge, doctor, pipeline, daemon, CLI commands, or archive. This means most logic is unverified.
- **Errors silently swallowed**: Multiple `SaveNodeState` and `SaveRootState` calls in the daemon (`daemon.go:176`, `daemon.go:367`) don't check return values. File write failures would be silently lost.
- **Config loads too many files**: `internal/config/config.go:157-163` loads from 5 paths (base/config.json, root config.json, custom/config.json, root config.local.json, local/config.json) — an overly generous interpretation of the two-file spec that could cause unexpected merge behavior.
- **Navigation doesn't skip blocked nodes**: `internal/state/state.go:214` only checks `if entry.State == "complete"` at the top of `dfsNode`. Blocked leaves are still visited and their state files loaded, though in practice the bug is mitigated because correctly-computed blocked leaves wouldn't have actionable tasks.
- **State mutation during navigation**: `internal/state/state.go:254-260` creates audit tasks and writes to disk as a side effect of the `FindNextTask` navigation function. Navigation should be a read-only operation; mutating state during traversal violates separation of concerns.

#### Critical Deviations from Spec
- Navigation (`dfsNode`) does not explicitly skip blocked nodes per the depth-first traversal spec.
- State files are mutated (audit task creation) as a side effect of navigation, which should be read-only.
- Config loading resolves 5 files instead of the spec'd 2 (config.json + config.local.json).

#### Wrong vs Missing
**Wrong**:
- Navigation does not skip blocked nodes (functionally mitigated but spec-noncompliant)
- State mutation during navigation (audit task creation in `FindNextTask`)
- Config resolution loads 5 files instead of 2

**Missing/Stubbed**:
- `spec` command (create/link)
- `install` command
- `overlap` advisory command
- Comprehensive test suite (only 4 tests exist)
- Audit gap lifecycle (open/fixed)
- Audit scope structured data

---

### Implementation C: Codex

| Dimension | Weight | Score (1-5) | Notes |
|-----------|:------:|:-----------:|-------|
| Spec & ADR Compliance | 20% | 4 | Comprehensive CLI and audit. Leaf recompute bug. Missing process group (Setpgid). |
| Code Quality | 25% | 3.5 | Zero dependencies (pure stdlib). 91 tests. Single-package design hurts modularity. |
| Completeness | 25% | 5 | Most complete implementation. 26+ CLI commands. Full audit lifecycle. Full daemon with supervisor/detach/worktree. |
| Algorithm Correctness | 30% | 3 | Orchestrator recompute correct. LEAF RECOMPUTE BUG: any blocked task blocks entire leaf. Config merge correct. |
| **Weighted Total** | | **3.83** | |

#### Strengths
- **Most complete implementation by far**: 26+ CLI commands including full audit system (breadcrumb, escalate, scope, gap, show, result, resolve), spec management (create, link), ADR management, install skill bundle, unblock, inbox, overlap advisory. Every CLI category from the spec is addressed.
- **Zero external dependencies**: `go.mod` has no dependencies — the entire CLI, daemon, and pipeline are implemented with pure standard library. This is impressive and eliminates supply chain concerns.
- **Most complete daemon lifecycle**: `daemon.go` implements PID file management (`ensureNoLivePID`), detached mode (`startDetached`), supervisor with configurable restarts (`runDaemonSupervisor`), graceful stop via stop file + SIGTERM, worktree support, stale daemon state recovery (`recoverStaleDaemonState`), and startup doctor validation that blocks on errors.
- **Most complete audit system**: Full audit lifecycle with breadcrumbs, gaps (open/fixed with deterministic IDs), escalations (with resolve), structured scope (description, files, systems, criteria), result summary, and a code audit review batch system with approval/rejection history. This goes beyond the spec.
- **Thorough doctor with inline fix**: `doctor.go` has 20+ issue types including audit-specific ones (INVALID_AUDIT_SCOPE, INVALID_AUDIT_STATUS, INVALID_AUDIT_GAP, INVALID_AUDIT_ESCALATION, AUDIT_STATUS_TASK_MISMATCH, INVALID_DAEMON_STATE, INVALID_AUDIT_REVIEW) with immediate inline fixes during the walk, including `normalizeStateValue` that accepts common typos ("completed", "done", "pending", "todo", "in-progress", "stuck").
- **Immutable config merge**: `config.go:71-93` uses `cloneJSONObject`/`cloneJSONValue` to deep-clone before merging, preventing accidental mutation of the base config. Claude and Gemini both mutate the destination map in place.

#### Weaknesses
- **CRITICAL: Leaf recompute bug**: `state_tree.go:194-196` — `recomputeLeafState` uses `case hasBlocked: node.State = "blocked"` as the first switch case. If *any* task is blocked, the entire leaf is set to blocked, even if other tasks are still `not_started`. Per spec, Mixed Blocked + Not Started should be `in_progress` because the not_started tasks can still progress. This would prevent work from advancing on leaves with a single blocked task alongside unblocked tasks.
- **Single-package architecture**: All 22 `.go` files are in the `main` package. No `internal/` or `pkg/` structure. This makes the codebase harder to understand, prevents package-level encapsulation, and means all symbols are globally accessible within the package. As the codebase grows, this would become increasingly painful.
- **Missing process group management**: `invokeStage` in `runtime_stage.go:150` creates `exec.CommandContext` without setting `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}`. Per ADR-013, model processes should be in their own process group for clean signal propagation. If the daemon receives SIGTERM, the model subprocess could be orphaned.
- **Hand-rolled CLI parsing**: `cli_args.go` and `app.go` implement manual argument parsing with switch statements. While this avoids the cobra dependency, it makes the CLI surface harder to maintain and doesn't provide automatic help generation, shell completions, or flag validation.

#### Critical Deviations from Spec
- **Leaf state recompute is WRONG**: `recomputeLeafState` at `state_tree.go:194` sets leaf to "blocked" if any single task is blocked, regardless of whether other tasks are not_started. Spec requires in_progress when there's a mix of blocked and not_started.
- **Missing process group on model invocation**: No `Setpgid` in `invokeStage`, violating ADR-013.
- **Root index schema differs**: Uses `root` as an array of root addresses plus `nodes` map, while the spec describes a flat registry keyed by tree address. The array of roots is an addition, though arguably useful.

#### Wrong vs Missing
**Wrong**:
- `recomputeLeafState` incorrectly blocks entire leaf when any task is blocked (`state_tree.go:194-196`)
- Missing process group management on model invocation (ADR-013 violation)

**Missing**:
- Package structure (everything in `main`)
- Setpgid on child processes
- The boundary for decomposition trigger in the daemon (decomposition is tracked but the prompt-based decomposition flow is partially implemented)

---

## Comparative Analysis

### Head-to-Head Summary

| Dimension | Weight | Claude | Gemini | Codex |
|-----------|:------:|:------:|:------:|:-----:|
| Spec & ADR Compliance | 20% | 4 | 3.5 | 4 |
| Code Quality | 25% | 4 | 2 | 3.5 |
| Completeness | 25% | 4 | 3 | 5 |
| Algorithm Correctness | 30% | 4 | 3 | 3 |
| **Weighted Total** | | **4.00** | **2.85** | **3.83** |

### Notable Differences

**State mutation during navigation**: Gemini creates audit tasks as a side effect of `FindNextTask` (`internal/state/state.go:254-260`), mutating state during what should be a read-only navigation. Claude and Codex keep navigation purely read-only.

**Blocked node handling divergence**: All three interpret "blocked" differently in edge cases:
- Claude: Correctly skips blocked nodes in navigation and computes leaf/orchestrator state with the Mixed Blocked + Not Started → In Progress rule.
- Gemini: Correct recompute but doesn't skip blocked nodes in navigation.
- Codex: Correct orchestrator recompute but incorrect leaf recompute (any blocked → blocked).

**Doctor philosophy**: Claude checks ~7 categories with auto-fix. Gemini defines all 17 categories with three fix strategies (including model-assisted). Codex has 20+ categories with inline fix during walk plus state normalization.

**Config merge safety**: Codex clones values before merging (`cloneJSONObject`/`cloneJSONValue`). Claude and Gemini mutate the destination map in place, which is safe for their use cases but less defensive.

**Dependency strategy**: Claude and Gemini use cobra for CLI. Codex uses zero dependencies and hand-rolls everything. Both approaches are defensible — cobra provides better UX (help, completions) while zero-dep reduces supply chain risk.

### Common Mistakes

1. **No implementation achieves atomic fix application with rollback**: The spec requires "All fixes to one file applied as single write" and "Post-fix re-validation; rollback if new issues introduced." None of the three implements post-fix re-validation with rollback. All apply fixes sequentially without checking if the fix introduced new problems.

2. **Decomposition trigger not fully wired**: All three track failure counts and decomposition depth, but none fully implements the model-prompted decomposition workflow where a leaf is converted to an orchestrator with new child leaves. This suggests the decomposition spec may need more concrete implementation guidance.

3. **Root index schema interpretation varies**: The spec describes "a flat registry keyed by tree address with node metadata." Claude uses `Nodes map[string]IndexEntry` with a `Parent` field per entry. Gemini uses a similar structure. Codex uses `Root []string` (array of root addresses) plus `Nodes map[string]IndexNode`. The array-of-roots approach is an addition not in the spec, but is arguably useful for avoiding a tree walk to find roots.

### Ranking

1. **Claude** — Most balanced: correct algorithms, clean architecture, comprehensive tests, and solid spec compliance across all dimensions. No bugs found in critical logic.
2. **Codex** — Most ambitious scope with the most complete feature set, but a critical bug in leaf state recompute undermines the state machine — the highest-priority spec requirement.
3. **Gemini** — Strong validation engine design with model-assisted fixes, but critically under-tested (4 tests) and has navigation/state-mutation issues that suggest insufficient verification of implemented logic.

## Overall Interpretation

### Implementation Philosophies

**Claude took a "correct core, expand outward" approach.** The state machine, propagation, and config merge were implemented first and thoroughly tested (propagation_test.go covers all 5 recompute cases). The architecture was designed for testability from the start — `PropagateUp` takes injectable functions rather than hardcoded filesystem access. CLI commands were built on top of this tested core. The result is a codebase where the most critical logic is the most verified, and the boundaries for missing features (more doctor checks, decomposition) are clean. The trade-off is that some secondary features (full validation categories, model-assisted fixes) weren't reached.

**Codex took a "build everything, ship wide" approach.** The implementation covers nearly every feature in the spec — 26+ CLI commands, full audit lifecycle, supervisor daemon, worktree support, code audit review batches — in a single package. The breadth is remarkable, and the 91 integration-style tests verify end-to-end flows. However, the rush to cover surface area introduced a critical bug in the leaf state recompute that affects the core state machine. The single-package design suggests the priority was getting features working quickly rather than establishing architectural boundaries.

**Gemini took a "design the architecture, fill in later" approach.** The most telling evidence is the 17-constant validation issue taxonomy in `doctor.go` — Gemini clearly read the spec carefully and defined the full problem space before implementing fixes. The model-assisted fix strategy shows architectural ambition. But the implementation was not followed through: only 4 tests exist, many error returns are ignored, and the navigation has a spec deviation. The codebase reads like a first draft by someone who understood the design but ran out of time for verification.

### Strengths Worth Adopting

- **From Claude**: The injectable `PropagateUp` function (`internal/state/propagation.go:57-110`) should be the model for testable state operations. Instead of hardcoding filesystem access, it takes `loadParent`, `saveParent`, and `getParentAddr` functions, enabling unit tests that run in microseconds without temp directories. Gemini and Codex both couple propagation directly to filesystem I/O.

- **From Codex**: The `normalizeStateValue` function (`doctor.go:647-660`) that accepts typos like "completed", "done", "pending", "todo", "in-progress", "stuck" and maps them to canonical values is genuinely useful for self-healing. Neither Claude nor Gemini handles this edge case. Also, `cloneJSONObject`/`cloneJSONValue` (`config.go:95-114`) should be standard practice for config merging.

- **From Codex**: The daemon supervisor pattern (`daemon.go:185-207`) with configurable max restarts and restart delay is production-quality. Claude and Gemini lack crash recovery at the daemon level.

- **From Gemini**: The `tryModelAssistedFix` function (`internal/doctor/doctor.go:510-572`) is the only implementation that actually invokes a model for structural repair, matching the spec's three-tier fix strategy. The prompt design (providing the node address and issue description, requesting JSON resolution) is straightforward and effective.

- **From Codex**: The full audit review batch system with approval/rejection flow and history (`auditcmd.go`, `audit_test.go`) goes beyond the spec and provides a practical workflow for audit governance. This is a genuinely useful addition.

### Weaknesses and Pitfalls

**Claude**: The validation engine's ~7 issue categories vs 17 spec'd is the biggest gap. If extended, the current `cmd/doctor.go` implementation is a single large `RunE` function (~350 lines) that would benefit from the composable `Check` interface pattern the spec describes. The monolithic approach would make adding new issue types increasingly painful.

**Gemini**: The 4-test codebase is the most urgent problem. Every significant code path is unverified. The error-swallowing pattern (`SaveNodeState(...)` without checking errors) could cause silent data corruption in production — a state write could fail, the daemon would continue with stale state in memory, and subsequent operations would diverge from disk. The state mutation in `FindNextTask` is a structural issue that would be difficult to refactor out because downstream code may depend on audit tasks being created during navigation.

**Codex**: The single-package architecture is a time bomb. With 22 `.go` files and 2000+ lines of types, state logic, CLI dispatch, daemon lifecycle, pipeline execution, audit commands, and doctor validation all sharing a single namespace, any new contributor would struggle to understand boundaries. The leaf recompute bug (`state_tree.go:194`) is particularly concerning because it's in a function called from multiple places (doctor, runtime_mutation, state transitions) — fixing it correctly requires understanding all callers.

### Cross-Pollination Opportunities

If assembling an ideal implementation from the best parts of all three:

- **Package structure**: Claude's `cmd/` + `internal/` layout with 11 focused packages. This provides the best balance of modularity and navigability.
- **State machine logic**: Claude's `internal/state/propagation.go` — correct algorithms with injectable interfaces. Drop in Codex's `recomputeOrchestratorState` (which is identical in logic) but fix `recomputeLeafState` to match Claude's approach.
- **Test suite**: Start with Codex's 91 integration tests for end-to-end coverage, then add Claude's 134 unit tests for fast feedback on core algorithms. Gemini's 4 tests are not usable.
- **Error handling**: Claude's pattern of wrapping errors with context (`fmt.Errorf("...: %w", err)`) throughout the codebase.
- **Daemon lifecycle**: Codex's supervisor pattern with PID management, detach mode, stop file, worktree support, and stale state recovery. Augment with Gemini's explicit self-healing scan.
- **Validation engine**: Gemini's 17-category taxonomy with three fix strategies as the framework, filled in with Codex's detailed inline fix logic and state normalization.
- **Config merge**: Codex's clone-before-merge approach for safety.
- **CLI framework**: Claude's cobra-based CLI for UX quality (help, completions, flag validation).
- **Audit system**: Codex's full audit lifecycle (breadcrumbs, gaps, escalations, scope, review batch, history).
- **Prompt assembly**: Claude's clean four-layer implementation with three-tier fragment resolution.
- **Model invocation**: Claude's `internal/invoke/invoker.go` with process group management and streaming support.

### Spec Feedback

Based on seeing three independent interpretations:

1. **Leaf state computation rules are ambiguous**: The spec describes `recompute_parent()` for orchestrators but doesn't explicitly state whether the same rules apply to leaf nodes deriving state from tasks. Codex interpreted "any blocked task blocks the leaf" while Claude applied the same orchestrator rules. The spec should explicitly state: "Leaf node state is derived from its tasks using the same algorithm as orchestrator state from children."

2. **Root index schema underspecified**: The spec says "flat registry keyed by tree address" but doesn't specify how to discover root nodes without walking the entire registry. Codex added a `root` array to solve this; Claude and Gemini walk the map looking for entries with no parent. The spec should specify whether a `root` or `top_level` field exists.

3. **Navigation as read-only operation not stated**: Gemini created audit tasks as a side effect of navigation. The spec should explicitly state that navigation is a read-only operation that must not modify state.

4. **Decomposition trigger mechanism underspecified**: All three track failure counts and decomposition depth, but none fully implements the model-prompted decomposition flow. The spec describes *when* decomposition should trigger but not the exact mechanism — does the daemon prompt the model with a special decomposition prompt? Does it call a CLI command? This needs clarification.

5. **Config file paths**: The spec says "Two files: config.json and config.local.json" but also "Three-tier file layering (base/custom/local)". Gemini loaded 5 files trying to honor both. The spec should clarify: are config files subject to three-tier layering (base/config.json → custom/config.json → local/config.json) or are they only the two root-level files?

6. **Atomic fix application**: The spec describes "Post-fix re-validation; rollback if new issues introduced" but all three implementations skip this. The spec should provide more guidance on what "rollback" means — restore from backup? Undo in memory? The ambiguity led all three to implement best-effort fixes without rollback.

### Model Tendencies

**Claude (as code generator)**:
- **Breadth vs depth**: Balanced. Built a correct core first, then expanded outward. Didn't try to cover every feature but what was built works.
- **Specification adherence**: High. Followed the spec closely and correctly for all implemented features. No improvised additions that contradict the spec.
- **Code style**: Clean and conventional. Good use of Go idioms (error wrapping, interfaces, package boundaries). Neither over-engineered nor under-engineered.
- **Testing discipline**: Strong. Wrote tests proactively for critical paths. Tests are meaningful — propagation_test.go covers all 5 recompute_parent cases.
- **Error handling**: Robust. Consistent `fmt.Errorf("...: %w", err)` wrapping throughout.

**Gemini (as code generator)**:
- **Breadth vs depth**: Architecture-first. Designed the full problem space (17 issue types, three fix strategies) but didn't fill in the implementation depth (4 tests, swallowed errors).
- **Specification adherence**: Strong on taxonomy, weak on behavioral correctness. Read the spec carefully enough to enumerate all validation categories but didn't verify the navigation implementation matches the spec.
- **Code style**: Verbose and somewhat scattered. The state.go file (345 lines) mixes navigation, propagation, loading, and saving without clear boundaries.
- **Testing discipline**: Very weak. 4 tests for the entire codebase suggests testing was an afterthought.
- **Error handling**: Optimistic. Multiple critical write operations ignore return values.

**Codex (as code generator)**:
- **Breadth vs depth**: Extreme breadth. Implemented nearly every feature in the spec including some additions (audit review batches, state normalization). The trade-off was a critical bug in core logic.
- **Specification adherence**: High coverage but with deviations. Hit more spec requirements than either competitor but introduced errors in the process (leaf recompute, missing Setpgid).
- **Code style**: Pragmatic and dense. Functions are focused and well-named, but the single-package architecture means no structural boundaries. Zero dependencies shows confidence in Go's stdlib.
- **Testing discipline**: Good. 91 tests with integration-style coverage that exercises real filesystem operations. Tests caught many issues but missed the leaf recompute bug.
- **Error handling**: Good. Contextual error messages throughout. Uses `fmt.Errorf` with wrapping consistently.
