# Changelog

## 0.2.0 (unreleased)

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
