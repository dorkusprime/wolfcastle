# Wolfcastle Implementation Evaluation Report

## Build Status

| Implementation | `go build ./...` | `go vet ./...` | `go test ./...` | Notes |
|----------------|:-----------------:|:--------------:|:---------------:|-------|
| Claude | Pass | Pass | 7 pass, 0 fail | Clean gate check. Build emitted a sandbox cache warning unrelated to repo code. |
| Gemini | Pass | Pass | 1 pass, 0 fail | Clean gate check. Build emitted a sandbox cache warning unrelated to repo code. |
| Codex | Pass | Pass | 1 pass, 0 fail | Clean gate check after rerunning with workspace-local `GOCACHE`/`GOMODCACHE`. |

## Per-Implementation Scores

### Claude

| Dimension | Weight | Score (1-5) | Notes |
|-----------|:------:|:-----------:|-------|
| Spec & ADR Compliance | 20% | 3 | Strong on core state semantics, but important schema and daemon-contract deviations remain. |
| Code Quality | 25% | 4 | Best package structure and test coverage of the three. |
| Completeness | 25% | 4 | Broadest implemented CLI surface and good subsystem spread. |
| Algorithm Correctness | 30% | 3 | Core propagation/navigation/config merge are good, but prompt assembly and daemon state handling are incomplete. |
| **Weighted Total** | | **3.50** | |

#### Strengths
- `internal/state/propagation.go:5-49` implements the spec’s `recompute_parent()` logic correctly, including the critical mixed `blocked + not_started -> in_progress` case from the state-machine spec (`wolfcastle/docs/specs/2026-03-12T00-00Z-state-machine.md:142-186`).
- `internal/state/navigation.go:12-106` gets the depth-first traversal and stale `in_progress` resume ordering right.
- `internal/config/merge.go:3-31` correctly deep-merges objects, replaces arrays wholesale, and honors null deletion, matching ADR-018.
- The codebase has the strongest package separation: `cmd`, `internal/state`, `internal/pipeline`, `internal/invoke`, `internal/logging`, `internal/archive`, `internal/project`.

#### Weaknesses
- The root index and task schema do not match the spec: `RootIndex` uses `root_id/root_name/root_state` instead of the spec’s root registry shape, and `Task` lacks `is_audit` and uses `block_reason` instead of `blocked_reason` (`internal/state/types.go:33-85` vs `wolfcastle/docs/specs/2026-03-12T00-00Z-state-machine.md:163-171` and `wolfcastle/docs/specs/2026-03-13T00-00Z-structural-validation.md:173-181`).
- `pipeline.AssemblePrompt()` drops iteration context entirely when `skip_prompt_assembly` is set (`internal/pipeline/prompt.go:13-19`), but the stage-contract spec requires iteration context even for skipped assembly (`wolfcastle/docs/specs/2026-03-12T00-03Z-pipeline-stage-contract.md:134-170`).
- `cmd/start.go:58-75` does stale PID handling, but it does not run startup doctor checks or explicit self-healing before entering the daemon loop as required by the CLI and ADR-020 specs.
- `internal/project/scaffold.go:51-60` writes an empty `config.json`, not a default shared config as required by the CLI spec (`wolfcastle/docs/specs/2026-03-12T00-06Z-cli-commands.md:78-87`).

#### Critical Deviations from Spec
- Schema mismatch in distributed state files: `internal/state/types.go:33-85`.
- `skip_prompt_assembly` omits iteration context: `internal/pipeline/prompt.go:13-19`.
- `doctor` implements only a subset of the 17 required validation categories and does not expose the composable validation engine described in the structural-validation spec: `cmd/doctor.go:23-380`.

#### Wrong vs Missing
- Wrong:
  - State/task JSON field names and shapes are inconsistent with the spec (`internal/state/types.go:33-85`).
  - `skip_prompt_assembly` behavior is incorrect (`internal/pipeline/prompt.go:13-19`).
  - `init` writes an empty shared config instead of a populated default config (`internal/project/scaffold.go:51-60`).
