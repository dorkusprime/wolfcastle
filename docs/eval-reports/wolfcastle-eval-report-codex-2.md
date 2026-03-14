# Wolfcastle Implementation Evaluation Report

## Build Status

| Implementation | `go build ./...` | `go vet ./...` | `go test ./...` | Notes |
|----------------|:-----------------:|:--------------:|:---------------:|-------|
| Claude | Pass | Pass | 8 pass, 0 fail | Builds and tests cleanly once `cobra` is available. |
| Gemini | Pass | Pass | 5 pass, 0 fail | Builds and tests cleanly once `cobra` is available. |
| Codex | Pass | Pass | 1 pass, 0 fail | Single-package test suite is large and passes end to end. |

## Per-Implementation Scores

### Implementation A: Claude

| Dimension | Weight | Score (1-5) | Notes |
|-----------|:------:|:-----------:|-------|
| Spec & ADR Compliance | 20% | 4 | Strong coverage of the spec surface: deep merge with null deletion in `internal/config/merge.go:3-31`, prompt assembly ordering in `internal/pipeline/prompt.go:10-57`, and a 17-category validator in `internal/validate/engine.go:41-264`. Main compliance miss is that the daemon mutates leaf state without propagating parent/root state in the same operation (`internal/daemon/daemon.go:246-259`, `328-390`), despite the CLI scripts doing the right thing via `cmd/helpers.go:10-86`. |
| Code Quality | 25% | 4 | Clean package boundaries (`cmd/`, `internal/state`, `internal/validate`, `internal/pipeline`), contextual errors, and meaningful tests around state, config, pipeline, tree, and validation. Daemon signal handling is serviceable but not rigorous: `Run` closes an internal shutdown channel on SIGINT/SIGTERM without cancelling the active model invocation context (`internal/daemon/daemon.go:121-127`, `298-315`). |
| Completeness | 25% | 4 | Broadest command surface of the three, including audit scope/show/gap/fix-gap/resolve, inbox, archive, follow, doctor, ADR/spec commands, and install. Validation, prompt assembly, tree addressing, and audit structures are all present. Biggest gaps are in daemon correctness rather than missing files. |
| Algorithm Correctness | 30% | 3 | `RecomputeState` matches the parent propagation truth table (`internal/state/propagation.go:5-49`) and navigation properly prefers stale `in_progress` tasks before `not_started` (`internal/state/navigation.go:82-111`). The main failure is daemon-side state mutation: the daemon claims and mutates leaf tasks without calling `propagateState`, so ancestor `state.json` files and the root index can drift during normal execution (`internal/daemon/daemon.go:234-395`). `STALE_IN_PROGRESS` also warns unconditionally when exactly one task is active instead of checking PID state (`internal/validate/engine.go:253-259`). |
| **Weighted Total** | | **3.70** | |

#### Strengths
- Best overall spec coverage and CLI breadth; the command surface is close to ADR-021.
- Strong validation-engine structure with clean deterministic/model-assisted fix typing in `internal/validate/engine.go:41-264`.
- Correct config merge semantics and prompt assembly behavior.

#### Weaknesses
- Daemon path violates ADR-024 propagation expectations by updating only the leaf and root entry in several paths.
- Signal handling is weaker than the spec requires for process-group-aware graceful shutdown.
- Stale in-progress detection is oversimplified and can report normal runtime as stale.

#### Critical Deviations from Spec
- Daemon mutations do not reliably write child, parent, and root index together in one deterministic path: `internal/daemon/daemon.go:246-259`, `328-390`.
- Startup/self-healing validation does not actually test whether a daemon PID is live before reporting `STALE_IN_PROGRESS`: `internal/validate/engine.go:253-259`.

#### Wrong vs Missing
Attempted but wrong:
- Daemon propagation during execution is implemented but incomplete/wrong.
- `STALE_IN_PROGRESS` detection is implemented but does not follow the PID-based rule.

Cleanly absent or stubbed:
- Little is cleanly stubbed; most major subsystems are attempted.

### Implementation B: Gemini

| Dimension | Weight | Score (1-5) | Notes |
|-----------|:------:|:-----------:|-------|
| Spec & ADR Compliance | 20% | 2 | It has many required files and commands, but several core state rules are wrong. `BlockTask` allows blocking any non-complete task, including `not_started` tasks, and forces the whole leaf blocked immediately (`internal/state/mutations.go:61-76`), which contradicts the state-machine spec. The codebase audit command is explicitly unfinished (`internal/cli/audit.go:261-267`). Config loading merges JSON correctly but does not validate prompt existence, model references, or stage uniqueness before use (`internal/config/config.go:169-205`). |
| Code Quality | 25% | 3 | Package layout is sensible and tests exist for archive/config/doctor/state/utils. Error handling is generally okay. The problem is behavioral rigor, not style: several subsystems are optimistic and silently wrong, especially task-state handling and doctor checks. |
| Completeness | 25% | 3 | Core CLI commands, daemon, state I/O, archive, and doctor all exist. The validation engine is present, but the codebase audit flow is incomplete, and several “implemented” features are only shallowly done. Prompt assembly and stage invocation exist, but overall boundary quality is mixed because wrong implementations are wired into the main path. |
| Algorithm Correctness | 30% | 2 | Parent recomputation itself is correct (`internal/state/propagation.go:24-58`) and navigation is mostly correct (`internal/state/navigation.go:46-85`). But task/blocking logic is materially wrong: `BlockTask` permits invalid transitions and collapses the leaf to `blocked` too aggressively (`internal/state/mutations.go:61-76`), and `UnblockTask` marks a leaf blocked whenever *any* task is blocked rather than only when all non-complete tasks are blocked (`internal/state/mutations.go:101-125`). Doctor propagation checking only validates orchestrators already marked `complete`, missing the general `recompute_parent()` invariant (`internal/doctor/doctor.go:172-186`). |
| **Weighted Total** | | **2.50** | |

