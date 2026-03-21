# Specs

Living system specifications for Wolfcastle. These describe the current design and are the implementation reference.

## Currency Audit (2026-03-21)

Each spec was verified against current code. Status meanings:

- **Current**: Spec matches implementation; no action needed.
- **Needs update**: Implementation has moved beyond the spec. The spec is still directionally correct but omits features, changes names, or describes behavior that was later revised.

| Spec | Status | Drift Notes |
|------|--------|-------------|
| State Machine | Current | |
| Config Schema | Current | Code adds fields (Archive, TaskClasses, Planning) beyond spec, but these follow the same patterns and don't contradict it |
| Tree Addressing | Current | Code extends with archived node tracking and hierarchical child tasks; no contradictions |
| Pipeline Stage Contract | **Needs update** | Missing AARs (richer than breadcrumbs, fully integrated into execution context), spec-review auto-trigger mechanism, and cross-reference to planning pipeline |
| Audit Propagation | **Needs update** | Breadcrumb timestamps use minute precision (spec says second); gap IDs use node internal ID, not path slug; CLI breadcrumb command doesn't resolve execution context for task field |
| Archive Format | **Needs update** | Missing Commit SHA in metadata table; no `summary.enabled` config gate; audit section combines gaps+fixes into single list with status badges instead of separate sections |
| CLI Commands | **Needs update** | Missing `audit aar` and `audit report` subcommands; missing `status --detail` and `status --archived` flags |
| Orchestrator Prompt | **Needs update** | `scanTerminalMarker` does not branch between planning and execution passes; CONTINUE marker is in the execution-path scan list, violating the spec's marker namespace isolation |
| Structural Validation | **Needs update** | Missing CHILDREF_STATE_MISMATCH category (Error severity, deterministic fix, fully tested); five audit state integrity categories defined in Section 1 but absent from severity and fix tables |
| CI/CD Pipeline | Current | Integration timeout intentionally expanded to 300s (spec says 120s); `govulncheck` job added post-spec |
| Test Strategy | Current | `internal/testutil` shared helper package not yet extracted (low priority) |
| Production Hardening | Current | Readline integration for `unblock` pending; CONTRIBUTING.md not yet created |
| Testability and Decoupling | **Needs update** | Callback-based marker parsing (P1) not implemented; property-based propagation tests via `testing/quick` (P2) not implemented; ~75% of spec realized |
| Prompt Externalization | **Needs update** | `decomposition-guidance.md` renamed to `decomposition.md`; `summary-required.md` uses CLI command instead of WOLFCASTLE_SUMMARY marker; `expand-context.md` and `file-context.md` are minimal stubs; new prompts (intake-context.md, context-headers.md, spec-review.md) not documented |
| Goroutine Architecture | **Needs update** | Real-time I/O via `io.Pipe` (spec Phase 3) not implemented; marker scanning still blocks on full output post-process; stall detector lives in invoke layer, not daemon goroutine topology; auto-archive runs inline per ADR 2026-03-21T12-57Z (spec predates this decision) |
| StateStore | Current | |
| Domain Repository Architecture | Current | |
| Orchestrator Planning Pipeline | **Needs update** | Marker scanning doesn't branch by pass type (planning vs execution); planning context omits extended task metadata (type, deliverables, constraints, acceptance criteria) |

## Specs

| Spec | Description |
|------|-------------|
| [State Machine](2026-03-12T00-00Z-state-machine.md) | Node lifecycle, four states, valid transitions, failure tracking, decomposition, state propagation across distributed files |
| [Config Schema](2026-03-12T00-01Z-config-schema.md) | Full JSON schema for the three-tier config (base/custom/local config.json) with defaults, merge semantics, doctor, audit, and overlap advisory |
| [Tree Addressing](2026-03-12T00-02Z-tree-addressing.md) | Address format, per-node filesystem mapping, root index, navigation algorithm, hybrid task descriptions |
| [Pipeline Stage Contract](2026-03-12T00-03Z-pipeline-stage-contract.md) | Stage definition, invocation, prompt assembly, default pipeline, error handling |
| [Audit Propagation](2026-03-12T00-04Z-audit-propagation.md) | Breadcrumbs, gaps, escalation mechanics, audit task invariant, scope definition |
| [Archive Format](2026-03-12T00-05Z-archive-format.md) | Entry template, filename convention, rollup process, inline summary via ADR-036 |
| [CLI Commands](2026-03-12T00-06Z-cli-commands.md) | Detailed spec for all CLI commands including doctor, install, spec, inbox, status --all |
| [Orchestrator Prompt](2026-03-12T00-07Z-orchestrator-prompt.md) | Prompt assembly, phase structure, guardrails, terminal markers, per-stage differences |
| [Structural Validation](2026-03-13T00-00Z-structural-validation.md) | Validation engine, 20+ issue types, severity levels, deterministic vs model-assisted fixes, Go API |
| [CI/CD Pipeline](2026-03-14T00-00Z-ci-cd-pipeline.md) | GitHub Actions CI workflow, quality gates, GoReleaser release automation, versioning |
| [Test Strategy](2026-03-14T00-01Z-test-strategy.md) | Three-tier test strategy (unit/integration/smoke), coverage targets, shared test infrastructure |
| [Production Hardening](2026-03-14T00-02Z-production-hardening.md) | State file locking, structured log levels, force stop, error message standards, interactive UX |
| [Testability and Decoupling](2026-03-14T00-03Z-testability-and-decoupling.md) | Integration tests, multi-pass doctor, time injection, centralized defaults, callback marker parsing, property-based propagation tests |
| [Prompt Externalization](2026-03-14T00-04Z-prompt-externalization.md) | Full prompt inventory, template variable system, new prompt files, override examples, migration plan |
| [Goroutine Architecture](2026-03-15T00-00Z-goroutine-architecture.md) | Communication architecture between concurrent goroutines, signal chain, real-time I/O pipeline |
| [StateStore](2026-03-15T00-01Z-state-store.md) | Unified StateStore abstraction replacing raw Load/Save pairs for file-backed state mutations |
| [Domain Repository Architecture](2026-03-16T00-00Z-domain-repository-architecture.md) | Domain-specific repositories replacing raw filepath.Join I/O |
| [Orchestrator Planning Pipeline](2026-03-17T00-00Z-orchestrator-planning-pipeline.md) | Lazy recursive planning for orchestrator nodes |

## Drafts

Specs that explore potential directions without proposing adoption.

| Spec | Description |
|------|-------------|
| [TUI](2026-03-15T00-02Z-tui.md) | Bubbletea-based terminal UI for observing and commanding the daemon |
| [Worktree by Default](2026-03-15T00-03Z-worktree-by-default.md) | Running all daemon work in isolated git worktrees by default |
| [Task Classes](2026-03-15T00-04Z-task-classes.md) | Classification system for tasks routing each to a behavioral prompt |

## Superseded

| Spec | Superseded By |
|------|---------------|
| [Wolfcastle FS](2026-03-16T00-00Z-wolfcastle-fs.md) | [Domain Repository Architecture](2026-03-16T00-00Z-domain-repository-architecture.md) |

## Naming

Specs use ISO 8601 timestamp prefixes per [ADR-011](../decisions/011-timestamp-filenames-for-docs.md): `2026-03-12T18-45Z-title.md`.
