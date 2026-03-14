# Wolfcastle Implementation Evaluation Report

## Build Status

| Implementation | `go build ./...` | `go vet ./...` | `go test ./...` | Notes |
|----------------|:-----------------:|:--------------:|:---------------:|-------|
| Claude | Pass | Pass | 152 pass, 0 fail | 90 Go files, 11,390 lines; 15 test files, 3,171 test lines |
| Gemini | Pass | Pass | 17 pass, 0 fail | 40 Go files, 6,571 lines; 6 test files, 814 test lines |
| Codex | Pass | Pass | 94 pass, 0 fail | 23 Go files, 9,783 lines; 3 test files, 3,208 test lines |

All three implementations compile, pass vet, and pass all tests. No build failures or warnings.

---

## Per-Implementation Scores

### Implementation A: Claude

| Dimension | Weight | Score (1-5) | Notes |
|-----------|:------:|:-----------:|-------|
| Spec & ADR Compliance | 20% | 4 | Faithful to nearly all specs and ADRs. Minor gap: audit lifecycle sync (pending/in_progress/passed/failed) not implemented; failure escalation logs decomposition threshold but doesn't trigger it. |
| Code Quality | 25% | 5 | Exemplary Go: idiomatic error wrapping, composable Engine pattern for validation, clean package boundaries (state/config/validate/pipeline/daemon/invoke/tree), Cobra CLI, only stdlib + Cobra deps. 152 passing tests across 8 packages. |
| Completeness | 25% | 5 | Most complete of the three. All CLI commands implemented including version, completions, overlap advisory. Full validation engine (17 categories), archive, inbox, specs, ADRs, three-tier prompt layering, NDJSON logging with rotation, daemon with PID/signals/self-healing. |
| Algorithm Correctness | 30% | 4 | recompute_parent: correct (all 9 test cases pass). Config merge: correct. DFS navigation: correct. Failure escalation: partially correct (threshold check uses `==` — fires only once; no decomposition action). Audit propagation: core primitives present but missing lifecycle state machine. |
| **Weighted Total** | | **4.45** | |

#### Strengths
- **Composable validation engine** (`internal/validate/engine.go`): Uses an `Engine` struct with category-filtered `include()` method. `ValidateAll` and `ValidateStartup` share the same engine with different category sets. This is the cleanest implementation of the spec's composable `Check` interface requirement.
- **Highest test coverage**: 152 tests across state machine (mutations, navigation, propagation), config (merge, validation, loading), pipeline (prompt assembly, fragment resolution, iteration context), archive, logging, output, project scaffolding, tree addressing, and validation engine. Tests are well-structured with parallel execution and helper factories.
- **Three-tier fragment resolution** (`internal/pipeline/fragments.go`): `ResolveFragment` and `ResolveAllFragments` with include/exclude lists, ordering, and proper local→custom→base layering — most faithful to the spec's prompt assembly contract.
- **Atomic writes everywhere**: temp-file-rename pattern in `internal/state/io.go` used consistently.
- **Only external dependency**: Cobra (+ pflag). No unnecessary third-party packages.

#### Weaknesses
- **Failure escalation incomplete**: `internal/daemon/daemon.go` lines 360-391 — when `failCount == threshold AND depth < max`, logs "decomposition_threshold" but does NOT initiate decomposition or block the task. The `==` operator means this check fires exactly once. Task continues failing indefinitely until hard cap.
- **No audit lifecycle sync**: Audit state (pending/in_progress/passed/failed) is not automatically derived from task state. No mechanism to block task completion when open gaps exist.
- **Config validation tests are thorough but navigation tests use map fallback**: `internal/state/navigation.go` line 35 falls back to iterating `idx.Nodes` map when `idx.Root` is empty — Go map iteration is non-deterministic.

#### Critical Deviations from Spec
- **ADR-019 (Failure Handling)**: Decomposition is never triggered. The spec says `failure_count = threshold AND depth < max → prompt decomposition`. Claude logs the condition but takes no action.
- **Audit lifecycle**: Spec defines audit status progression (pending→in_progress→passed/failed) synchronized with task state. Claude stores audit data but doesn't implement the lifecycle state machine.

#### Wrong vs Missing
**Wrong (attempted but incorrect):**
- Failure threshold check uses `==` instead of `>=` — fires once, not on every subsequent failure past threshold.

