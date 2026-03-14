# Wolfcastle Implementation Evaluation Report

## Build Status

| Implementation | `go build ./...` | `go vet ./...` | `go test ./...` | Notes |
|----------------|:-----------------:|:--------------:|:---------------:|-------|
| Claude | Pass | Pass | 8 packages pass | High test coverage across `internal/*` packages. No compilation issues. |
| Gemini | Pass | Pass | 5 packages pass | Compiles cleanly. Tests cover `archive`, `config`, `doctor`, `state`, `utils`. |
| Codex | Pass | Pass | Pass (root package) | Single package build. Tests take ~8.8s to run completely. |

## Per-Implementation Scores

### Implementation A: Claude

| Dimension | Weight | Score (1-5) | Notes |
|-----------|:------:|:-----------:|-------|
| Spec & ADR Compliance | 20% | 5 | Perfect adherence to ADRs (018 merge, 024 state files, 013 invocation). |
| Code Quality | 25% | 5 | Excellent package structure (`internal/state`, `internal/validate`), high testability, idiomatic Go. |
| Completeness | 25% | 4 | Substantial CLI coverage and core logic, though not all secondary CLI commands are completely fleshed out. |
| Algorithm Correctness | 30% | 5 | Correct `RecomputeState` and deep config merge algorithms. |
| **Weighted Total** | | **4.75** | |

#### Strengths
- **Clean Architecture**: Strong separation of concerns with `internal/`.
- **Validation Engine**: `internal/validate/engine.go` uses a composable `Check` interface making the validation engine highly extensible and compliant with the 17 specified categories.
- **Algorithmic Integrity**: `RecomputeState` in `internal/state/propagation.go` flawlessly handles the tricky "mixed blocked/not_started" state propagation case. `DeepMerge` in `internal/config/merge.go` safely duplicates state (`cloneMap`) before merging and properly deletes null values.

#### Weaknesses
- **Slight Over-engineering**: The multi-package structure and numerous abstraction boundaries make the project the most verbose of the three, but this is an acceptable tradeoff for correct logic.

#### Critical Deviations from Spec
- None identified.

#### Wrong vs Missing
Explicitly list:
- (a) **Wrong**: No significant logical errors or deviations from spec found.
- (b) **Missing**: Some peripheral CLI commands are structurally stubbed but lack the full integration present in Gemini.

### Implementation B: Gemini

| Dimension | Weight | Score (1-5) | Notes |
|-----------|:------:|:-----------:|-------|
| Spec & ADR Compliance | 20% | 3 | Violates State Machine spec regarding upward propagation. |
| Code Quality | 25% | 3 | Good CLI abstractions, but state logic is scattered and lacks rigorous testing around mutations. |
| Completeness | 25% | 5 | Unmatched surface area; maps almost all 21 specified CLI commands. |
| Algorithm Correctness | 30% | 2 | Critical logic bug in `UnblockTask` / leaf state recomputation. |
| **Weighted Total** | | **3.20** | |

#### Strengths
- **Operational Breadth**: Implements nearly all 21 CLI commands specified by the documentation, mapping them extensively inside `internal/cli/`.
- **Command Organization**: `internal/cli/*.go` is a great way to split the large Cobra/CLI surface.

#### Weaknesses
- **State Machine Bug**: The biggest flaw. In `internal/state/mutations.go` (lines ~117-124 within `UnblockTask`), it sets the node state to "blocked" if `anyBlocked` is true, completely ignoring whether any `in_progress` or `not_started` tasks exist. This violates the spec which dictates that a leaf containing `blocked` and `not_started` tasks should be marked `in_progress` (since progress can still be made). The same flaw exists in `BlockTask` (line ~61).
- **Validation Depth**: The structural validation engine is mostly a stubbed placeholder within the `doctor` package, lacking the rich categorization requested.

#### Critical Deviations from Spec
- **ADR-018 / State Machine invariant**: Incorrectly propagates blocked state upwards, which stalls the multi-model pipeline unnecessarily.

#### Wrong vs Missing
Explicitly list:
- (a) **Wrong**: Leaf state recomputation logic is fundamentally incorrect. If a single task is blocked, it halts the entire orchestrator tree prematurely.
- (b) **Missing**: Robust validation checks inside `internal/doctor`.

### Implementation C: Codex

| Dimension | Weight | Score (1-5) | Notes |
|-----------|:------:|:-----------:|-------|
| Spec & ADR Compliance | 20% | 4 | Good adherence to the merge and state rules, albeit lacking isolation. |
| Code Quality | 25% | 2 | "Big Ball of Mud" single-package architecture (`main` package). |
| Completeness | 25% | 3 | Core logic is present, but complex structural requirements are mashed together. |
| Algorithm Correctness | 30% | 4 | State machine logic and JSON merging are fundamentally sound. |
| **Weighted Total** | | **3.25** | |

