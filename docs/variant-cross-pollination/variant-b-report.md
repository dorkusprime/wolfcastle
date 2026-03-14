# Variant-B Comparative Analysis

A comprehensive comparison of the two Wolfcastle implementations — variant-a (this codebase) and variant-b — covering architecture, design philosophy, strengths, weaknesses, and actionable opportunities for cross-pollination.

---

## 1. Architectural Overview

Both variants implement the same Wolfcastle specification: a model-agnostic autonomous project orchestrator with tree-structured work, multi-stage pipelines, deterministic state mutations, and audit propagation. They share the same ADRs (37 decisions) and living specifications. The divergence lies in *how* each interprets those contracts.

### Variant-A: The Cathedral

Variant-a adopts a **deeply modular, package-per-concern architecture** with 14 internal packages. Every domain boundary — state, config, pipeline, invoke, validate, tree, logging, output, project, archive, inbox — lives behind its own API surface. The `cmd/` directory holds 43 thin command files that delegate to internal packages. This is classical Go library design: small interfaces, clear dependency graphs, high testability.

### Variant-B: The Bazaar

Variant-b takes a **semi-flat approach**, placing most daemon logic, CLI commands, and state operations in the root package. Internal packages exist (`internal/config`, `internal/state`, `internal/runtime`, `internal/validate`, `internal/cli`) but carry less of the total weight. The root package files — `daemon.go`, `runtime_stage.go`, `runtime_mutation.go`, `status.go`, `doctor.go`, `app.go` — form a dense, interconnected surface where domain boundaries are conventions rather than compiler-enforced walls.

---

## 2. Package Structure Comparison

| Concern | Variant-A | Variant-B |
|---------|-----------|-----------|
| State types & mutations | `internal/state/` (7 files) | `internal/state/` (6 files) + `runtime_mutation.go` (root) |
| Configuration | `internal/config/` (4 files) | `internal/config/` (3 files) |
| Pipeline & prompts | `internal/pipeline/` (4 files) | `internal/runtime/` (4 files) + `runtime_stage.go` (root) |
| Model invocation | `internal/invoke/` (2 files) | `internal/runtime/` (shared) |
| Validation | `internal/validate/` (5 files) | `internal/validate/` (3 files) |
| Tree addressing | `internal/tree/` (2 files) | `internal/state/address.go` |
| Logging | `internal/logging/` (1 file) | Inline in daemon/runtime |
| Output envelopes | `internal/output/` (1 file) | Inline in command handlers |
| Scaffolding | `internal/project/` (3 files) | Root package files |
| Archive | `internal/archive/` (1 file) | `archive.go` (root) |
| Inbox | `internal/inbox/` (2 files) | Inline |
| CLI commands | `cmd/` (43 files) | Root package + `cmd/wolfcastle/main.go` |

**Takeaway:** Variant-a has roughly 2x the file count but each file is smaller and more focused. Variant-b is more compact but couples concerns that variant-a keeps separate.

---

## 3. Feature-by-Feature Comparison

### 3.1 Daemon Loop

Both implement serial execution with a supervisor pattern, self-healing on startup, and graceful shutdown via stop file + signal handling.

| Aspect | Variant-A | Variant-B |
|--------|-----------|-----------|
| Supervisor | `RunWithSupervisor()` wraps `Run()` | `runDaemonSupervisor()` wraps `runDaemonOnce()` |
| Iteration granularity | Full loop in `Run()` with internal iteration | Each iteration is `runDaemonOnce()` — cleaner restart boundary |
| Self-healing | Scans index for stale in_progress tasks | Same approach |
| Stop mechanism | Stop file + PID file + signals | Same |
| Background mode | Re-exec without `-d` flag | Same pattern |
| Worktree support | `--worktree` flag in start command | Same |

**Variant-B advantage:** The `runDaemonOnce()` pattern creates a cleaner boundary for the supervisor — each iteration is a self-contained function call, making crash recovery semantics more obvious.

### 3.2 Pipeline & Prompt Assembly

| Aspect | Variant-A | Variant-B |
|--------|-----------|-----------|
| Default stages | expand → file → execute | execute → summary |
| Prompt composition | Rules + script ref + stage prompt + context | Rules + script ref + stage prompt + context |
| Fragment resolution | Three-tier (base → custom → local) | Three-tier (base → custom → local) |
| Skip logic | `skip_prompt_assembly` flag per stage | Same, plus inbox/filing skip heuristics |
| Stage skipping | Stages can be disabled in config | Same, plus conditional skipping based on inbox state |

**Variant-B advantage:** The inbox-aware stage skipping is more sophisticated — stages are automatically skipped when the inbox has pending ideas or the tree needs filing, rather than requiring explicit configuration.