#### Strengths
- Good package structure and readable state/daemon separation.
- Correct deep-merge semantics, including null deletion, in `internal/config/config.go:148-167`.
- Pipeline invocation uses `exec.CommandContext` with `Setpgid` as required in `internal/pipeline/pipeline.go:245-301`.

#### Weaknesses
- State-machine correctness is the weakest of the three.
- Doctor checks are broader than a toy implementation but still miss core invariants.
- Config resolution is not paired with the validation discipline the spec requires.

#### Critical Deviations from Spec
- Invalid task transition handling and incorrect blocked-state derivation in `internal/state/mutations.go:61-76`, `101-125`.
- Codebase audit flow is admitted incomplete in `internal/cli/audit.go:261-267`.
- Doctor does not recompute orchestrator state generally; it only checks one narrow case in `internal/doctor/doctor.go:172-186`.

#### Wrong vs Missing
Attempted but wrong:
- Task block/unblock logic.
- Doctor propagation validation.
- Orphan-state detection logic is not actually checking parent `children` membership correctly (`internal/doctor/doctor.go:162-169`).

Cleanly absent or stubbed:
- Interactive codebase audit approval flow is explicitly not implemented.
- Config validation beyond merge/load is mostly absent.

### Implementation C: Codex

| Dimension | Weight | Score (1-5) | Notes |
|-----------|:------:|:-----------:|-------|
| Spec & ADR Compliance | 20% | 3 | Strong on daemon lifecycle, audit mechanics, JSON-only state/config, and distributed state files. The main compliance gaps are config/schema fidelity and defaults: `readResolvedConfig` only merges files and performs no schema/reference validation (`config.go:11-34`), and the default pipeline omits the spec’s `expand` and `file` stages (`defaults.go:21-26`). There are also uninstructed doctor categories beyond the 17 spec categories, which is neutral except where they replace spec behavior. |
| Code Quality | 25% | 4 | Best-tested implementation by far; the single-package test suite exercises state, daemon, doctor, archive, audit, and help flows. Daemon/process-group handling is robust (`daemon.go:21-135`, `447-462`), and the runtime/pipeline code is cohesive despite being flatter than the other two repos. The main maintainability drawback is the very large single package. |
| Completeness | 25% | 4 | Broad command surface including audit pending/approve/reject/history, unblock, follow, install, archive, spec, ADR, inbox, and daemon controls. Prompt assembly, stage skipping, archive generation, audit propagation, and doctor fixes are all implemented. The main incompleteness is config validation and spec-accurate default pipeline configuration. |
| Algorithm Correctness | 30% | 4 | Core state algorithms are solid: leaf and orchestrator recomputation both match the spec truth tables (`state_tree.go:174-211`, `297-330`), navigation prioritizes `in_progress` before `not_started` (`state_tree.go:332-362`), and the pipeline invocation path honors stage order, `enabled`, `skip_prompt_assembly`, retries, and summary skipping (`runtime_stage.go:72-227`, `265-340`). The biggest algorithmic miss is doctor’s depth check, which enforces `child = parent + 1` instead of the spec’s `child >= parent` (`doctor.go:465-473`). |
| **Weighted Total** | | **3.85** | |

#### Strengths
- Strongest daemon/process management and end-to-end test discipline.
- Correct distributed-state propagation and navigation logic in the main runtime path.
- Most complete audit subsystem, including result/show/review/history behaviors.

#### Weaknesses
- Config loading is under-validated relative to the spec.
- Default pipeline and default models are pragmatic bootstraps rather than spec-faithful defaults.
- Flat package structure is workable now but could get harder to extend than Claude’s split internal packages.

#### Critical Deviations from Spec
- Config loader does not enforce model-reference resolution, prompt existence, unique stage names, or identity-local-only rules at load time (`config.go:11-34`).
- Default pipeline omits `expand` and `file` stages (`defaults.go:21-26`).
- Doctor depth invariant is wrong: it forces `parent + 1` rather than `>= parent` (`doctor.go:465-473`).

#### Wrong vs Missing
Attempted but wrong:
- Doctor depth validation/fix semantics.
- Config handling is implemented but under-validated versus the schema/ADR contract.

