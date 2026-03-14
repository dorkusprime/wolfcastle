# Wolfcastle Implementation Evaluation — LLM Judge Prompt

You are an expert software evaluator. You will be given three independent, **in-progress** implementations of the same software project — **Wolfcastle** — each produced by a different AI model. Your job is to evaluate each implementation against the project's specifications and architecture decision records, then produce a structured comparative report.

**Important**: These implementations are works in progress, not finished products. Your evaluation should distinguish between code that is *missing* (not yet attempted — clean boundaries, stubs, TODOs) and code that is *wrong* (attempted but incorrect or contradicts the spec). Wrong code is significantly worse than missing code — a silent bug is a landmine, while a gap is obvious.

## What Is Wolfcastle

Wolfcastle is a **model-agnostic autonomous project orchestrator** written in Go. It breaks complex work into a persistent tree of projects, sub-projects, and tasks, then executes them through configurable multi-model pipelines. Key architectural properties:

- **Tree-structured work**: Orchestrator nodes (contain child nodes) and Leaf nodes (contain ordered task lists), traversed depth-first
- **JSON state, deterministic scripts**: All state mutations happen through validated Go scripts, never by the model directly
- **Three-tier configuration**: `base/` (defaults) → `custom/` (team overrides) → `local/` (personal overrides)
- **Configurable pipelines**: Multi-stage (expand → file → execute → summary), multi-model workflows defined in JSON
- **Distributed state files**: One `state.json` per node plus a root index for fast navigation
- **Four states only**: `not_started`, `in_progress`, `complete`, `blocked` — no others
- **Audit propagation**: Every leaf ends with an audit task; gaps escalate upward
- **Model invocation via CLI shell-out**: Models are external processes, invoked with assembled prompts piped to stdin
- **Daemon lifecycle**: A long-running Go daemon drives the pipeline loop, with graceful shutdown, PID management, and self-healing crash recovery
- **Engineer-namespaced projects**: Multiple engineers work concurrently without merge conflicts

---

## Evaluation Strategy

Follow this order to avoid anchoring bias and to build context efficiently:

1. **Survey all three first.** For each implementation, read `go.mod`, run `find . -type f -name '*.go' | head -60` to see the file layout, and skim the top-level `main.go` or `cmd/` entry point. Do this for all three before diving deep into any one.
2. **Gate check: does it build?** Run `go build ./...` and `go vet ./...` in each directory. Record whether each implementation compiles. A codebase that doesn't compile is categorically different from one that does — note this prominently but don't let it dominate every dimension's score.
3. **Drill into the same subsystem across all three** before moving to the next. Evaluate the state machine in all three, then config merge in all three, then the daemon in all three, etc. This gives you the best basis for comparison and prevents one implementation from setting an anchor.
4. **Run tests** if they exist (`go test ./...`). Record pass/fail counts.
5. **Score only after completing all subsystem reviews** for all three implementations.

---

## Evaluation Criteria

Evaluate each implementation across four dimensions. For each dimension, assign a score from 1 to 5 using the rubric anchors below. Provide specific evidence for every score.

### Rubric Anchors

These anchors apply to all four dimensions. Use them to calibrate your scores consistently:

