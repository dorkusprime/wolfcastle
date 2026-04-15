# Changelog

## 0.6.8

### Features
- **Dedicated `ParentLogger` for daemon-lifetime events.** A third `*logging.Logger` opens at `Run()` entry with a `daemon` prefix and stays open until the daemon exits. Parent-loop events — `drainCompleted` diagnostics, auto-archive decisions, spec-review hooks, knowledge maintenance, instance-registry warnings, shutdown-signal lifecycle — route through this logger instead of reaching for `d.Logger`, which in parallel mode has no active file between planning passes. File name is `2NNNN-daemon-TIMESTAMP.jsonl` (iteration counter offset by 20000 so it can't collide with exec/plan or inbox namespaces).
- **Nil-safe `Logger.Log` receiver.** Calling `.Log(...)` on a nil `*logging.Logger` now returns an error and increments the dropped-records counter instead of panicking. Tests that construct `Daemon` directly without calling `New()` — and any future caller that holds a nil reference through a free function — no longer need defensive guards.

### Bug Fixes
- **Canary-caught silent drops closed up across the parent loop.** The v0.6.6 plumbing fixed `runIteration` itself, but several helpers and lifecycle paths were still calling `d.Logger.Log` (or `d.log`) on an iteration-scoped logger that had no active file. This release routes them through the always-live parent logger:
  - `parallel.go` — every `drainCompleted`, `fillSlots`, `reclaimOrphans` diagnostic, plus the three `commitAfterIteration` invocations (success, failure, scope-conflict) and the `propagateState` fallback diagnostic.
  - `archive.go` — `auto_archive_failed` and `archived` events fired from `tryAutoArchive` in the main loop.
  - `spec_review.go` — `review_queued` and `review_failed` events fired from the post-iteration hook.
  - `knowledge_maintenance.go` — `budget_exceeded` events.
  - `daemon.go` — the shutdown-signal goroutine, the 2-second force-exit grace message, the instance-registry warning, and the three `task_event` sites (serial planning error, serial iteration error, parallel planning error).
  - `iteration.go` — six `d.log(task_event ...)` calls inside `handleBlockedMarker` / `handleCompleteMarker` / `handleFailure` that the v0.6.6 sweep missed because they used the nil-safe wrapper instead of `d.Logger.Log` directly. Now route through the `lg` parameter alongside every other logger call in those methods.
- **Worker trace IDs no longer duplicate the task number.** The previous format stamped `worker-<node>-task-0001-0001` — the trailing `0001` was the Child logger's iteration counter, which is always `0001` because each worker runs exactly one iteration. Dropped the redundant suffix; trace IDs read `worker-cart-and-promo-domain-task-0001` now.

### Quality
- **`DroppedRecords()` and the silent-drop canary on stderr** are now test-covered. New `internal/logging/dropped_records_test.go` pins the counter behavior: zero on the happy path, one on a nil receiver (no panic), one on a missing iteration, two after a close-then-log sequence.

## 0.6.7

### Features
- **Dirty-tree start now confirms through a TUI modal** instead of silently refusing. `wolfcastle start` gains an `--allow-dirty` flag that skips the interactive y/N prompt and proceeds with a warning. The TUI detects "commit or stash changes first" in daemon-start stderr, pops an `UNCOMMITTED CHANGES` confirmation modal showing which files would be swept into the first commit, and on Enter re-invokes `start -d --allow-dirty`. Previously the TUI's `s` key ran `start -d`, which inherits no TTY and so returned an EOF to `confirmContinue()`, aborting with no visible error — the daemon would just fail to come up and the tab would go cold without explanation.

## 0.6.6

### Features
- **Knowledge pipeline: orchestrator audit findings become persistent checks automatically.** Phase D of `plan-review.md` now instructs the model to emit one or more `WOLFCASTLE_KNOWLEDGE: <pattern>` marker lines per finding. The daemon parses those markers after a `completion_review` pass, checks the token budget, and appends each entry to the project's knowledge file through a structured channel — no more relying on the model to remember to shell out to `wolfcastle knowledge add`. Individual failures (budget exceeded, write error) surface as `knowledge_add_error` events but never break the planning pass, so the worst case is a lost entry rather than a lost review. The scanner handles raw markers, Claude Code stream-json envelopes, markdown decoration (backticks, asterisks, underscores), and the trailing colon isolates knowledge markers from terminal markers like WOLFCASTLE_COMPLETE. The audit overhaul spec (`docs/specs/2026-04-12T00-00Z-audit-overhaul.md`) is marked Accepted now that this channel exists.

### Bug Fixes
- **Parallel mode: worker execution content is captured on disk.** `runIteration` and every helper it calls (`handleYieldMarker`, `handleBlockedMarker`, `handleCompleteMarker`, `handleFailure`, `autoCompleteDecomposedParents`, `createRemediationSubtasks`, `maybeWriteAuditReport`, `invokeWithRetry`, `propagateState`) now take an explicit `*logging.Logger`. Sequential callers pass `d.Logger`; parallel workers pass their own child logger. Before this change, `d.Logger` had no active file during worker invocations, so every `d.Logger.Log(...)` call inside `runIteration` hit the "no active iteration" guard and was silently dropped. Parallel daemon runs produced 170-byte stub files containing one `iteration_start` record and nothing else; the TUI's log modal was dark.
- **Worker log filenames are unique per task address.** The child logger's prefix now includes the slugified node path (`worker-<slug(nodeAddr)>-<taskID>`), so three concurrent workers with the same `task-0001` on different leaves write to three distinct files instead of stomping the same `0001-worker-task-0001-{ts}.jsonl` inode.
- **Retention respects active workers.** `EnforceRetention` now sorts log files by mtime (not by filename) and applies a 30-second quiet window to compression, count-delete, and age-delete. Previously, worker filenames starting with `0001-` sorted alphabetically before parent daemon files (`0519-heal`, `10497-intake`), so count-based retention treated brand-new worker files as the oldest and deleted them first. And even before that, compression was unlinking files while sibling workers still held the inode open.
- **Silent-drop canary.** `logging.Logger.Log` now writes a diagnostic line to stderr and increments a global counter when called without an active iteration file. Exposed as `logging.DroppedRecords()`. A healthy daemon should always return 0; any non-zero value means a code path is logging to an uninitialized Logger (the class of bug that hid the parallel logger problem for months behind `_ =` error discards).

### Quality
- Worker trace IDs are compact: `worker-<leaf-basename>-<taskID>-<iter>` instead of the full slugified node path. The filename stays long for uniqueness, but the log view's `[trace]` column is readable again.
- The TUI log modal's trace filter cycle gains a `worker` category so you can isolate parallel worker output with `T`. `traceCategory` recognizes `worker-*` prefixes.

## 0.6.5

### Bug Fixes
- Welcome screen: pressing Enter on a subdirectory that already contains `.wolfcastle` now opens a session there instead of descending into its filesystem. Enter on the `.wolfcastle` entry itself opens the current directory as a project. Both paths route through the existing init-is-a-no-op branch, so the app/tab wiring stays in one place.
- Project directories in the welcome browser are badged with a gold `◆` and their selected-row hint reads `[Enter to open]`, so a fresh-start user browsing the filesystem can spot and open existing projects without guessing.

## 0.6.4

### Features
- Task detail pane renders description, body, deliverables, acceptance criteria, and constraints through glamour, vendorized. File paths and other meta characters are escaped so `file_a.go` stays literal instead of triggering italic emphasis. Glamour's dark style sets the tone; document margins are accounted for in the wrap width so content never overflows the pane.
- Node detail pane shows gap and escalation details inline under the audit section. Each entry carries a timestamp, ID, source, and a full wrapped description. Audit `result_summary` now wraps cleanly under its own label instead of running off the right edge.
- Log modal enriches tool activity: `[tool: Bash] command…`, `[tool: Read] /path`, `[tool: Grep] pattern`, and so on. Tool result blocks surface a truncated preview of their content instead of a bare `[tool result]` marker.

### Bug Fixes
- `wolfcastle task add` and `wolfcastle task amend` switched from pflag's `StringSlice` to `StringArray` for `--deliverable`, `--constraint`, `--acceptance`, `--reference`, and their `--add-*` counterparts. `StringSlice` silently shreds any value containing a comma, so a single acceptance criterion listing "name, slug, description" ended up as three bullets with leading spaces. Newly ingested tasks keep their fields intact; existing state.json files are not self-healed.
- Long task titles and audit breadcrumb text in node detail are now broken onto their own indented lines with `wrapIndent` instead of being emitted as raw one-shot `fmt.Sprintf` rows.
- Empty pad lines around glamour blocks are stripped so each section butts flush against its heading instead of floating in a vertical moat.

## 0.6.3

### Features
- TUI design system overhaul. Color palette derived from the neon-wolf logo: neon cyan primary, gold targets, deep teal header, magenta accents. Gradient "WOLFCASTLE" title cycles cyan through purple and magenta to gold. (#274)
- Near-black base background (ANSI 234) fills the entire alt-screen via cell-level canvas, making the TUI readable on light terminals without breaking transparency on dark ones. (#274)
- Help overlay restyled: section titles in neon cyan, key hints in gold. (#274)
- Toast notifications use neon cyan borders instead of red. (#274)
- Search results use gold/dark-gold foreground tinting instead of background highlighting. (#274)

### Documentation
- Design system reference at `docs/agents/design-system.md`: full palette, component specs, principles, Terminal.app limitations. (#274)

## 0.6.2

### Features
- Per-directory workspace tabs. Each tab owns its own tree, detail pane, search, daemon lifecycle, and watcher. Press `+` to open a directory picker, `-` to close a tab, `<`/`>` to switch. The header renders a tab bar when multiple tabs are open. All directory-dependent state lives on the Tab struct, eliminating the class of bugs where context switches left stale fields on TUIModel.
- Directory picker overlay (`+`): shows running daemon sessions at the top, followed by a filesystem browser. Directories with `.wolfcastle/` are highlighted. Selecting a running session opens a tab connected to that daemon. Duplicate directories refocus the existing tab.
- Tab-scoped daemon control: `s` starts/stops the daemon in the active tab's directory, not the CWD. The daemon modal shows which directory it will affect.

### Bug Fixes
- Log modal now streams ALL log types (exec, plan, intake, inbox), not just exec. The watcher tracks every uncompressed `.jsonl` file in the log directory.
- Trace filter (`T` in log modal) matches by category prefix instead of exact trace ID. Expanded cycle: all, exec, plan, inbox, system.
- Modal overlays (inbox, logs, daemon confirm, new tab picker) have solid dark backgrounds. Cell-level background fill via lipgloss Canvas prevents ANSI resets from punching transparent holes.

### Breaking Changes
- Instance tab bar replaced by user-managed tabs. The `<`/`>` keys now switch tabs instead of instances. Digit keys (1-9) for instance selection removed.

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
