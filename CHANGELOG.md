# Changelog

## 0.6.1

### Bug Fixes
- TUI launched from outside a project directory (cold start) now updates live. The poll chain, watcher event drain, and per-node state polling all start unconditionally in Init(), so instance auto-discovery works correctly. (#268)
- Orchestrator state propagation no longer overwrites `NeedsPlanning`. PropagateUp and reconcileOrchestratorStates hold orchestrators at in_progress when planning is pending, preventing completion_review from being silently skipped. (#261)
- Stale in-memory index no longer blocks completion_review triggers. The index entry for a completed node is refreshed from disk before checkReplanningTriggers runs. (#261)
- Log renderer in `wolfcastle start` no longer replays old session logs. SnapshotExisting records file sizes before the daemon starts; only new content is rendered. (#263, #267)
- Per-node state polling added as a fallback alongside fsnotify. macOS kqueue can silently drop events for certain paths; the 2-second poll tick now checks subscribed node mtimes directly. (#268)

### Quality
- Extracted `state.StateFileName` constant, replacing 6 hardcoded `"state.json"` literals. (#262)
- Wrapped bare error return in `runPlanningPass` with context. (#262)
- Integration test for the full completion_review loop (`TestDaemon_ExploratoryReview_CreatesRemediationLeaf`). (#261)
- Unit test for PropagateUp NeedsPlanning guard. (#261)
- Unit test for per-node mtime polling in the TUI watcher. (#268)

### Documentation
- Voice fixes: replaced "helps" with direct phrasing, em dashes with colons. (#262)
- Screenshot generator: orchestrator nodes include ChildRef arrays, stall model prevents state mutation during capture. Regenerated all 16 screenshots. (#264, #265, #266)

## 0.6.0

### Features
- Interactive TUI: launching `wolfcastle` with no subcommand opens a full Bubbletea v2 terminal interface. Node tree with vi-style navigation, dashboard overview, real-time log stream, node and task detail views, inbox, instance switcher, and notification toasts. Inbox, logs, and daemon start/stop are modal overlays that take focus and dismiss with Esc; the daemon toggle requires Enter confirmation. The welcome screen lists running daemons alongside a directory browser for `init`. State refreshes via fsnotify with per-leaf subscriptions (including leaves added mid-session by daemon decomposition) and a 2-second polling fallback. Log rendering extracts human-readable content from structured NDJSON; Recent Activity filters to milestones only. All displayed timestamps are local. (#228, #231, #232, #233, #234, #236, #237, #238, #240, #241, #242, #243, #244)
- Multi-process architecture: per-worktree daemon locking replaces the global lock, so multiple daemons can run concurrently across separate worktrees. A new `internal/instance` registry under `~/.wolfcastle/instances/` routes commands by CWD with longest-prefix matching and prunes stale PIDs on read. (#227)
- `--instance` flag on `start`, `stop`, `status`, `log`, and `inbox` to target a daemon by worktree path instead of CWD discovery. Useful when acting on a daemon from outside its worktree, or when multiple daemons are running and you want to be explicit. (#239)
- Worktree-aware tier regeneration on `start`: when `.wolfcastle/` exists with tracked content but the gitignored base/local tiers are missing (the fresh-checkout case), `start` regenerates them automatically before loading config instead of failing with a confusing config-load error. (#239)

### Bug Fixes
- `DeriveParentStatus` treats blocked children with `superseded` or `decomposed` reasons as effectively complete, ending the retry-decompose loop on parents whose remaining children were intentionally cleared. (#230)
- Daemon accepts COMPLETE results when deliverables exist on disk, even if the iteration produced no git progress. (#230)
- Tasks that wrote deliverables to `.wolfcastle/docs/` no longer get stuck in infinite retry-decompose cycles. (#230)
- Default stall timeout raised from 120s to 600s. The previous value was killing Opus sessions that were thinking on large contexts, not genuinely stalled. (#243)
- `wolfcastle scope add` reports failures as errors instead of calling `os.Exit(1)`, so Cobra-managed cleanup runs and exit codes flow normally. (#224)
- Atomic write helpers in `config` and `tierfs` now share `internal/fsutil.AtomicWriteFile`, fixing a missing temp-file cleanup on rename failure. (#224)

### Documentation
- Comprehensive TUI guide (`docs/humans/the-tui.md`): launching, every screen, every keybinding, navigation flows. 16 VHS-generated screenshots with scripted state and retry logic for deterministic captures. (#246, #256)
- Existing concept docs reframed to present TUI as the primary interface, CLI as secondary. Both READMEs updated. (#246)

### Quality
- TUI acceptance test suite: 35 tests exercising a real `tea.Program` headless via `charmbracelet/x/exp/teatest/v2`. Covers welcome screen, dashboard, tree navigation, inbox, help overlay, search, daemon modal, log stream, terminal states, status glyphs, and all key binding categories. Zero flakes across 105 runs. (#247)
- New `internal/fsutil` package with full test coverage for `AtomicWriteFile` (happy path, overwrite, parent creation, permission errors, rename failures). (#225)
- Comprehensive multi-process tests covering two-worktree coexistence, deep-subdirectory CWD routing, prefix-boundary cross-match prevention, `--instance` overriding CWD end-to-end, tier regeneration from a partial scaffold, `stop --all` with mixed live/stale entries, and full coverage for the new `App.InitFromDir` and `resolveInstance` helpers. (#239)
- TUI wiring smoke tests and app-package coverage expanded from 65% to 80%. Overall project coverage at 91%. (#241, #244)
- Daemon test suite trimmed: hardcoded 1-second context timeouts in fifteen tests dropped to 200-300ms, cutting ~5 seconds off every full daemon test run. (#229)

## 0.4.3

### Bug Fixes
- In parallel mode, orphaned `in_progress` tasks (lost worker, stall kill) are now reclaimed at runtime, not just on startup. The dispatcher scans for tasks with no active worker each iteration and resets them to `not_started`. (#201)
- Status screen "Recent:" section no longer shows stale content when intake log files outnumber execution files (#203)

## 0.4.2

### Features
- `wolfcastle stop --drain`: tell a running daemon to finish its current work then exit. No signal sent, no work lost. In parallel mode, active workers finish but no new workers are dispatched. `wolfcastle status` shows "draining" while pending. (#197)

### CLI
- `status --watch` and `--interval` merged into a single `-w` flag that accepts an optional interval (`status -w 0.5`). Default remains 2s.

## 0.4.1

### Bug Fixes
- Orchestrators with all `not_started` children no longer show as `in_progress` after planning (#192)

## 0.4.0

### Bug Fixes
- Process group kill applied unconditionally to both streaming and non-streaming invocations, preventing orphaned child processes (test runners, linters, build tools) from accumulating during long sessions (#177, #178)
- Remediation subtasks inherit the parent task's class, so language-specific guidance (e.g., `coding/go.md`) is available during gap fixes instead of falling back to `coding/default.md` (#183)
- Empty leaf nodes (audit-only, no regular tasks) no longer stuck in `not_started` forever; the navigator checks the parent orchestrator's planning state to determine if the task set is final (#186)
- Blocked siblings with pending remediation tasks are now found regardless of creation order; the DFS orchestrator child loop is split into a creation-order scan (new work) and a remediation scan (blocked children) (#188)

### Features
- `last_activity_at`, `current_iteration`, `current_node`, and `current_task` fields in `wolfcastle status --json` output for external stall detection without filesystem stat tricks (#179)
- Intake stage logs `result.Summary` to both human-readable output and NDJSON, making intake actions visible without parsing raw model output (#180)
- Leaf audit context now includes sibling task deliverables and acceptance criteria, giving the auditor an explicit checklist instead of relying on breadcrumb prose (#184)
- `wolfcastle status` shows the 2 most recent rendered log lines at the bottom, using the same format as foreground mode

### Prompt Improvements
- Audit procedure includes a linter verification step: run the project's linter and record violations as gaps (#185)
- Orchestrator audits verify cross-node integration: shared interfaces, wiring files, dependency injection, and configuration between children (#181)
- Audit procedure reviews codebase knowledge entries for applicable conventions (#182)
- Knowledge maintenance instructs the pruning agent to migrate enforceable conventions out of knowledge files and into class or rule files (#182)

### CLI
- 75 commands (up from 74)

## 0.3.0

### Core
- Parallel sibling execution: when `daemon.parallel.enabled` is true, sibling tasks under the same orchestrator run concurrently with file-level scope locks preventing collisions (ADR-095)
- Parallel dispatcher with worker pool, buffered result channel, and scoped git commits serialized through `gitMu`
- Scope conflict handling: workers that overlap yield with `ErrYieldScopeConflict`, tracked in a blocked map with stale-entry cleanup
- `parallel-status.json` snapshot for `wolfcastle status` to display worker pool state without IPC
- Codebase knowledge files: per-namespace markdown files injected into iteration context for accumulated informal observations
- Positional task address arguments for lifecycle commands (claim, complete, block, unblock)
- Removed deprecated `LoadConfig`, `Invoke`, and `InvokeStreaming` wrappers

### CLI
- 74 commands (up from 53)
- `task scope add`, `task scope list`, `task scope release` for file-level scope locks
- `knowledge add`, `knowledge show`, `knowledge edit`, `knowledge prune` for codebase knowledge management
- `config validate` for configuration validation
- `execute` and `intake` as standalone commands with live interleaved output
- `install skill` for Claude Code skill deployment

### Pipeline
- Knowledge injection in `ContextBuilder`: reads per-namespace knowledge file fresh each iteration
- Scope lock table (`scope-locks.json`) with `ScopeLockTable` and `ScopeLock` types
- `FindParallelTasks` navigation: returns up to `maxCount` actionable sibling tasks under the same orchestrator

### Safety
- Scope path validation: rejects empty, absolute, and `..`/`.` segment paths
- Bidirectional prefix matching for file/directory scope conflicts
- Git commit serialization in parallel mode: only the worker's declared files are staged

### Validation
- 28 validation categories (up from 27)

### Quality
- `internal/knowledge` package for knowledge file management
- `internal/logrender` package for log record rendering (summaries, thoughts, session views)
- 20 internal packages (up from 18)

### Documentation
- 95 Architecture Decision Records (up from 89)
- 44 specs (up from 38)
- 74 CLI commands documented
- Agent guides updated for parallel dispatch flow and scope lock types

## 0.2.0

### Core
- Orchestrator planning pipeline: lazy planning, completion review, AAR action item triage
- Spec review pipeline: auto-review after spec completion, feedback loop on block
- After Action Reviews (AARs) per task: objective, what happened, improvements, action items
- Sequential inbox intake: one item per invocation with tree refresh, preventing duplicate projects (ADR-080)
- Auto-archive: completed nodes move to archive after configurable delay
- Stall detector for model invocations: kills processes that stop producing output
- Context-aware marker scanning: CONTINUE marker excluded during execution (planning-only)
- Orchestrator state reconciliation at iteration start
- Self-heal creates remediation subtasks for blocked audits with open gaps

### CLI
- 53 commands (up from 39)
- `config show` with `--section` filtering
- `config set`, `config unset`, `config append`, `config remove` for tier mutation
- `audit aar` for structured after-action reviews
- `audit report` for consolidated audit reports
- `status --detail` for task bodies, deliverables, breadcrumbs
- `status --archived` and `status --all` for archive visibility
- Node type prefixes in status (Proj/Orch/Leaf)
- Contextual error messages replacing generic Cobra argument errors
- Shell completions for `archive delete` and `archive restore`
- `output.Plural` helper for proper singular/plural formatting

### Pipeline
- Stages refactored from array to dict (`map[string]PipelineStage` + `StageOrder`)
- TTL-based caching in tierfs (CachingResolver, 30s default)
- Unknown-field detection in config files
- Task classes: `ClassDef` config type, `ClassRepository` with hierarchical fallback, `--class` CLI flag
- Planning prompts: `--description` required, spec-first ordering, direct children only, AAR triage in completion review

### Safety
- Atomic write for global daemon lock (temp+rename, matching state file pattern)
- Task ID format validation in `autoCommitPartialWork`
- `SilenceUsage` on root cobra command (prevents usage/JSON interleaving)
- Force-exit log message in signal handler
- Subprocess environment inheritance documented in SECURITY.md

### Validation
- 27 validation categories (up from 24): `CHILDREF_STATE_MISMATCH`, `ORPHANED_TEMP_FILE`, `INVALID_TASK_ID`
- Doctor detects parent-child state divergence and fixes it deterministically

### Quality
- `state.StateStore` renamed to `state.Store` (Go naming convention)
- Git operations consolidated into `internal/git.Service` (daemon no longer duplicates)
- Domain repository architecture: 7 domain-specific repositories replacing raw filepath.Join I/O
- Test-to-source ratio: 3.4:1 (72,800 test LOC, 21,400 source LOC)
- Coverage: 94.9%

### Documentation
- README rewrite: thesis framing (four problems, Ralph loops, determinism principle)
- Architecture diagram (Mermaid) in CONTRIBUTING.md
- 89 Architecture Decision Records (up from 76)
- 38 specs (up from 20), all current per currency audit
- Task classes spec accepted (updated from DRAFT)
- All 53 CLI commands documented in `docs/humans/cli/`

## 0.1.0

Initial release.

### Core
- Model-agnostic autonomous coding orchestrator
- Four-state lifecycle (not_started, in_progress, complete, blocked) with upward propagation
- StateStore with lock-protected atomic mutations and auto-propagation
- Two-goroutine daemon: execute loop + inbox watcher (fsnotify with polling fallback)
- Supervisor with crash recovery and configurable restarts
- Self-healing on startup (detects interrupted tasks)
- Discovery-first intake pipeline with pre-blocking for infeasible work

### CLI
- 39 commands across lifecycle, work management, auditing, documentation, diagnostics
- Tree view status with colored glyphs and --watch mode
- Log command with --follow streaming and level filtering
- Interactive unblock sessions with readline support
- Doctor with 24 validation categories and multi-pass deterministic fixing
- JSON output envelope on every command

### Pipeline
- Three-tier config (base, custom, local) with deep merge and null deletion
- Prompt assembly: rule fragments, filtered script reference, stage prompt, iteration context
- Deliverable verification with SHA-256 baseline change detection
- Terminal markers (COMPLETE, YIELD, BLOCKED) with priority-ordered scanning

### Safety
- Atomic writes (temp file + fsync + rename) for all state files
- File locking with stale lock detection
- Terminal restoration after child process exit (ISIG/ICANON/ECHO)
- Three-layer signal handling (NotifyContext + backup channel + spinner timeout)
- Deliverable path traversal validation
- Address validation prevents path traversal in node names

### Documentation
- 76 Architecture Decision Records
- 17 implemented specs, 3 draft specs (TUI, worktree, task classes)
- Human-facing docs for every command
- Agent-facing guides for code modification
- CONTRIBUTING.md, CODE_OF_CONDUCT.md, issue templates