Cleanly absent or stubbed:
- No dedicated schema-validation layer for config.
- Defaults do not fully scaffold the canonical four-stage pipeline, though runtime support for those stages exists.

## Comparative Analysis

### Head-to-Head Summary

| Dimension | Weight | Claude | Gemini | Codex |
|-----------|:------:|:------:|:------:|:-----:|
| Spec & ADR Compliance | 20% | 4 | 2 | 3 |
| Code Quality | 25% | 4 | 3 | 4 |
| Completeness | 25% | 4 | 3 | 4 |
| Algorithm Correctness | 30% | 3 | 2 | 4 |
| **Weighted Total** | | **3.70** | **2.50** | **3.85** |

### Notable Differences
- Claude optimized for breadth and architectural separation. It built a large, credible product surface first, then filled in substantial internals behind it.
- Gemini implemented the right nouns but was looser on invariants. It often has code for a feature, but the behavior is not constrained tightly enough by the spec.
- Codex focused on executable end-to-end behavior and verification. It is less schema-faithful than Claude, but its daemon/runtime path is the most defensible under test.

### Common Mistakes
- None of the three fully nailed config validation to the level the spec asks for.
- All three are somewhat looser than the spec on doctor fidelity. Claude is closest; Gemini is materially incomplete; Codex adds non-spec checks and misses the exact depth rule.
- All three have at least one place where the distributed-state invariants are weaker in runtime paths than they look on paper.

### Ranking
1. **Codex** — Best critical-path correctness and daemon/runtime discipline, with the strongest test evidence, despite weaker config/schema conformance.
2. **Claude** — Broadest and most spec-shaped implementation, but it loses the top spot because the daemon path can leave distributed state inconsistent during normal execution.
3. **Gemini** — Respectable structure and coverage, but too many core state-machine behaviors are attempted incorrectly rather than left cleanly unimplemented.

## Overall Interpretation

### Implementation Philosophies
Claude approached Wolfcastle like a product codebase. It has the cleanest subsystem boundaries and the most obviously deliberate package structure. The implementation personality is “build the whole organism,” which worked well for coverage, but it also meant one of the most important invariants, state propagation during daemon execution, slipped through even though the supporting library code exists.

Gemini approached the problem as a reasonably broad scaffold with enough internals to function. The personality is “wire up the whole flow, then make it plausible.” That produced a repo that looks complete at first glance, but closer reading shows too many places where state transitions are not defended tightly enough.

Codex approached Wolfcastle as an executable workflow first. The personality is “make the daemon and scripts actually behave, then harden them with tests.” That bias shows up in the strongest runtime and test story, but also in weaker up-front schema fidelity and a flatter package structure.

### Strengths Worth Adopting
- Claude’s validation engine structure in `wolfcastle-claude/internal/validate/engine.go:41-264` is the best extensibility pattern. It is the clearest foundation for adding or refining issue categories.
- Gemini’s config merge implementation in `wolfcastle-gemini/internal/config/config.go:148-167` is concise and correct; both other repos could borrow that exact style if they wanted a simpler merge core.
- Codex’s daemon runtime in `wolfcastle-codex/daemon.go:21-135`, `265-340` is the strongest operational skeleton. If I were assembling a production version, I would start from that loop and process-management model.

### Weaknesses and Pitfalls
- Claude’s biggest risk is false confidence: the library-level propagation model is good, but the daemon bypasses it. That kind of split-brain correctness is dangerous because the codebase looks more correct than it is.
- Gemini’s problem is landmines in core invariants. Wrong state-machine behavior is materially worse than missing features here, and there are too many examples of that.
- Codex’s problem is spec drift. The runtime is strong, but parts of config/defaults/doctor feel driven by implementation convenience rather than strict adherence to the written contract.

### Cross-Pollination Opportunities
- Skeleton: use Codex’s daemon/runtime loop and process-group stop behavior.
- Validation architecture: use Claude’s `internal/validate` package design and fix typing.
- CLI/package organization: use Claude’s package boundaries, not Codex’s flat package.
- Test suite discipline: use Codex’s end-to-end tests as the quality bar.
- Config merge core: Gemini’s deep-merge implementation is the simplest correct version.

### Spec Feedback
- The spec should be more explicit that runtime daemon paths must use the same propagation discipline as manual CLI scripts. Claude demonstrates how easy it is to satisfy the script layer but still violate the invariant in the daemon.
- The distinction between audit-task status and audit-object status is subtle; Codex handled it best, and the others show that the spec likely needs even more examples around audit lifecycle transitions.
- The doctor spec would benefit from an explicit machine-readable checklist of the 17 categories and exact fix-policy mapping. Claude came closest, but divergence across all three suggests the prose leaves room for interpretation.

### Model Tendencies
- **Claude**: breadth-first, architecture-conscious, good abstractions, solid testing, but can miss a critical integration seam between otherwise-correct subsystems.
- **Gemini**: broad scaffold, acceptable code style, but more likely to implement a rule approximately rather than exactly.
- **Codex**: depth-first on operational correctness, strongest tests, pragmatic rather than doctrinaire, but more willing to drift from spec details when a simpler executable design presents itself.