- Missing or stubbed:
  - Full structural validation engine with all 17 issue categories, startup subset, rollback semantics, and model-guardrailed fixes.
  - A spec-complete startup validation/self-healing sequence in `start`.

### Gemini

| Dimension | Weight | Score (1-5) | Notes |
|-----------|:------:|:-----------:|-------|
| Spec & ADR Compliance | 20% | 2 | Broad attempt, but several core schema and command-contract choices contradict the spec. |
| Code Quality | 25% | 3 | Reasonable structure, but sparse testing and some shaky implementation details. |
| Completeness | 25% | 3 | Many major subsystems exist, but several are only partial or diverge from the intended boundaries. |
| Algorithm Correctness | 30% | 2 | Multiple critical algorithms are partially right but break spec invariants in important cases. |
| **Weighted Total** | | **2.45** | |

#### Strengths
- `internal/doctor/doctor.go:15-339` is the closest implementation to the spec’s 17 validation categories and explicitly models deterministic vs model-assisted vs user-guided fixes.
- `internal/pipeline/pipeline.go:51-68` preserves the intended prompt assembly order for normal stages.
- `internal/state/state.go:275-344` gets the orchestrator propagation formula broadly right.

#### Weaknesses
- The root index schema is incompatible with the spec: `RootState` uses `root_id/root_name/root_state` rather than the required root registry shape (`internal/state/state.go:20-26`).
- Navigation invents orchestrator audit tasks on the fly (`internal/state/state.go:250-266`), contradicting the state-machine spec that orchestrators contain children and leaves contain ordered task lists (`wolfcastle/docs/specs/2026-03-12T00-00Z-state-machine.md:11-18`).
- The task CLI requires both `--node` and a positional `<task-id>` (`internal/cli/task.go:141-260`), diverging from the command spec where `--node` itself is the task tree address (`wolfcastle/docs/specs/2026-03-12T00-06Z-cli-commands.md:9-16` and task command sections).
- The pipeline reads script reference from `base/scripts/script-reference.md` (`internal/pipeline/pipeline.go:112-118`) even though the prompt contract specifies `base/prompts/script-reference.md` (`wolfcastle/docs/specs/2026-03-12T00-03Z-pipeline-stage-contract.md:153-159`).

#### Critical Deviations from Spec
- Orchestrator audit-task fabrication in navigation: `internal/state/state.go:250-266`, and it is explicitly tested as intended behavior in `internal/state/state_test.go:215-237`.
- Depth-first self-healing is wrong for leaves with both `not_started` and `in_progress` tasks, because `FindNextTask()` returns the first task in either state order rather than preferring stale `in_progress` globally (`internal/state/state.go:224-227`).
- Config loading supports an extra `base/custom/local` config-file stack (`internal/config/config.go:153-172`) instead of the spec’s `config.json` + `config.local.json` pair (`wolfcastle/docs/specs/2026-03-12T00-01Z-config-schema.md:5-22`).

#### Wrong vs Missing
- Wrong:
  - Root/index schema shape (`internal/state/state.go:20-26`).
  - Orchestrator task model in navigation (`internal/state/state.go:250-266`).
  - Task CLI address contract (`internal/cli/task.go:141-260`).
  - Script-reference path in prompt assembly (`internal/pipeline/pipeline.go:112-118`).
- Missing or stubbed:
  - Strong test coverage outside `internal/state/state_test.go`.
  - A clean, spec-conformant task-address model across commands.
  - File-atomic repair/rollback semantics in doctor.

### Codex

| Dimension | Weight | Score (1-5) | Notes |
|-----------|:------:|:-----------:|-------|
| Spec & ADR Compliance | 20% | 3 | Broadest practical implementation, but several contract/schema gaps remain. |
| Code Quality | 25% | 4 | Strong test discipline and pragmatic end-to-end coverage, though the single-package layout is less clean. |
| Completeness | 25% | 4 | Most complete command surface and the strongest doctor/audit/archive coverage. |
| Algorithm Correctness | 30% | 2 | Two central state/navigation algorithms are wrong in ways that materially affect execution. |
| **Weighted Total** | | **3.10** | |