**Missing (clean boundaries, stubs):**
- Audit lifecycle state machine (pending/in_progress/passed/failed sync with task state)
- Decomposition action when failure threshold reached at allowable depth
- Expand/file stage handlers in daemon (stubs exist, non-fatal)

---

### Implementation B: Gemini

| Dimension | Weight | Score (1-5) | Notes |
|-----------|:------:|:-----------:|-------|
| Spec & ADR Compliance | 20% | 3 | Follows core specs but has multiple subtle deviations: non-standard "escalated" gap status, direct state override in failure handling, non-deterministic navigation. |
| Code Quality | 25% | 3 | Reasonable Go structure (internal/ with state/config/daemon/pipeline/cli/doctor packages). Monolithic doctor and CLI files. Only 17 tests with 814 lines — weakest test coverage. Cobra dependency like Claude. |
| Completeness | 25% | 4 | All core features present: state machine, config, daemon, pipeline, audit, CLI commands, archive, doctor. Some commands are skeletal (unblock, follow, inbox). Missing version/completions commands. |
| Algorithm Correctness | 30% | 3 | recompute_parent: correct. Config merge: correct. DFS navigation: partially correct (non-deterministic root ordering). Failure escalation: partially correct (no decomposition + direct node state override bypassing recompute). Doctor: buggy orphan detection. |
| **Weighted Total** | | **3.25** | |

#### Strengths
- **Git integration** (`internal/git/git.go`): Auto-commit, branch detection, worktree setup/cleanup — most production-ready git workflow of the three.
- **Worktree support**: `wolfcastle start --worktree` creates isolated git worktrees for branch-scoped execution with cleanup.
- **Identity auto-detection** (`internal/identity/identity.go`): Clean separation of user-machine namespace detection.
- **Config merge**: Functionally identical to Claude — correct deep merge, array replacement, null deletion.

#### Weaknesses
- **Lowest test coverage**: Only 17 tests across 6 test files (814 lines). No tests for daemon, pipeline, CLI commands, git integration, identity, or scripts.
- **Non-deterministic navigation**: `internal/state/navigation.go` lines 16-20 iterates `rootState.Nodes` map to discover top-level nodes. Go map iteration order is random, meaning which root node is processed first is non-deterministic across runs.
- **Direct state override in failure escalation**: `internal/daemon/daemon.go` line 510-511 sets `nodeState.State = "blocked"` directly, bypassing `recomputeParent()`. If other tasks in the leaf are still actionable, the node state will be incorrectly set to blocked.
- **Buggy orphan detection**: `internal/doctor/doctor.go` line 162-168 checks `!expectedDirs[entry.Parent] && rootState.Nodes[entry.Parent].Name == ""`. A parent with a valid name but that doesn't list the child in its `children` array would NOT be detected as an orphan.
- **Monolithic functions**: Doctor check is a single 283-line function. CLI commands are in large files without much decomposition.

#### Critical Deviations from Spec
- **Non-standard gap status**: `internal/cli/audit.go` line 406 introduces `"escalated"` as a gap status when escalating. The spec defines only `"open"` and `"fixed"`. This would confuse validation and cross-implementation interop.
- **Direct node state mutation**: Failure escalation bypasses the propagation algorithm, violating the upward-only propagation invariant.
- **ADR-019**: Like Claude, no decomposition is triggered at threshold with allowable depth.

#### Wrong vs Missing
**Wrong (attempted but incorrect):**
- Failure escalation sets `nodeState.State = "blocked"` directly instead of blocking the task and letting `recomputeParent()` derive the node state.
- Navigation has non-deterministic root ordering from Go map iteration.
- Doctor orphan detection logic is semantically incorrect.
- Escalation creates non-standard `"escalated"` gap status.

**Missing (clean boundaries):**
- Decomposition trigger at failure threshold
- Audit lifecycle state machine
- Comprehensive test suite (only 17 tests)
- Version/completions CLI commands

---

### Implementation C: Codex