**Variant-A advantage:** The three-stage default pipeline (expand → file → execute) provides better separation of concerns in the autonomous workflow, while variant-b's two-stage default (execute → summary) collapses more work into a single model call.

### 3.3 State Management

Both use the same fundamental model: `RootIndex` + per-node `state.json` + upward propagation.

| Aspect | Variant-A | Variant-B |
|--------|-----------|-----------|
| State recomputation | `RecomputeState()` for orchestrators | `recomputeOrchestratorState()` + `recomputeLeafState()` |
| Propagation | `PropagateUp()` with cycle detection (max 100) | `propagateUp()` — similar |
| Audit sync | `SyncAuditLifecycle()` maps node state → audit status | Same mapping |
| Atomic writes | Temp file + rename pattern | Same pattern |
| Task ID generation | Sequential `task-N` | Same |

**Variant-B advantage:** Explicit `recomputeLeafState()` as a separate function makes the leaf state derivation logic more discoverable and testable.

### 3.4 Marker System

Both parse `WOLFCASTLE_*` prefixes from model output. The marker sets are nearly identical:

| Marker | Variant-A | Variant-B |
|--------|-----------|-----------|
| COMPLETE | Yes | Yes |
| BLOCK | Yes | Yes |
| BREADCRUMB | Yes | Yes |
| DECOMPOSE | Yes | Yes |
| GAP | Yes | Yes |
| FIX_GAP | Yes | Yes |
| SCOPE/SCOPE_FILES/SCOPE_SYSTEMS/SCOPE_CRITERIA | Yes | Yes |
| RESOLVE_ESCALATION | Yes | Yes |
| SUMMARY | Yes | Yes |
| YIELD | Yes | Yes |

No meaningful differences in marker handling.

### 3.5 Configuration

| Aspect | Variant-A | Variant-B |
|--------|-----------|-----------|
| Deep merge | `DeepMerge()` — clones dst, merges src recursively | Same approach |
| Null deletion | Null values in src delete keys from dst | Same |
| Array handling | Arrays in src replace dst entirely | Same |
| Validation timing | `ValidateStructure()` at Load, full `Validate()` before daemon | Post-merge validation |
| Validation count | 17 validators | Similar set |

**Variant-A advantage:** Two-phase validation (structural at load, full before daemon) catches problems earlier and with better error messages.

### 3.6 Validation & Doctor

| Aspect | Variant-A | Variant-B |
|--------|-----------|-----------|
| Categories | 17 structural categories | Similar categories, somewhat fewer |
| Severity levels | error, warning, info | error, warning, info |
| Auto-fix | `ApplyDeterministicFixes()` with atomic writes | Similar auto-fix capability |
| Normalization | Typo map (complete/completed/done → complete) | Same normalization |
| Startup gate | Blocks daemon if errors found | Same |
| Fix types | deterministic, model-assisted, manual | Same classification |

**Variant-A advantage:** The validation engine is more extensively decomposed — `engine.go`, `fix.go`, `normalize.go`, `types.go`, `categories.go` — making it easier to add new categories.

**Variant-B advantage:** Doctor also cleans up stale daemon artifacts (orphaned PID files, worktrees), which variant-a handles separately.

### 3.7 Audit System

| Aspect | Variant-A | Variant-B |
|--------|-----------|-----------|
| Scope, gaps, escalations, breadcrumbs | Full implementation | Full implementation |
| Audit review workflow | `audit codebase` with interactive approval | Two-phase: staged batch → approve/reject individual findings |
| Audit history | Not persisted beyond node state | `audit-review-history.json` records all decisions |

**Variant-B advantage:** The **staged audit review workflow** is significantly more sophisticated. Audit findings are saved as a pending batch (`audit-review.json`), reviewed in parallel, individually approved or rejected, and all decisions recorded in a durable history file. This creates a proper review pipeline rather than variant-a's immediate-approval model.

### 3.8 CLI Design

| Aspect | Variant-A | Variant-B |
|--------|-----------|-----------|
| Framework | Cobra (spf13/cobra) | Hand-rolled argument parsing |
| Output format | JSON envelope with `--json` flag | JSON envelope with `--json` flag |
| Envelope shape | `{ok, action, error, code, data}` | Same shape |
| Command count | ~43 | ~35 |
| Shell completion | Via Cobra's built-in completion | `completion bash\|zsh` command |

**Variant-A advantage:** Cobra provides consistent flag parsing, help generation, completion, and subcommand routing out of the box. Variant-b's hand-rolled parsing is more fragile and harder to extend.

**Variant-B advantage:** Fewer commands means a tighter, more opinionated surface — less cognitive overhead for users.

