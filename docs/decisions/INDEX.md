# Wolfcastle Architecture Decision Records

| # | Decision | Status | Date |
|---|----------|--------|------|
| 001 | [ADR Format](001-adr-format.md) | Accepted | 2026-03-12 |
| 002 | [JSON for Configuration and State](002-json-config-and-state.md) | Accepted | 2026-03-12 |
| 003 | [Deterministic Scripts with Static Documentation](003-deterministic-scripts-with-static-docs.md) | Accepted | 2026-03-12 |
| 004 | [Model-Agnostic Design](004-model-agnostic.md) | Accepted | 2026-03-12 |
| 005 | [Composable Rule Fragments with Sensible Defaults](005-composable-rule-fragments.md) | Accepted | 2026-03-12 |
| 006 | [Configurable Pipelines](006-configurable-pipelines.md) | Accepted | 2026-03-12 |
| 007 | [Audit Model Preserved, Mechanics via Scripts](007-audit-via-scripts.md) | Accepted | 2026-03-12 |
| 008 | [Tree-Addressed Operations](008-tree-addressed-operations.md) | Accepted | 2026-03-12 |
| 009 | [Distribution, Project Layout, and Three-Tier File Layering](009-distribution-and-project-layout.md) | Accepted | 2026-03-12 |
| 010 | [Wolfcastle-Managed Documentation (ADRs and Specs)](010-wolfcastle-managed-docs.md) | Accepted | 2026-03-12 |
| 011 | [ISO 8601 Timestamp Filenames for ADRs and Specs](011-timestamp-filenames-for-docs.md) | Accepted | 2026-03-12 |
| 012 | [NDJSON Logs with Configurable Rotation](012-ndjson-logs-with-rotation.md) | Accepted | 2026-03-12 |
| 013 | [Model Invocation via CLI Shell-Out with Pipeline Configuration](013-model-invocation-and-pipeline-config.md) | Accepted | 2026-03-12 |
| 014 | [Serial Execution with Node Scoping](014-serial-execution-with-node-scoping.md) | Accepted | 2026-03-12 |
| 015 | [Git Branch Behavior and Optional Worktree Isolation](015-git-branch-behavior.md) | Accepted | 2026-03-12 |
| 016 | [Archive Format with Deterministic Rollup and Model Summary](016-archive-format-and-summary.md) | Accepted | 2026-03-12 |
| 017 | [Script Reference via Prompt Injection](017-script-reference-via-prompt-injection.md) | Accepted | 2026-03-12 |
| 018 | [Merge Semantics for Config and Prompt Layering](018-merge-semantics.md) | Accepted | 2026-03-12 |
| 019 | [Failure Handling, Decomposition, and Retry Thresholds](019-failure-decomposition-and-retry.md) | Accepted | 2026-03-12 |
| 020 | [Daemon Lifecycle and Process Management](020-daemon-lifecycle.md) | Accepted | 2026-03-12 |
| 021 | [CLI Command Surface](021-cli-command-surface.md) | Accepted | 2026-03-12 |
| 022 | [Security Model: User-Configured, Wolfcastle-Transparent](022-security-model.md) | Accepted | 2026-03-12 |
| 023 | [Decisions Emerging from Spec Phase](023-decisions-from-speccing.md) | Accepted | 2026-03-12 |
| 024 | [Distributed State Files, Task Working Documents, and Runtime Aggregation](024-distributed-state-and-task-docs.md) | Accepted | 2026-03-13 |
| 025 | [Wolfcastle Doctor: Structural Validation and Repair](025-doctor-command.md) | Accepted | 2026-03-13 |
| 026 | [wolfcastle install Command and Claude Code Skill](026-install-command-and-skills.md) | Accepted | 2026-03-13 |
| 027 | [Cross-Engineer Overlap Advisory](027-overlap-advisory.md) | Accepted | 2026-03-13 |
| 028 | [Three-Tier Unblock Workflow](028-unblock-workflow.md) | Accepted | 2026-03-13 |
| 029 | [Codebase Audit Command with Discoverable Scopes](029-codebase-audit-command.md) | Accepted | 2026-03-13 |
| 030 | [Help at Every CLI Level](030-comprehensive-help.md) | Accepted | 2026-03-13 |
| 031 | [In-Flight Specs with State-Based Linkage](031-in-flight-specs.md) | Accepted | 2026-03-13 |
| 032 | [Go Project Structure and Cobra CLI Framework](032-go-project-structure.md) | Accepted | 2026-03-13 |
| 033 | [Embedded Templates via go:embed](033-embedded-templates.md) | Accepted | 2026-03-13 |
| 034 | [Inbox Format and Lifecycle](034-inbox-format.md) | Accepted (partially superseded by ADR-064) | 2026-03-13 |
| 035 | [Model-Driven Decomposition via CLI](035-decomposition-via-cli.md) | Accepted | 2026-03-13 |
| 036 | [Summaries via Inline Marker, Not Separate Stage](036-summary-via-inline-marker.md) | Accepted | 2026-03-13 |
| 037 | [Daemon Dual Output (Console + NDJSON)](037-daemon-logging-dual-output.md) | Superseded by ADR-097 | 2026-03-13 |
| 038 | [Staged Audit Review Workflow](038-staged-audit-review.md) | Accepted | 2026-03-14 |
| 039 | [Clean Daemon Iteration Boundary](039-daemon-iteration-boundary.md) | Accepted | 2026-03-14 |
| 040 | [Daemon Artifact Cleanup in Doctor](040-daemon-artifact-cleanup.md) | Accepted | 2026-03-14 |
| 041 | [Algorithmic Overlap Detection](041-algorithmic-overlap-detection.md) | Accepted | 2026-03-14 |
| 042 | [State File Locking](042-state-file-locking.md) | Accepted | 2026-03-14 |
| 043 | [CI/CD Pipeline and Quality Gates](043-ci-cd-pipeline.md) | Accepted | 2026-03-14 |
| 044 | [Test Strategy: Unit, Integration, and Smoke](044-test-strategy.md) | Accepted | 2026-03-14 |
| 045 | [Daemon Package Decomposition](045-daemon-package-decomposition.md) | Accepted | 2026-03-14 |
| 046 | [Structured Log Levels](046-structured-log-levels.md) | Accepted | 2026-03-14 |
| 047 | [Release Automation via GoReleaser](047-release-automation.md) | Accepted | 2026-03-14 |
| 048 | [Interactive Session UX](048-interactive-session-ux.md) | Accepted | 2026-03-14 |
| 049 | [Lint Policy via golangci-lint](049-lint-policy.md) | Accepted | 2026-03-14 |
| 050 | [Integration Test Harness via CLI Dispatch](050-integration-test-harness.md) | Accepted | 2026-03-14 |
| 051 | [Multi-Pass Doctor with Fix Verification](051-multi-pass-doctor.md) | Accepted | 2026-03-14 |
| 052 | [Time Injection for Deterministic Testing](052-time-injection.md) | Accepted | 2026-03-14 |
| 053 | [Centralized Configuration Defaults](053-centralized-config-defaults.md) | Accepted | 2026-03-14 |
| 054 | [Callback-Based Marker Parsing](054-callback-marker-parsing.md) | Accepted (not implemented) | 2026-03-14 |
| 055 | [Property-Based Propagation Tests](055-property-based-propagation-tests.md) | Accepted | 2026-03-14 |
| 056 | [Cobra Dependency Evaluation](056-cobra-evaluation.md) | Accepted | 2026-03-14 |
| 057 | [All Prompts Externalized to Overridable Markdown Files](057-externalized-prompts.md) | Accepted | 2026-03-14 |
| 058 | [Small Package Consolidation](058-small-package-consolidation.md) | Accepted | 2026-03-14 |
| 059 | [Pre-Commit Hooks via .githooks/](059-pre-commit-hooks.md) | Accepted | 2026-03-14 |
| 060 | [Platform-Specific Code via Build Tags](060-platform-build-tags.md) | Accepted | 2026-03-14 |
| 061 | [MIT License](061-mit-license.md) | Accepted | 2026-03-14 |
| 062 | [Realistic Model Mocks for Integration Testing](062-realistic-model-mocks.md) | Accepted | 2026-03-14 |
| 063 | [Three-Tier Configuration](063-config-three-tier.md) | Accepted | 2026-03-14 |
| 064 | [Consolidated Intake Stage and Parallel Inbox Processing](064-intake-stage-and-parallel-inbox.md) | Accepted | 2026-03-14 |
| 065 | [Typed Error Categories](065-typed-errors.md) | Accepted | 2026-03-14 |
| 066 | [Scoped Script References per Pipeline Stage](066-scoped-script-references.md) | Accepted | 2026-03-15 |
| 067 | [Terminal Markers Only; Audit Mutations via CLI](067-terminal-markers-only-cli-audit.md) | Accepted | 2026-03-15 |
| 068 | [Unified Store for File-Backed State](068-unified-state-store.md) | Accepted | 2026-03-15 |
| 069 | [Task Deliverables](069-task-deliverables.md) | Accepted | 2026-03-15 |
| 070 | [Deliverable Change Detection](070-deliverable-change-detection.md) | Accepted (mechanism changed) | 2026-03-16 |
| 071 | [Discovery-First Intake Pipeline](071-discovery-first-intake.md) | Accepted | 2026-03-16 |
| 072 | [Pre-blocking Not-Started Tasks](072-pre-blocking-not-started-tasks.md) | Accepted | 2026-03-16 |
| 073 | [Follow-to-Log Rename](073-follow-to-log-rename.md) | Accepted | 2026-03-16 |
| 074 | [Status Tree View](074-status-tree-view.md) | Accepted | 2026-03-16 |
| 075 | [Foreground Process Group Reclaim](075-foreground-process-group-reclaim.md) | Superseded by ADR-076 | 2026-03-16 |
| 076 | [Signal Handling and Terminal Restoration After Model Invocation](076-signal-handling-and-terminal-restoration.md) | Accepted | 2026-03-16 |
| 077 | [System Directory Restructure](077-system-directory-restructure.md) | Accepted | 2026-03-16 |
| 078 | [Task.RenderContext Parameterless Refactoring](078-task-rendercontext-parameterless.md) | Accepted | 2026-03-18 |
| 079 | [NodeState.RenderContext Phantom taskID Parameter](079-nodestate-rendercontext-phantom-taskid.md) | Accepted | 2026-03-18 |
| 080 | [Sequential Inbox Intake](080-sequential-inbox-intake.md) | Accepted | 2026-03-21 |
| 081 | [Use Interface for tierfs.Resolver](081-tierfs-resolver-interface.md) | Accepted | 2026-03-18 |
| 082 | [Hardcode Tier Order in tierfs](082-hardcode-tier-order.md) | Accepted | 2026-03-18 |
| 083 | [Fluent Builder Pattern for testutil.Environment](083-fluent-builder-testutil.md) | Accepted | 2026-03-18 |
| 084 | [Shell Out to Git Binary for Git Operations](084-shell-out-to-git.md) | Accepted | 2026-03-18 |
| 085 | [Concrete Identity Struct with Constructor Validation](085-concrete-identity-struct.md) | Accepted | 2026-03-18 |
| 086 | [One-Level Hierarchical Fallback for Class Prompt Resolution](086-class-prompt-fallback.md) | Accepted | 2026-03-18 |
| 087 | [Use Interfaces in ScaffoldService to Break Import Cycles](087-scaffoldservice-interfaces.md) | Accepted | 2026-03-18 |
| 088 | [DaemonRepository Uses Concrete Struct with Explicit Parameters](088-daemonrepository-concrete.md) | Accepted | 2026-03-18 |
| 089 | [Auto-Archive Runs Inline in RunOnce](089-auto-archive-inline.md) | Accepted | 2026-03-21 |
| 090 | [WithConfig Writes to Custom Tier](090-withconfig-custom-tier.md) | Accepted | 2026-03-18 |
| 091 | [Task.RenderContext Parameterless Refactoring](091-task-rendercontext-parameterless.md) | Duplicate of ADR-078 | 2026-03-18 |
| 092 | [Result.Unavailable Field over Sentinel Error](092-result-unavailable-field.md) | Accepted | 2026-03-20 |
| 093 | [Separate GIT_INDEX_FILE for Daemon Commits](093-separate-git-index-file.md) | Superseded | 2026-03-22 |
| 094 | [PIDChecker Interface Decouples Validate from Daemon](094-pidchecker-interface.md) | Accepted (supersedes ADR-088) | 2026-03-22 |
| 095 | [Parallel Sibling Execution](095-parallel-sibling-execution.md) | Accepted | 2026-03-23 |
| 096 | [Config Versioning and Migration](096-config-versioning.md) | Accepted | 2026-03-24 |
| 097 | [Unified Log Output](097-unified-log-output.md) | Accepted (supersedes ADR-037) | 2026-04-05 |