| Dimension | Weight | Score (1-5) | Notes |
|-----------|:------:|:-----------:|-------|
| Spec & ADR Compliance | 20% | 4 | Strong spec adherence. Audit lifecycle fully implemented. No non-standard extensions. Minor: no composable Check interface (monolithic doctor). |
| Code Quality | 25% | 3 | All code in a single package (no internal/ hierarchy) — flat structure with 23 files. Good error handling. Zero external dependencies (no Cobra — hand-rolled CLI). 94 tests but all in one package. Naming is clear but package boundaries are absent. |
| Completeness | 25% | 4 | Core features complete: state machine, config, daemon, pipeline, audit with full lifecycle, doctor (25+ check categories), archive, status. Some commands incomplete (inbox list/clear, unblock partially connected). |
| Algorithm Correctness | 30% | 4 | recompute_parent: correct (both leaf and orchestrator variants). Config merge: correct. DFS navigation: correct with explicit sort for determinism. Failure escalation: partially correct (no decomposition but proper state recompute). Audit propagation: most correct — full lifecycle with gap-blocks-completion. |
| **Weighted Total** | | **3.75** | |

#### Strengths
- **Audit lifecycle state machine** (`state_tree.go` lines 213-263, `syncAuditLifecycle`): The most sophisticated audit implementation. Syncs audit task state to audit status (pending/in_progress/passed/failed), blocks audit task completion when open gaps exist, tracks timestamps. This is the only implementation that correctly prevents marking audit as "passed" while gaps are open.
- **Most thorough doctor** (`doctor.go`): 25+ issue categories, well beyond the 17 spec minimum. Includes audit-specific checks (MISSING_AUDIT_OBJECT, INVALID_AUDIT_SCOPE, INVALID_AUDIT_STATUS, INVALID_AUDIT_GAP with sub-checks, INVALID_AUDIT_ESCALATION, AUDIT_STATUS_TASK_MISMATCH, INVALID_DAEMON_STATE).
- **Deterministic navigation** (`state_tree.go` line 351): Explicitly sorts children by slug before traversal — ensures deterministic left-to-right ordering regardless of storage order. Claude and Gemini rely on stored array order.
- **Zero external dependencies**: Hand-rolled CLI with no Cobra. `go.mod` has only the Go version declaration. Impressive self-sufficiency.
- **Marker-based mutation** (`runtime_mutation.go`): Comprehensive model output parsing with 10+ marker types (WOLFCASTLE_COMPLETE, WOLFCASTLE_BLOCK, WOLFCASTLE_DECOMPOSE, WOLFCASTLE_GAP, WOLFCASTLE_FIX_GAP, WOLFCASTLE_SCOPE*, WOLFCASTLE_SUMMARY, WOLFCASTLE_BREADCRUMB, WOLFCASTLE_RESOLVE_ESCALATION). Each mutation is atomic with proper propagation.
- **94 tests with 3,208 lines**: Second-highest test count, highest test line count. Covers project/task flow, nested depth tracking, state transitions, navigation ordering, block/unblock, decomposition, audit gap lifecycle, overflow checks, doctor repairs.

#### Weaknesses
- **Flat package structure**: All 23 `.go` files live in a single `main` package. No `internal/` hierarchy, no separation between state, config, daemon, CLI, validation, pipeline. This makes the codebase harder to reason about at scale and impossible to import individual packages.
- **Hand-rolled CLI**: While zero-dependency is admirable, the CLI parsing (`cli_args.go`) lacks shell completions, help formatting, and subcommand grouping that Cobra provides. The `help.go` file manually builds help text.
- **No package-level testability**: Since everything is in one package, tests can access all internal state. This makes it impossible to verify that the public API surface is sufficient — tests may rely on internal implementation details.
- **Legacy alias handling**: `normalizeAuditState` (`state_tree.go` lines 46-62) syncs between top-level and nested audit fields, suggesting the data model evolved during development. This is technical debt.

#### Critical Deviations from Spec
- **No composable Check interface**: Spec calls for a composable `Check` interface with `TreeState` abstraction. Codex uses a monolithic `runDoctor()` function. While it exceeds the spec in thoroughness, it doesn't match the specified architecture.
- **ADR-019**: Like the others, no decomposition trigger. But Codex's `describeFailurePolicy` function at least communicates the policy to the model via prompt context.

#### Wrong vs Missing
**Wrong (attempted but incorrect):**
- No significant correctness issues. The failure escalation missing decomposition is a gap, not a bug in what's implemented.

**Missing (clean boundaries):**
- Decomposition trigger at failure threshold (though policy is communicated to model)
- `inbox list` and `inbox clear` CLI commands
- Composable Check interface for validation
- Package-level separation (everything in `main` package)

---

## Comparative Analysis

### Head-to-Head Summary