### 3.9 Overlap Advisory

| Aspect | Variant-A | Variant-B |
|--------|-----------|-----------|
| Implementation | Command + overlap detection | Token-based similarity with bigrams |
| Similarity method | Referenced but implementation details sparse | Bigram Jaccard index + shared term detection + stop-word filtering |

**Variant-B advantage:** The overlap detection algorithm is more explicitly defined — token-based similarity with bigrams and Jaccard scoring gives clear, reproducible results.

### 3.10 Testing

| Aspect | Variant-A | Variant-B |
|--------|-----------|-----------|
| Test files | 17 across internal packages | Fewer, concentrated in root package |
| Patterns | Table-driven, parallel execution, setup helpers | Functional tests with temp directories |
| Coverage areas | State, pipeline, config, output, validate, tree, logging, project, archive, inbox | State, audit workflow, status, identity |
| Isolation | Clean package boundaries = easy mocking | Filesystem I/O mixed with business logic |

**Variant-A advantage:** Significantly more comprehensive test coverage with better isolation. The package-per-concern architecture makes unit testing natural.

### 3.11 Logging

| Aspect | Variant-A | Variant-B |
|--------|-----------|-----------|
| Format | NDJSON per-iteration files | Same |
| Naming | `NNNN-ISO8601.jsonl` | Same |
| Record types | 12+ structured types (daemon_start, stage_start, marker_*, etc.) | Similar set |
| Retention | Max files + max age + optional compression | Same |
| AssistantWriter | io.Writer adapter for streaming | Same pattern |

No meaningful differences — both implementations follow the spec faithfully.

---

## 4. Strengths & Weaknesses

### Variant-A Strengths

1. **Modularity** — 14 internal packages with clean boundaries. Each package can be understood, tested, and modified independently. This is the primary architectural advantage.

2. **Test coverage** — 17 test files with table-driven tests, parallel execution, and good isolation. The test suite provides genuine confidence.

3. **Validation depth** — 17 structural categories with auto-fix, normalization, and a proper validation engine. The engine is composable and extensible.

4. **CLI framework** — Cobra provides professional-grade flag parsing, help text, shell completion, and subcommand routing.

5. **Pipeline sophistication** — Three default stages (expand → file → execute) provide better separation of autonomous work phases.

6. **Configuration safety** — Two-phase validation catches problems early. Deep merge with null deletion is well-tested.

7. **Embedded templates** — `go:embed` for base prompts and audit rules ensures binary self-containment.

### Variant-A Weaknesses

1. **File proliferation** — 43 command files and 14 packages creates navigation overhead. Finding where something happens requires understanding the package map.

2. **Immediate audit approval** — The `audit codebase` command uses inline interactive approval, which doesn't support async review or team workflows.

3. **No audit history** — Audit decisions aren't recorded beyond the current node state. There's no durable history of what was approved or rejected.

4. **Iteration boundary** — The daemon loop is a single `Run()` function. The supervisor wraps the entire loop, not individual iterations, making crash recovery slightly less clean.

5. **Overlap detection** — Less algorithmically defined than variant-b's bigram/Jaccard approach.

### Variant-B Strengths

1. **Staged audit review** — The batch → review → approve/reject → history pipeline is production-grade. It supports async workflows, team review, and audit trail compliance.

2. **Audit history persistence** — `audit-review-history.json` provides a durable record of all audit decisions, scopes, and created nodes.

3. **Clean iteration boundary** — `runDaemonOnce()` makes each iteration self-contained, simplifying supervisor restart semantics.

4. **Compact surface** — Fewer files and a flatter structure make it easier to hold the whole system in your head at once.

5. **Overlap algorithm** — Explicit bigram Jaccard similarity with stop-word filtering produces reproducible, tunable results.

6. **Daemon artifact cleanup** — Doctor cleans up orphaned PID files and worktrees, not just structural state issues.

7. **Inbox-aware stage skipping** — Stages skip automatically based on inbox state, reducing unnecessary model invocations.

### Variant-B Weaknesses

1. **Package coupling** — Root package files are heavily interconnected. Changing `runtime_mutation.go` risks affecting `daemon.go`, `status.go`, and command handlers.

2. **Hand-rolled CLI** — Argument parsing without a framework leads to inconsistent flag handling, missing validation, and maintenance burden.

3. **Test isolation** — Business logic mixed with filesystem I/O makes unit testing harder. Tests must create temp directories for even simple state operations.

4. **Validation engine** — Less decomposed than variant-a's engine. Adding new categories requires touching more code.

5. **Fewer tests** — Test coverage is thinner, concentrated in functional tests rather than unit tests.

---

## 5. Opportunities for Variant-A

