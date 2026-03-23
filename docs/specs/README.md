# Specs

Living system specifications for Wolfcastle. These describe the current design and are the implementation reference.

## Currency Audit (2026-03-23)

Each spec was verified against current code. Status meanings:

- **Current**: Spec matches implementation; no action needed.
- **Needs update**: Implementation has moved beyond the spec. The spec is still directionally correct but omits features, changes names, or describes behavior that was later revised.

| Spec | Status | Drift Notes |
|------|--------|-------------|
| State Machine | Current | |
| Config Schema | Current | Code adds fields (Archive, TaskClasses, Planning) beyond spec, but these follow the same patterns and don't contradict it |
| Tree Addressing | Current | Code extends with archived node tracking and hierarchical child tasks; no contradictions |
| Pipeline Stage Contract | Current | Updated: added AARs in iteration context, spec-review auto-trigger section, planning pipeline cross-reference |
| Audit Propagation | Current | Updated: breadcrumb timestamp precision corrected to time.Time, gap ID format corrected to use node internal ID, breadcrumb task field sourced from --node argument, AAR section added |
| Archive Format | Current | Updated: Commit SHA documented in metadata table, summary.enabled config gate documented, audit section rendering updated to match combined Findings format with status badges |
| CLI Commands | Current | Updated: added `audit aar` and `audit report` subcommands, added `--detail` and `--archived` flags to `status` |
| Orchestrator Prompt | Current | Updated: marker table expanded with SKIP and CONTINUE markers and pass-type column, namespace isolation section added documenting scanTerminalMarker's validMarkers parameter, daemon behavior split by pass type |
| Structural Validation | Current | Updated: added CHILDREF_STATE_MISMATCH, ORPHANED_TEMP_FILE, INVALID_TASK_ID categories; five audit state categories added to severity and fix tables; built-in checks table updated |
| CI/CD Pipeline | Current | Integration timeout intentionally expanded to 300s (spec says 120s); `govulncheck` job added post-spec |
| Test Strategy | Current | `internal/testutil` shared helper package not yet extracted (low priority) |
| Production Hardening | Current | Readline integration for `unblock` pending; CONTRIBUTING.md not yet created |
| Testability and Decoupling | Current | Updated: callback marker parsing and property-based tests marked as not implemented with status notes; implementation priority table updated with status column |
| Prompt Externalization | Current | Updated: file renames documented, new prompts documented (context-headers.md, spec-review.md, stages/ subdirectory), deployment tree updated |
| Goroutine Architecture | Current | Updated: Phase 3 status corrected (StdoutPipe not io.Pipe), stall detector location documented, auto-archive inline documented, Section 6 rewritten |
| Store | Current | |
| Domain Repository Architecture | Current | |
| Orchestrator Planning Pipeline | Current | Updated: marker scanning branching documented (execution excludes CONTINUE, planning defaults to all), planning context updated to match BuildPlanningContext with note on extended metadata |
| Unknown Field Detection | Current | Implemented via `DisallowUnknownFields` in `internal/config/unknown.go`; warnings on Config struct per spec recommendation |
| tierfs Resolver Contract | Current | |
| RenderContext Rendering Contract | Current | |
| Git Provider Contract | Current | |
| Identity Domain Type Contract | Current | |
| ConfigRepository Contract (type renamed to Repository) | Current | Type renamed from `ConfigRepository` to `Repository` per commit 47252ec |
| ClassRepository Contract | Current | |
| MigrationService Contract | Current | |
| ScaffoldService Contract | Current | |
| ContextBuilder Contract | Current | |
| FindNextTask Navigation Invariants | Current | Property-based test invariants for navigation; no implementation drift |
| Config Show Command | Current | |
| Auto-Archive Service Contract | Current | |
| Config Write Commands | Current | |
| Dict-Format Pipeline Stages | Current | Implemented; `StageOrder` field and dict-format stages in source |
| Log Command Design | Current | |
| Config Docs Overhaul | Current | Planning spec for documentation restructuring; all four target pages exist |
| Deterministic Git | Needs update | Updated: config field names corrected (`skip_hooks_on_auto_commit`, `commit_prefix`), staging area section rewritten to reflect `commitDirect` implementation (GIT_INDEX_FILE reversed), commit message format updated with prefix/title/body support, `commitStateFlush` documented |
| Codebase Knowledge | Current | `internal/knowledge` package implements described file layout, read/append/budget mechanics |
| Template File Generation | Current | Design spec for future refactor; `TemplateRepository` and tier resolution exist in pipeline, scaffold files still use string builders |

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
| [Store](2026-03-15T00-01Z-state-store.md) | Unified Store abstraction replacing raw Load/Save pairs for file-backed state mutations |
| [Domain Repository Architecture](2026-03-16T00-00Z-domain-repository-architecture.md) | Domain-specific repositories replacing raw filepath.Join I/O |
| [Orchestrator Planning Pipeline](2026-03-17T00-00Z-orchestrator-planning-pipeline.md) | Lazy recursive planning for orchestrator nodes |
| [Unknown Field Detection](2026-03-20T15-57Z-unknown-field-detection.md) | Detection and reporting of unrecognized fields in config unmarshalling |
| [tierfs Resolver Contract](2026-03-18T20-23Z-tierfs-resolver-contract.md) | Resolver interface and FS implementation for three-tier file resolution |
| [RenderContext Rendering Contract](2026-03-18T21-05Z-rendercontext-rendering-contract.md) | Output format, section ordering, and division of responsibility for domain RenderContext methods and ContextBuilder |
| [Git Provider Contract](2026-03-18T21-44Z-git-provider-contract.md) | Provider interface and Service implementation for git operations via shell-out |
| [Identity Domain Type Contract](2026-03-18T21-50Z-identity-domain-type-contract.md) | Identity struct, constructors (IdentityFromConfig, DetectIdentity), and namespace derivation |
| [ConfigRepository Contract](2026-03-18T21-57Z-configrepository-contract.md) | Three-tier config resolution, merge algorithm, Write methods, and error behavior |
| [ClassRepository Contract](2026-03-18T22-31Z-classrepository-contract.md) | Class prompt resolution with hierarchical fallback, goroutine-safe Reload, Validate |
| [MigrationService Contract](2026-03-18T22-45Z-migrationservice-contract.md) | Directory layout and config migration for upgrading users |
| [ScaffoldService Contract](2026-03-18T22-48Z-scaffoldservice-contract.md) | Init and Reinit algorithms for .wolfcastle/ directory lifecycle |
| [ContextBuilder Contract](2026-03-18T23-11Z-contextbuilder-contract.md) | Full iteration context assembly, template resolution, and migration path from legacy functions |
| [FindNextTask Navigation Invariants](2026-03-20T15-22Z-findnexttask-navigation-invariants.md) | Seven property-based invariants for task navigation, suitable for direct translation to test predicates |
| [Config Show Command](2026-03-20T15-46Z-config-show-command.md) | CLI spec for `wolfcastle config show` with tier filtering, section filtering, and JSON envelope |
| [Dict-Format Pipeline Stages](2026-03-21T03-11Z-dict-format-stages.md) | Migration of pipeline.stages from array to dict format, stage_order field, validation rules |
| [Auto-Archive Service Contract](2026-03-21T12-27Z-auto-archive-service-contract.md) | Archive state model, file layout, move/restore/delete operations, daemon timer integration |
| [Config Write Commands](2026-03-21T14-23Z-config-write-commands.md) | CLI spec for `config set`, `unset`, `append`, `remove` with dot-notation paths and rollback |
| [Log Command Design](2026-03-21T18-00Z-log-command-design.md) | `wolfcastle log` display modes, session reconstruction, verbosity flags |
| [Config Docs Overhaul](2026-03-22T06-00Z-config-docs-overhaul.md) | Four-page configuration documentation restructure: quickstart, guide, reference, task classes |
| [Deterministic Git](2026-03-22T07-00Z-deterministic-git.md) | Daemon-owned git commits after every iteration, configurable via `git.*` fields, agent never touches git |
| [Codebase Knowledge](2026-03-22T08-00Z-codebase-knowledge.md) | Per-namespace markdown knowledge files accumulating codebase observations across tasks |
| [Template File Generation](2026-03-22T09-00Z-template-file-generation.md) | Move generated file content from string builders to overridable templates via three-tier resolution |

