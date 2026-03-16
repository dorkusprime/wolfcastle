# Changelog

## 0.1.0 (unreleased)

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