| Score | Meaning |
|:-----:|---------|
| **1** | Not attempted, or fundamentally broken (e.g., won't compile, core logic inverted). |
| **2** | Attempted but mostly wrong or severely incomplete. Major gaps in the subsystem. |
| **3** | Partially implemented. Core happy path works but significant edge cases, invariants, or subsystems are missing or incorrect. |
| **4** | Substantially complete and mostly correct. Minor edge cases missed or minor deviations from spec. Code is functional and reasonable. |
| **5** | Fully implemented and correct. Handles edge cases, follows the spec precisely, production-quality code. |

### 1. Spec & ADR Compliance (Weight: 20%)

How faithfully does the implementation follow the specifications and architecture decision records? This is table stakes — all three models received the same design docs. Check against these critical specs and ADRs:

**State Machine** (highest priority):
- Four states only: `not_started`, `in_progress`, `complete`, `blocked`
- Valid transitions enforced (e.g., Not Started → Complete is invalid, Complete is terminal)
- Orchestrator state derived from children via `recompute_parent()` algorithm
- Upward-only propagation — never downward
- Single In Progress invariant (at most one task across the entire tree)
- Audit task always last in every leaf, auto-created, immovable, undeletable
- Failure tracking per-task with configurable decomposition threshold (default 10), max depth (default 5), hard cap (default 50)
- Decomposition converts a leaf into an orchestrator with new child leaves
- Self-healing: on startup, resume any stale In Progress task

**Distributed State Files**:
- Root index `state.json`: flat registry keyed by tree address with node metadata
- Per-node `state.json`: orchestrators have `children` array, leaves have `tasks` array
- Both schemas must match the spec'd JSON schemas (required fields, types, enums)
- Propagation writes to child, parent, and root index in the same script invocation

**Configuration Schema**:
- Two files: `config.json` (committed) and `config.local.json` (gitignored)
- Recursive deep merge for objects, full replacement for arrays, null deletion
- All config sections present: models, pipeline, logs, retries, failure, identity, summary, docs, validation, prompts, daemon, git, doctor, unblock, overlap_advisory
- Validation rules: model references resolve, stage names unique, identity only in local, type/constraint checking, prompt file existence

**Pipeline Stage Contract**:
- Stages execute sequentially in array order, one task per execute stage per iteration
- Prompt assembly: rule fragments → script reference → stage prompt → iteration context
- `skip_prompt_assembly` and `enabled` flags honored
- Model invocation via `exec.Command` with stdin pipe, own process group
- Stages communicate through side effects (filesystem/state), not output passing
- Summary stage is conditional and opt-out

**CLI Commands**:
- 21+ commands across categories: lifecycle, task, project, audit, documentation, archive, inbox, navigation, diagnostics, integration
- `--json` flag on all commands; `--node` accepts tree addresses
- Error output to stderr; JSON error envelope with `ok`, `error`, `code` fields
- Correct exit codes per command
- `.wolfcastle/` directory detection

**Structural Validation Engine**:
- 17 issue categories with correct severity assignments
- Deterministic vs model-assisted vs manual fix strategies
- Composable `Check` interface with `TreeState` abstraction
- Startup subset (fast checks that block daemon start on errors)
- Atomic fix application with rollback on failure
- Doctor prompt with guardrails (scope restriction, no creation, no deletion of work)

**Tree Addressing**:
- Slash-separated, kebab-case paths from root
- Per-node filesystem mapping to directories
- Address validation and creation mechanics

**Audit Propagation**:
- Breadcrumbs: append-only, timestamped, task-attributed
- Gaps: open/fixed lifecycle with deterministic IDs
- Escalations: child-to-parent with source tracking
- Audit scope with description, files, systems, criteria

**Archive Format**:
- ISO 8601 timestamp filenames
- Deterministic rollup process
- Optional model-written summary from `audit.result_summary`

**Key ADR compliance points** (spot-check these architectural decisions):

- **ADR-002**: All state and config in JSON (not YAML, TOML, etc.)
- **ADR-003**: State mutations through deterministic Go scripts, not model edits
- **ADR-004**: Model-agnostic — no hardcoded model names in core logic; models defined in config
- **ADR-009**: Three-tier file layering (base/custom/local) with correct merge semantics
- **ADR-012**: NDJSON logs with per-iteration files, rotation by count and age
- **ADR-013**: Model invocation via CLI shell-out, not SDK embedding
- **ADR-014**: Serial execution — one task at a time, depth-first traversal
- **ADR-018**: Deep merge for JSON objects, full replacement for arrays, null deletion
- **ADR-019**: Failure thresholds, decomposition, exponential backoff retries
- **ADR-020**: Daemon with PID file, SIGTERM/SIGINT handling, process group management, self-healing
- **ADR-024**: Distributed state files with per-node state.json and root index
- **ADR-025**: Validation engine as core infrastructure, not just a doctor feature
- **ADR-028**: Unblock resets task to Not Started (requires re-claim)

**Uninstructed additions**: If an implementation adds features, abstractions, or behaviors not present in the specs, note them. Score neutrally unless the addition contradicts the spec (negative) or fills a genuine gap the spec missed (note as observation, don't award extra credit).

### 2. Code Quality (Weight: 25%)

- **Idiomatic Go**: Proper error handling (no swallowed errors), interfaces over concrete types, composition over inheritance, clear package boundaries
- **Project structure**: Sensible `cmd/` and `pkg/` (or `internal/`) layout; separation of concerns between CLI, daemon, state management, validation, and pipeline
- **Error handling**: Errors wrapped with context (`fmt.Errorf("...: %w", err)`), fail-fast behavior, descriptive messages that would help debug a production issue
- **Testing**: Tests exist and cover critical paths (state machine transitions, propagation, validation checks, config merge, CLI parsing). Tests actually run and pass.
- **Testability**: Beyond whether tests exist — does the architecture *support* testing? Are interfaces used so the state machine can be tested without a filesystem? Can config merge be tested without touching disk? Are dependencies injectable?
- **No dead code**: No large blocks of commented-out code, unused imports, or placeholder functions
- **Naming**: Clear, consistent naming that follows Go conventions
- **Dependencies**: What's in `go.mod`? Appropriate use of standard library vs third-party packages. Did the implementation pull in heavy frameworks where the stdlib would suffice, or hand-roll something that a well-known library handles better?
- **Concurrency and safety**: The daemon is a long-running process with signal handling, PID files, child process management, and atomic file writes. Check for race conditions, correct PID file handling (check-and-create atomicity), proper signal propagation to child process groups, and safe concurrent access to shared state.

### 3. Completeness (Weight: 25%)

How much of the specified surface area is actually implemented vs stubbed/missing? For each item, note whether it is: **implemented**, **stubbed** (structure exists but logic is placeholder/TODO), or **absent**.

- [ ] State machine with all transitions and propagation
- [ ] All CLI commands (at least the core ones: init, start, stop, status, navigate, task claim/complete/block/unblock/add, project create, audit breadcrumb/escalate, doctor, follow, log)
- [ ] Configuration loading with two-file merge
- [ ] Pipeline stage execution loop
- [ ] Prompt assembly (four layers)
- [ ] Daemon lifecycle (PID, signals, graceful shutdown)
- [ ] Structural validation engine with all 17 issue types
- [ ] Tree addressing and navigation
- [ ] Audit propagation (breadcrumbs, gaps, escalations)
- [ ] Archive generation
- [ ] NDJSON logging
- [ ] Three-tier file layering for prompts and fragments

**Boundary quality**: For features that are not yet implemented, how clean is the boundary? Is there a clear interface or stub showing where the feature would plug in, or does the missing feature leave a ragged edge that would require refactoring to add later?

### 4. Correctness of Critical Algorithms (Weight: 30%)

Test these specific algorithms by tracing through the code. For each algorithm, note whether it is **correct**, **partially correct** (specify which cases fail), or **incorrect/absent**.

**`recompute_parent()` propagation**:
- All Not Started → Not Started
- Any In Progress or (Complete + Not Started) → In Progress
- All Complete → Complete
- All non-Complete are Blocked → Blocked
- Mixed Blocked + Not Started → In Progress (not Blocked, because Not Started can still progress)

**Depth-first task navigation**:
- Resumes In Progress task first (self-healing)
- Then finds first Not Started task in first actionable leaf, depth-first
- Skips Blocked and Complete nodes

**Config merge**:
- Deep merge objects recursively
- Arrays fully replaced (not element-merged)
- Null deletes the key
- Resolution order: hardcoded defaults ← config.json ← config.local.json

**Failure escalation**:
- failure_count < threshold → keep iterating
- failure_count = threshold AND depth < max → prompt decomposition
- failure_count = threshold AND depth = max → auto-block
- failure_count = hard_cap → auto-block regardless

**Atomic fix application**:
- All fixes to one file applied as single write
- Cross-file fixes in dependency order (leaf → parent → root)
- Post-fix re-validation; rollback if new issues introduced

---

## Output Format

Save your evaluation report as a markdown file named `wolfcastle-eval-report-{cli}-2.md`, where `{cli}` is the name of the CLI command used to invoke you (e.g., `wolfcastle-eval-report-claude.md`, `wolfcastle-eval-report-gemini.md`, `wolfcastle-eval-report-codex.md`). Save it in the project root directory (same directory as this prompt file).

Produce your evaluation using this structure:

```markdown
# Wolfcastle Implementation Evaluation Report

## Build Status

| Implementation | `go build ./...` | `go vet ./...` | `go test ./...` | Notes |
|----------------|:-----------------:|:--------------:|:---------------:|-------|
| Claude | Pass/Fail | Pass/Fail | X pass, Y fail | ... |
| Gemini | Pass/Fail | Pass/Fail | X pass, Y fail | ... |
| Codex | Pass/Fail | Pass/Fail | X pass, Y fail | ... |

## Per-Implementation Scores

### Implementation A: [model name]

| Dimension | Weight | Score (1-5) | Notes |
|-----------|:------:|:-----------:|-------|
| Spec & ADR Compliance | 20% | X | ... |
| Code Quality | 25% | X | ... |
| Completeness | 25% | X | ... |
| Algorithm Correctness | 30% | X | ... |
| **Weighted Total** | | **X.XX** | |

#### Strengths
- ...

#### Weaknesses
- ...

#### Critical Deviations from Spec
- ...

#### Wrong vs Missing
Explicitly list: (a) features that were attempted but implemented incorrectly, and (b) features that are cleanly absent or stubbed. This distinction matters more than the raw completeness score.

[Repeat for Implementation B and C]

## Comparative Analysis

### Head-to-Head Summary

| Dimension | Weight | Claude | Gemini | Codex |
|-----------|:------:|:------:|:------:|:-----:|
| Spec & ADR Compliance | 20% | X | X | X |
| Code Quality | 25% | X | X | X |
| Completeness | 25% | X | X | X |
| Algorithm Correctness | 30% | X | X | X |
| **Weighted Total** | | **X.XX** | **X.XX** | **X.XX** |

### Notable Differences
Where did the implementations diverge in their interpretation of the specs? Were any of these divergences defensible or even improvements?

### Common Mistakes
What did all three get wrong or miss? This may indicate ambiguity in the specs themselves.

### Ranking
1. **[model]** — [one-sentence justification]
2. **[model]** — [one-sentence justification]
3. **[model]** — [one-sentence justification]

## Overall Interpretation

This is the most important section of the report. Step back from the scores and write a thorough narrative analysis.

### Implementation Philosophies
How did each model *approach* the problem? Did one focus on getting the core state machine right before building outward? Did another scaffold the full CLI surface first and fill in logic later? Did any take a notably different architectural approach (e.g., different package structure, different abstraction boundaries) even while implementing the same spec? Characterize each model's "personality" as an implementer.

### Strengths Worth Adopting
For each implementation, identify the best ideas, patterns, or decisions that the other two could learn from. Be specific — name the file, the function, the design choice. Examples:
- "Claude's `pkg/validate` uses a composable `Check` interface that makes adding new validation categories trivial. Gemini and Codex both hardcode their checks in a single function."
- "Codex's config merge handles the null-deletion edge case that both Claude and Gemini miss entirely."

### Weaknesses and Pitfalls
For each implementation, identify the most significant problems — not just missing features, but *structural* issues that would make the codebase hard to extend, maintain, or debug. Flag anything that would be painful to fix later (e.g., state mutation logic scattered across packages, missing error context that would make debugging production issues difficult).

### Cross-Pollination Opportunities
If you could take the best parts of all three and assemble an ideal implementation, what would you take from each? Be concrete:
- Which implementation's package structure would you use as the skeleton?
- Which implementation's state machine logic would you drop in?
- Which implementation's test suite would you keep?
- Which implementation's error handling patterns would you adopt?

### Spec Feedback
Based on seeing three independent interpretations, where are the specs ambiguous, underspecified, or arguably wrong? If all three models made the same mistake, the spec is probably the problem. If two diverged on the same point, the spec likely needs clarification. List specific spec sections and what you'd recommend changing.

### Model Tendencies
What does this evaluation reveal about each model's tendencies as a code generator? Consider:
- **Breadth vs depth**: Did the model try to implement everything shallowly, or focus on getting core subsystems right?
- **Specification adherence vs improvisation**: Did the model follow the spec literally, or did it make its own design choices? Were those choices good?
- **Code style**: Verbose vs terse? Over-engineered vs under-engineered? Heavy on abstractions vs pragmatically concrete?
- **Testing discipline**: Did the model write tests proactively, or skip them? Are the tests meaningful or superficial?
- **Error handling**: Robust and contextual, or optimistic and bare?
```

---

## Instructions for the Evaluator

1. **Follow the Evaluation Strategy** (above) — survey all three, gate-check builds, then drill into subsystems across all three before scoring.
2. **Be precise**: cite specific files, functions, and line numbers when noting strengths or deviations.
3. **Spec is ground truth**: if an implementation makes a reasonable choice that contradicts the spec, it is still a deviation. Note whether the deviation is harmful or arguably an improvement, but score against the spec.
4. **Wrong is worse than missing**: an incorrect `recompute_parent()` that silently produces wrong state is worse than a missing archive generator. Weight your scores accordingly.
5. **Don't penalize twice**: if a missing feature causes failures in multiple dimensions, note it once and reference it elsewhere.
6. **Weight matters**: Algorithm Correctness (30%) is the heaviest weight — a wrong `recompute_parent()` or broken config merge is a showstopper regardless of how clean the code looks. Spec & ADR Compliance (20%) is the lightest because following the design docs is table stakes; the interesting signal is in execution.
7. **Partial credit**: a feature that is partially implemented (e.g., config merge works for objects but doesn't handle null deletion) gets partial credit, not zero.
8. **In-progress is expected**: do not treat incompleteness as a fundamental failing. Instead, evaluate what *is* there for quality and correctness, and evaluate what *isn't* there for how cleanly it could be added (boundary quality).

## Implementation Directories

The three implementations to evaluate are:

- **Claude**: `./wolfcastle-claude/`
- **Gemini**: `./wolfcastle-gemini/`
- **Codex**: `./wolfcastle-codex/`

The canonical specs and ADRs are in `./wolfcastle/docs/`.