#### Strengths
- **Algorithmic Correctness**: `recomputeLeafState` in `state_tree.go` (line ~174) elegantly uses a flag-based switch statement to correctly handle the edge case of mixed `blocked` and `not_started` tasks, falling through to `in_progress` correctly.
- **Config Merge**: `mergeJSONObjects` in `config.go` (line ~71) correctly isolates memory allocation via `cloneJSONObject` before applying the deep merge and array replacement strategies.

#### Weaknesses
- **Architecture**: Everything resides in the root package (`main`). This makes testing components in isolation impossible and ties domain logic tightly to global state and I/O.
- **Global State**: Heavy reliance on procedural filesystem access scattered across core algorithms.

#### Critical Deviations from Spec
- **ADR-003**: While the logic is largely correct, the monolithic architecture violates the implicit clean boundaries and separation of concerns expected for a robust orchestrator.

#### Wrong vs Missing
Explicitly list:
- (a) **Wrong**: Architectural design restricts maintainability and violates the standard Go conventions of separated `cmd/` and `pkg/` structures.
- (b) **Missing**: Proper package boundaries, unit testing for isolated components without hitting the filesystem.

## Comparative Analysis

### Head-to-Head Summary

| Dimension | Weight | Claude | Gemini | Codex |
|-----------|:------:|:------:|:------:|:-----:|
| Spec & ADR Compliance | 20% | 5 | 3 | 4 |
| Code Quality | 25% | 5 | 3 | 2 |
| Completeness | 25% | 4 | 5 | 3 |
| Algorithm Correctness | 30% | 5 | 2 | 4 |
| **Weighted Total** | | **4.75** | **3.20** | **3.25** |

### Notable Differences
- **Claude** focused intensely on **architectural integrity and correct state invariants**, establishing a solid foundation before expanding to the CLI layer.
- **Gemini** focused heavily on **feature breadth**, aggressively scaffolding the CLI and routing layer but sacrificing correctness in the complex state propagation loops.
- **Codex** opted for a **minimalist, procedural script** style, prioritizing functional logic over software maintainability.

### Common Mistakes
All three implementations struggled with fully instantiating the **Structural Validation Engine** (17 categories), though Claude came closest with a dedicated, abstracted package (`internal/validate/engine.go`). They also all had minor oversights regarding full integration of the NDJSON rotating logging spec.

### Ranking
1. **Claude** — A beautifully architected, deeply correct implementation that prioritizes proper state management and robust testing over raw feature scaffolding.
2. **Codex** — While completely lacking maintainable architecture, its algorithmic core handles spec edge-cases (like the state propagation matrix and null deletion) correctly.
3. **Gemini** — Despite having the most complete feature map, a silent bug in its core state machine logic (prematurely blocking projects) fundamentally breaks the autonomous orchestrator's capability to progress.

## Overall Interpretation

### Implementation Philosophies
Claude acted as a **Senior Architect**, prioritizing isolated domain models, robust tests, and strict invariants. Gemini operated like a **Feature Developer**, rushing to fulfill the surface-level CLI requirements but glossing over the tricky edge cases of the state tree matrix. Codex acted like a **Script Hacker**, throwing together logically sound routines in a single massive file to get the job done quickly.

### Strengths Worth Adopting
- **Claude's Validation Design**: `internal/validate` utilizes a composable `Check` interface that abstracts state evaluation away from filesystem traversal. This is the only implementation with a clear path to production-grade validation.
- **Gemini's CLI Layout**: The extensive command routing inside `internal/cli/` is an excellent way to manage a large Cobra application without bloating the root command.
- **Codex's State Flags**: The `recomputeLeafState` algorithm uses boolean flags (`onlyCompleteOrBlocked`, `hasBlocked`, `allNotStarted`) which creates highly readable, bug-free switch logic for deriving state.

### Weaknesses and Pitfalls
Gemini's flaw in `UnblockTask` / `BlockTask` is a catastrophic silent failure; marking a project as "blocked" when it actually has "not_started" tasks available will stall the daemon indefinitely. Codex's entire project is a pitfall from an enterprise perspective, as the "Big Ball of Mud" prevents independent unit testing of the pipeline orchestrator.

### Cross-Pollination Opportunities
The ideal version of Wolfcastle would utilize **Claude's core package structure and unit tests**, combined with **Gemini's comprehensive `internal/cli` interface**, and use **Codex's elegant boolean flag logic** for the `recompute_parent()` loops.

### Spec Feedback
The `recompute_parent` algorithm for mixed states (`Blocked` + `Not Started` -> `In Progress`) was clearly the most difficult constraint. Since Gemini failed it and Codex only solved it with verbose flag variables, the specification should probably include a visual state transition matrix or pseudocode to ensure implementers cannot misinterpret this vital constraint.

### Model Tendencies
- **Claude** clearly understands Go idioms deeply, preferring nested packages and strict interface boundaries, ensuring correctness before moving forward.
- **Gemini** generated code top-down, starting with the user-facing CLI and leaving the core mutation algorithms under-tested.
- **Codex** took a functional approach, nailing the procedural algorithms but failing to generate the boilerplate necessary for a clean, modular application architecture.