| Dimension | Weight | Claude | Gemini | Codex |
|-----------|:------:|:------:|:------:|:-----:|
| Spec & ADR Compliance | 20% | 4 | 3 | 4 |
| Code Quality | 25% | 5 | 3 | 3 |
| Completeness | 25% | 5 | 4 | 4 |
| Algorithm Correctness | 30% | 4 | 3 | 4 |
| **Weighted Total** | | **4.45** | **3.25** | **3.75** |

### Notable Differences

**State propagation**: All three implementations arrived at the same correct algorithm through slightly different code structures. Claude checks `allNotStarted` first, Gemini checks `allComplete` first, Codex checks `allNotStarted` first. Functionally equivalent.

**Config merge**: All three implementations are essentially identical — deep merge objects, replace arrays, null deletes. This is the area of strongest convergence.

**Navigation determinism**: Claude uses a `Root` array in the index for deterministic ordering (with a non-deterministic map fallback). Gemini has no deterministic ordering at all. Codex explicitly sorts children by slug. Only Codex's approach is robust in all cases.

**Audit depth**: Claude implements breadcrumbs, gaps, and escalations as primitives but not the lifecycle state machine. Gemini adds a non-standard "escalated" gap status. Codex implements the full lifecycle with `syncAuditLifecycle` including gap-blocks-completion logic.

**Package structure**: Claude uses a well-organized `internal/` hierarchy (state, config, validate, pipeline, daemon, invoke, tree, logging, output, inbox, archive, project). Gemini uses `internal/` with fewer packages. Codex uses a flat single-package structure.

**Dependencies**: Claude and Gemini use Cobra (the standard Go CLI framework). Codex has zero external dependencies.

### Common Mistakes

All three implementations share these gaps, suggesting spec ambiguity:

1. **No actual decomposition trigger**: All three detect when `failure_count >= threshold AND depth < max` but none actually initiate decomposition. Claude logs it, Codex communicates the policy to the model via prompt, Gemini does nothing. The spec says "prompt decomposition" but doesn't specify the mechanism clearly enough — is the Go code supposed to call a decomposition function, or is it supposed to include the policy in the next model prompt and let the model decide?

2. **Audit lifecycle as separate concern**: The spec defines audit status progression (pending→in_progress→passed/failed) but only Codex implements it. This suggests the spec's audit lifecycle may not be prominent enough in the documentation, or that it's seen as a lower-priority feature.

3. **Summary stage conditionality**: All three implement summary as optional/conditional, but the exact trigger conditions differ. The spec says "conditional and opt-out" but the conditions for when summary runs vs. is skipped are interpreted differently.

### Ranking

1. **Claude** — Best overall code quality, most complete feature set, strongest test coverage (152 tests), clean architecture with composable patterns. Minor algorithmic gaps don't overcome the structural advantages.
2. **Codex** — Best algorithm correctness (deterministic navigation, full audit lifecycle, proper state recompute in failure handling). Zero-dependency implementation is impressive. Flat package structure is the main weakness.
3. **Gemini** — Solid foundation but multiple correctness bugs (non-deterministic navigation, direct state override, buggy orphan detection, non-standard gap status) and weakest test coverage (17 tests) place it third.

---

## Overall Interpretation

### Implementation Philosophies

**Claude** approached this as an architect would: establish clean package boundaries first, build composable infrastructure (the validation Engine pattern, the Resolver for tree addresses, the pipeline builder), then fill in features methodically. The result is the most feature-complete implementation with the strongest code quality. Claude wrote tests alongside features — 152 tests across 8 packages covering state mutations, navigation, propagation, config merge/validation/loading, pipeline prompt assembly, archive generation, logging rotation, and validation categories. The investment in infrastructure paid off in completeness.

**Gemini** took a pragmatic middle path: establish reasonable package structure, implement the core state machine and daemon loop, then build outward toward CLI and peripheral features. Gemini's git integration (auto-commit, worktree support) is the most production-ready of the three, suggesting attention to the operational aspects of the tool. However, Gemini underinvested in testing (17 tests) and has several subtle correctness bugs that suggest insufficient self-verification.

**Codex** focused on getting the runtime behavior right: the daemon loop, model invocation, marker parsing, and mutation atomicity received the most attention. The single-package structure suggests Codex prioritized getting things working over architectural purity. The result is the most correct runtime behavior (deterministic navigation, full audit lifecycle, proper state propagation in all mutation paths) but in a structure that would be harder to maintain at scale. Codex wrote focused, high-value tests (94 tests, 3,208 lines) covering the critical paths.