#### Strengths
- This is the most complete implementation overall: command surface in `app.go:43-82` and `help.go:5-245`, extensive doctor coverage in `doctor.go`, archive handling, audit workflow, decomposition, and strong end-to-end tests in `main_test.go`.
- The config merge implementation is correct and well tested: `config.go:11-35`, `config.go:71-93`, and tests in `main_test.go:159-196`.
- Failure escalation behavior is closer to spec than the other two: `runtime_mutation.go:153-193` handles threshold, max-depth, and hard-cap blocking.
- Prompt assembly honors rule fragments, script reference, stage prompt, and iteration context for normal stages, and preserves iteration context when `skip_prompt_assembly` is false: `runtime_stage.go:78-130`.

#### Weaknesses
- `recomputeLeafState()` is wrong: any blocked task makes the leaf blocked (`state_tree.go:174-208`), but the spec says a leaf is blocked only when progress truly cannot continue; this also conflicts with the orchestrator propagation model and unblock workflow.
- Navigation does not prioritize stale `in_progress` work; it simply returns the first non-complete task in leaf order (`state_tree.go:329-356`), violating the self-healing requirement.
- The resolved config omits required top-level sections like `docs`, `prompts`, and `git` (`types.go:3-17`), so it does not satisfy the full config schema (`wolfcastle/docs/specs/2026-03-12T00-01Z-config-schema.md:26-45`).
- Model invocation does not put children in their own process group (`runtime_stage.go:149-157`), and `stop --force` kills only the daemon PID, not the child process group (`daemon.go:404-432`), which misses ADR-020 and ADR-013 expectations.
- JSON writes are not atomic (`app.go:1052-1058`), so doctor and state updates do not meet the “single write / rollback-safe” expectation from the structural-validation spec.

#### Critical Deviations from Spec
- Leaf-state derivation is inverted for blocked-task cases: `state_tree.go:174-208`.
- Depth-first self-healing order is wrong: `state_tree.go:337-343`.
- The config surface is incomplete relative to the schema: `types.go:3-17`.
- Error handling does not consistently emit the spec’s JSON error envelope; `main.go:8-14` prints plain stderr text and exits `1`.

#### Wrong vs Missing
- Wrong:
  - Leaf state recomputation on blocked tasks (`state_tree.go:174-208`).
  - Navigation/self-healing ordering (`state_tree.go:329-356`).
  - Daemon process-group ownership and forced-stop semantics (`runtime_stage.go:149-157`, `daemon.go:404-432`).
- Missing or stubbed:
  - Full config-schema coverage for `docs`, `prompts`, `git`, and related validation (`types.go:3-17`).
  - Atomic multi-file repair with rollback.
  - Standardized JSON error envelopes on every failure path.

## Comparative Analysis

### Head-to-Head Summary

| Dimension | Weight | Claude | Gemini | Codex |
|-----------|:------:|:------:|:------:|:-----:|
| Spec & ADR Compliance | 20% | 3 | 2 | 3 |
| Code Quality | 25% | 4 | 3 | 4 |
| Completeness | 25% | 4 | 3 | 4 |
| Algorithm Correctness | 30% | 3 | 2 | 2 |
| **Weighted Total** | | **3.50** | **2.45** | **3.10** |

### Notable Differences
- Claude optimized for internal structure and core state algorithms first. Its best work is in `internal/state`, `internal/config`, and `internal/pipeline`, but it leaves more spec-edge infrastructure unfinished.
- Gemini spread effort across almost every subsystem, including doctor, pipeline, daemon, and CLI, but it improvised more aggressively, especially around orchestrator audit behavior and CLI task addressing.
- Codex pushed furthest toward an integrated product: broad command surface, doctor repair logic, archive flow, decomposition, and end-to-end tests. The tradeoff is that some foundational state-machine logic is simply wrong.

### Common Mistakes
- All three diverge from the exact distributed-state schema in some way. None is a clean match for the root-index and per-node JSON shape mandated by the state-machine and structural-validation specs.
- All three underdeliver on the validation engine’s full contract. Codex and Gemini are much closer than Claude, but none cleanly implements the spec’s atomic-fix/rollback model.
- All three hardcode model defaults around Claude-family commands, which is awkward against ADR-004’s model-agnostic intent even when the runtime paths remain configurable.