## Drafts

Specs that explore potential directions without proposing adoption. Implementation status as of 2026-03-23:

| Spec | Description | Implementation Status |
|------|-------------|-----------------------|
| [TUI](2026-03-15T00-02Z-tui.md) | Bubbletea-based terminal UI for observing and commanding the daemon | Not started. No bubbletea dependency or TUI code exists |
| [Worktree by Default](2026-03-15T00-03Z-worktree-by-default.md) | Running all daemon work in isolated git worktrees by default | Not started. The opt-in `--worktree` flag exists (spec Section 1 status quo) but none of the default-worktree behavior, auto-merge, or config gates have been built |
| [Task Classes](2026-03-15T00-04Z-task-classes.md) | Classification system for tasks routing each to a behavioral prompt | Pipeline implemented: class resolution, prompt injection, CLI validation, daemon startup validation, audit auto-assign, planning-time assignment, 54 config entries. Remaining: author 55 behavioral prompt `.md` files, model override dispatch, intake-level classification |

## Superseded

| Spec | Superseded By | Linkage Verified |
|------|---------------|------------------|
| [Wolfcastle FS](2026-03-16T00-00Z-wolfcastle-fs.md) | [Domain Repository Architecture](2026-03-16T00-00Z-domain-repository-architecture.md) | Yes. Frontmatter `superseded_by` field and prose link both point to the correct spec. Superseding spec is implemented and current |

## Naming

Specs use ISO 8601 timestamp prefixes per [ADR-011](../decisions/011-timestamp-filenames-for-docs.md): `2026-03-12T18-45Z-title.md`.