Concrete improvements that can be incorporated from variant-b's design, ordered by estimated impact.

### 5.1 Staged Audit Review Workflow (High Impact)

**What:** Replace the immediate-approval model in `audit codebase` with variant-b's two-phase review pipeline.

**How:**
- Add `audit-review.json` and `audit-review-history.json` to the `.wolfcastle/` directory
- Implement `audit pending`, `audit approve`, `audit reject`, `audit history` commands
- Refactor `audit_codebase.go` to save findings as a pending batch rather than prompting inline
- Record all decisions (approve/reject, timestamp, scope, created nodes) in the history file

**Why:** Async review supports team workflows, provides audit compliance, and prevents rushed approval of model-generated findings.

### 5.2 Audit Decision History (High Impact)

**What:** Persist all audit decisions in a durable history file.

**How:**
- Define `AuditReviewEntry` type with decision, timestamp, scope, findings, created nodes
- Write to `audit-review-history.json` on every approve/reject action
- Add `audit history` command to query past decisions

**Why:** Without history, there's no way to understand past audit decisions or demonstrate compliance.

### 5.3 Clean Iteration Boundary (Medium Impact)

**What:** Refactor the daemon loop so each iteration is a self-contained function call.

**How:**
- Extract iteration logic from `Run()` into `RunOnce()` (or similar)
- Have `Run()` loop over `RunOnce()` calls
- Supervisor wraps `Run()` but restart semantics become cleaner because each iteration starts from a known-good state

**Why:** Cleaner crash recovery, easier testing of individual iterations, better separation between "loop control" and "iteration logic."

### 5.4 Inbox-Aware Stage Skipping (Medium Impact)

**What:** Automatically skip expand/file stages when there's nothing in the inbox, and skip execute when the tree needs filing.

**How:**
- Before running each stage, check inbox state
- If inbox is empty and stage is "expand", skip
- If no unprocessed items and stage is "file", skip
- Log skip decisions

**Why:** Reduces unnecessary model invocations (and cost) when stages have no work to do.

### 5.5 Daemon Artifact Cleanup in Doctor (Medium Impact)

**What:** Have `doctor --fix` clean up orphaned daemon artifacts (stale PID files, dead worktrees).

**How:**
- Add validation categories for STALE_PID_FILE and ORPHANED_WORKTREE
- In `ApplyDeterministicFixes()`, remove PID files for dead processes and clean orphaned worktrees
- Check process liveness via `os.FindProcess()` + signal(0)

**Why:** Currently these artifacts must be cleaned manually. Doctor is the natural home for this.

### 5.6 Overlap Detection Algorithm (Low Impact)

**What:** Adopt variant-b's bigram Jaccard similarity for overlap detection.

**How:**
- Implement bigram tokenization with stop-word filtering
- Compute Jaccard index on bigram sets
- Report overlapping nodes with similarity scores and shared terms
- Make similarity threshold configurable

**Why:** Produces more reproducible and tunable results than fuzzy matching.

### 5.7 Explicit Leaf State Recomputation (Low Impact)

**What:** Extract leaf state derivation into its own named function.

**How:**
- Create `RecomputeLeafState()` alongside `RecomputeState()` (which handles orchestrators)
- Call from `TaskComplete()`, `TaskBlock()`, `TaskUnblock()`

**Why:** Makes the leaf state derivation logic more discoverable and independently testable.

---

## 6. Things to Avoid from Variant-B

1. **Flat package structure** — Variant-b's own improvement specs acknowledge this as a problem. Variant-a's modular architecture is the better foundation.

2. **Hand-rolled CLI parsing** — Cobra is the right choice. Don't regress to manual argument parsing.

3. **Mixed I/O and logic** — Keep filesystem operations behind package boundaries. Don't let state mutation functions also handle file I/O directly.

4. **Fewer tests** — Variant-a's test coverage is a genuine advantage. Don't sacrifice it for compactness.

---

## 7. Summary

The two variants are more alike than different — they implement the same specification with the same fundamental patterns. Variant-a's advantage is **structural discipline**: clean packages, comprehensive tests, and a professional CLI framework. Variant-b's advantage is **workflow sophistication**: the staged audit review pipeline, decision history, and smarter stage skipping represent genuinely better user-facing workflows.

The highest-impact improvement for variant-a is adopting variant-b's audit review workflow — it transforms a synchronous, interactive process into an async, auditable pipeline. The daemon iteration boundary cleanup and inbox-aware skipping are solid quality-of-life improvements. The overlap algorithm and leaf state extraction are smaller wins that round out the picture.

None of variant-b's advantages require sacrificing variant-a's architectural strengths. The improvements can be layered onto the existing package structure cleanly.