### Ranking
1. **Claude** — Best balance of correct core algorithms, package quality, and implementation discipline, even though some important infrastructure is still incomplete.
2. **Codex** — Broadest and most tested implementation, but the leaf-state and navigation bugs land directly in the heaviest-weight algorithm bucket.
3. **Gemini** — The most ambitious on doctor breadth, but too many contract-level deviations undermine the core design.

## Overall Interpretation

### Implementation Philosophies
- Claude approached Wolfcastle as a maintainable Go application first. The package graph is deliberate, tests target core subsystems, and the best work is in deterministic logic. The downside is that some required surface area remains partial, and a few spec edges were left for later.
- Gemini approached Wolfcastle as a full-feature scaffold. It tried to stand up nearly every major subsystem quickly, including a notably broad doctor implementation. That breadth came with more improvisation: when the spec left friction, Gemini sometimes changed the model instead of preserving the contract.
- Codex approached Wolfcastle like a pragmatic product build. It stitched together the highest number of visible features, backed them with end-to-end tests, and implemented a large amount of operator-facing workflow. The main failure mode is classic systems risk: the code around the edges is rich, but the core state-machine invariants are not consistently protected.

### Strengths Worth Adopting
- Claude’s `internal/state/propagation.go:5-49` and `internal/state/navigation.go:12-106` are the strongest reference implementations for the state-machine core.
- Claude’s `internal/config/merge.go:3-31` is the cleanest merge implementation.
- Gemini’s `internal/doctor/doctor.go:15-339` is the best starting point for a spec-complete validation taxonomy.
- Codex’s end-to-end tests in `main_test.go` are the most valuable practical asset in the repo set; they exercise archive generation, doctor fixes, daemon lifecycle, failure escalation, and audit propagation.
- Codex’s audit/archive workflow is the most fleshed out operationally, especially in `auditcmd.go`, `archive.go`, and `doctor.go`.

### Weaknesses and Pitfalls
- Claude’s biggest long-term risk is false confidence from clean structure hiding spec mismatches in schemas and daemon lifecycle. Those are fixable, but they sit at integration boundaries.
- Gemini’s biggest risk is architectural drift: orchestrator audit tasks, alternate config layering, and task-ID CLI semantics would force painful compatibility fixes later.
- Codex’s biggest risk is semantic corruption. A wrong `recomputeLeafState()` and wrong navigation order will silently produce bad execution behavior even though the rest of the system appears feature-rich.

### Cross-Pollination Opportunities
- Skeleton: Claude’s package structure.
- State machine and navigation: Claude’s `internal/state`.
- Validation taxonomy: Gemini’s doctor issue model and category coverage.
- Product workflow and tests: Codex’s audit/archive/decomposition flows plus its end-to-end tests.
- Config merge: Claude or Codex; both handle deep merge/null deletion correctly.

### Spec Feedback
- The specs describe “three-tier configuration” globally, but the config schema specifically standardizes only `config.json` and `config.local.json`. Gemini’s extra `base/custom/local config.json` stack is wrong against the schema, but the docs could still be clearer about “three-tier file layering” meaning prompts/fragments rather than top-level config files.
- The CLI spec should state more explicitly whether task-oriented `--node` values name the leaf or the full task address. The evaluation criteria and most command text imply full task addressing; Gemini interpreted it differently.
- The state-machine spec and archive/audit specs rely on audit-task semantics heavily; a short explicit statement that orchestrators never have tasks would have prevented Gemini’s interpretation.

### Model Tendencies
- **Claude**: depth over breadth. It favored correctness in the state core and cleaner abstractions, with better test discipline than Gemini and better package design than Codex.
- **Gemini**: breadth over depth. It attempted nearly everything, including ambitious doctor logic, but spec adherence weakened whenever the design became awkward.
- **Codex**: execution-oriented and pragmatic. It produced the most operational surface area and the best high-level tests, but it accepted more risk in foundational invariants and architectural cleanliness.