### Strengths Worth Adopting

- **From Claude**: The composable validation engine (`internal/validate/engine.go`) with its `include()` filter method is the cleanest implementation of the spec's composable Check interface. `ValidateAll` and `ValidateStartup` share the same engine infrastructure with different category sets — adding a new validation category requires only adding a constant and a check function. Gemini and Codex both use monolithic doctor functions. Additionally, Claude's three-tier fragment resolution (`internal/pipeline/fragments.go`) with include/exclude ordering is the most faithful to the prompt assembly spec.

- **From Gemini**: The git integration module (`internal/git/git.go`) with auto-commit, branch detection, and worktree lifecycle management is the most production-complete. Claude and Codex have git support but Gemini's is better encapsulated.

- **From Codex**: The `syncAuditLifecycle` function (`state_tree.go` lines 213-263) correctly implements the full audit lifecycle state machine, including the critical invariant that audit task completion is blocked when open gaps exist. Neither Claude nor Gemini implement this. Also, Codex's explicit `sort.Slice(node.Children, ...)` before navigation traversal (line 351) ensures deterministic ordering — a simple one-line fix that the other two should adopt.

### Weaknesses and Pitfalls

- **Claude**: The failure escalation threshold check using `==` instead of `>=` (`internal/daemon/daemon.go` line 367) is a subtle bug that would be hard to diagnose in production — the decomposition prompt would fire exactly once at failure count 10, then never again as the count climbs toward the hard cap at 50. The 40 failures between threshold and hard cap would be wasted iterations.

- **Gemini**: The direct `nodeState.State = "blocked"` assignment in failure handling (`internal/daemon/daemon.go` line 510-511) bypasses the recompute_parent algorithm. If a leaf has two tasks — one blocked at hard cap and one still not_started — the leaf would be incorrectly marked "blocked" instead of "in_progress". This is a state corruption bug that could freeze the daemon on a tree that still has actionable work. The non-deterministic navigation ordering is also problematic for reproducibility.

- **Codex**: The flat package structure means all 9,783 lines of code share a single namespace. There are no import boundaries to prevent coupling between, say, the daemon loop and the CLI argument parser. As the codebase grows, this will make refactoring increasingly difficult. It also means external tools cannot import individual subsystems (e.g., using the state machine in a test harness).

### Cross-Pollination Opportunities

If assembling an ideal implementation from the best parts of all three:

- **Package structure**: Use Claude's `internal/` hierarchy as the skeleton. It has the cleanest separation: `state/` (machine + persistence), `config/` (loading + merge + validation), `validate/` (composable engine), `pipeline/` (prompt assembly), `daemon/` (lifecycle), `invoke/` (model shell-out), `tree/` (addressing + resolution), `logging/` (NDJSON), `output/` (JSON envelope), `archive/`, `inbox/`, `project/`.

- **State machine logic**: Use Claude's `recomputeState` + `propagateUp` for the core algorithm (correct and well-tested), but adopt Codex's `syncAuditLifecycle` as an additional propagation step after every state change.

- **Navigation**: Use Codex's `navigateAddress` with its explicit `sort.Slice` for determinism.

- **Validation engine**: Use Claude's composable `Engine` with category filtering, but incorporate Codex's extended check categories (25+ vs 17) as additional check functions plugged into the engine.

- **Test suite**: Start with Claude's 152-test suite for breadth, supplement with Codex's focused runtime tests (decomposition, audit gap lifecycle, doctor repairs) for depth.

- **Error handling**: Claude's patterns — `fmt.Errorf("...: %w", err)` throughout, fail-fast, descriptive messages.

- **Git integration**: Gemini's `internal/git/git.go` module.

- **Audit propagation**: Codex's full lifecycle (`syncAuditLifecycle`, `normalizeAuditState`, gap-blocks-completion).

- **Model invocation**: Codex's marker parsing in `runtime_mutation.go` is the most comprehensive (10+ marker types with atomic per-marker mutations).

### Spec Feedback

Based on seeing three independent interpretations:

1. **Decomposition trigger mechanism is underspecified**: All three failed to implement decomposition at `failure_count = threshold AND depth < max`. The spec says "prompt decomposition" but doesn't clarify: does the Go daemon automatically call `project create` to split the leaf into children? Does it add a special prompt to the next model invocation asking the model to decompose? Does it block the task with a message suggesting decomposition? The spec should explicitly define the mechanism — likely "set a `needs_decomposition` flag that the next pipeline iteration's prompt includes, letting the model decide whether and how to decompose."

2. **Audit lifecycle priority**: The audit status state machine (pending→in_progress→passed/failed) and the gap-blocks-completion invariant are critical correctness properties but appear to be buried in the audit spec. Two of three implementations missed them entirely. Consider promoting these to the state machine spec where they'd be more prominent.

3. **Navigation ordering guarantee**: The spec says "depth-first" but doesn't specify whether children must be processed in a deterministic order (e.g., alphabetical by slug, or insertion order). Two of three implementations have non-deterministic or order-dependent behavior. The spec should state: "Children are traversed in lexicographic order by slug."

4. **Gap status enum**: The spec defines `"open"` and `"fixed"` as the only gap statuses. Gemini invented `"escalated"`. If the spec intends escalation to be tracked separately from the gap's lifecycle status (which it does — escalations have their own array), this should be stated more explicitly: "A gap's status is always either 'open' or 'fixed'. Escalation does not change the gap's status."

5. **Failure escalation operator**: The spec uses `=` (equals) for the threshold check: "failure_count = threshold AND depth < max." One implementation interpreted this as `==` (exact match), which only fires once. The spec should use `>=` or explicitly state: "At and beyond the threshold, decomposition is prompted on every failure until the task is blocked or decomposed."

### Model Tendencies

**Claude (claude-opus-4-6)**:
- **Breadth vs depth**: Breadth-first. Scaffolded the complete feature surface (42 commands, 8 internal packages, base prompt templates) before drilling into algorithmic correctness. This produced the most complete implementation but left some algorithms at 80% correctness.
- **Specification adherence**: High adherence. Faithfully implemented nearly every spec requirement including minor ones (version command, completions, overlap advisory). Made very few unauthorized additions.
- **Code style**: Balanced. Neither over-engineered nor under-engineered. The composable Engine pattern shows architectural thinking without excessive abstraction. Clean, readable Go.
- **Testing discipline**: Strongest. 152 tests written alongside features, covering all major packages. Tests are well-structured with helpers and parallel execution.
- **Error handling**: Excellent. Consistent `fmt.Errorf("...: %w", err)` wrapping with descriptive context throughout.

**Gemini (gemini-2.5-pro)**:
- **Breadth vs depth**: Middle ground. Implemented most features but some are skeletal. Invested in operational concerns (git integration, worktree support) that the other two treated as secondary.
- **Specification adherence**: Moderate. Generally follows the spec but introduced non-standard extensions (escalated gap status) and made incorrect shortcuts (direct state assignment). More "spirit of the spec" than "letter of the spec."
- **Code style**: Pragmatic but occasionally hasty. Monolithic functions (283-line doctor check), less decomposition than Claude. Reasonable package structure but fewer abstractions.
- **Testing discipline**: Weakest. Only 17 tests. Suggests testing was deprioritized in favor of feature breadth — a risky tradeoff that left multiple bugs undetected.
- **Error handling**: Adequate but less consistent. Some errors wrapped, some bare. The direct state mutation in failure handling is a symptom of insufficient defensive programming.

**Codex (o4-mini)**:
- **Breadth vs depth**: Depth-first. Focused intensely on getting runtime behavior correct — the daemon loop, mutation atomicity, audit lifecycle, marker parsing. Less attention to package architecture and CLI polish.
- **Specification adherence**: High for core algorithms, lower for architecture. The audit lifecycle is the most spec-faithful of all three. But the flat package structure contradicts the spec's implied separation of concerns.
- **Code style**: Dense and functional. Zero external dependencies shows a preference for self-sufficiency over convenience. The code is correct but harder to navigate due to the flat structure. Naming is clear within files but lacks the organizational clarity that package boundaries provide.
- **Testing discipline**: Strong where it matters. 94 tests focused on critical paths — state transitions, navigation, decomposition, audit lifecycle, doctor repairs. Tests are integration-style (full App lifecycle) rather than unit-style, which catches more real bugs but is harder to debug when failures occur.
- **Error handling**: Good. Consistent error returns with context. The `syncAuditLifecycle` function properly handles all edge cases including gap-blocks-completion, showing careful defensive thinking in the most critical code paths.
