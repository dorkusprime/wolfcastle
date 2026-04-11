# Architecture

## Module Layout

```
wolfcastle/
├── main.go                  # Minimal: calls cmd.Execute()
├── cmd/                     # CLI command layer (Cobra)
│   ├── root.go              # Root command, PersistentPreRunE for config loading
│   ├── cmdutil/             # Shared App context, completions, overlap detection
│   ├── audit/               # Audit subcommands (approve, reject, gap, aar, etc.)
│   ├── config/              # Config subcommands (show, set, unset, append, remove)
│   ├── daemon/              # start, stop, status, log
│   ├── inbox/               # add, list, clear
│   ├── knowledge/           # add, show, edit, prune
│   ├── project/             # create
│   ├── orchestrator/        # criteria (planning pipeline support)
│   ├── task/                # add, amend, claim, complete, block, unblock, deliverable
├── internal/                # Core logic (not importable outside the module)
│   ├── archive/             # Archive entry rollup (Markdown generation)
│   ├── clock/               # Time abstraction for deterministic testing (ADR-052)
│   ├── config/              # Config loading, merging, validation, types
│   ├── daemon/              # Daemon loop, pipeline execution, marker parsing
│   ├── errors/              # Typed error categories (ADR-065)
│   ├── fsutil/              # Filesystem helpers (atomic writes, path resolution)
│   ├── git/                 # Git operations behind Provider interface
│   ├── instance/            # Per-worktree instance registry and routing (ADR-099, ADR-100)
│   ├── invoke/              # Model CLI invocation (buffered + streaming)
│   ├── knowledge/           # Per-namespace codebase knowledge files
│   ├── logging/             # Per-iteration NDJSON logging
│   ├── logrender/           # Log record rendering (summaries, thoughts, session views)
│   ├── output/              # Structured JSON envelopes + human-readable printing
│   ├── pipeline/            # Prompt assembly, iteration context, fragment resolution
│   ├── project/             # Project scaffolding and embedded templates
│   ├── selfupdate/          # Binary self-update logic
│   ├── signals/             # Canonical shutdown signal set (platform-specific via build tags)
│   ├── state/               # State types, I/O, mutations, navigation, propagation, inbox, review (ADR-058)
│   ├── testutil/            # Shared test helpers
│   ├── tierfs/              # Three-tier file resolution (ADR-063)
│   ├── tree/                # Tree addressing, slug generation, resolver
│   ├── tui/                 # TUI application (Bubbletea v2 models and views)
│   └── validate/            # Structural validation engine and auto-fix
├── docs/
│   ├── decisions/           # ADRs (001-101)
│   ├── specs/               # Implementation specs (timestamped)
│   └── agents/              # This directory (agent guidance)
└── Makefile
```

## Orchestrator Planning Pipeline

The daemon uses lazy, recursive planning for orchestrator nodes. Rather than decomposing the entire project tree up front during intake, each orchestrator is planned on demand, right before its subtree needs work. The full design is in [docs/specs/2026-03-17T00-00Z-orchestrator-planning-pipeline.md](../specs/2026-03-17T00-00Z-orchestrator-planning-pipeline.md).

## Execution Pipeline and Side Flows

The daemon's `StageOrder` contains two stages: **intake** (inbox processing, runs in a parallel goroutine per ADR-064) and **execute** (task work, runs sequentially in the main loop). Several mechanisms operate alongside or within the execute stage:

**After Action Reviews (AARs):** Structured post-task narratives recorded via `wolfcastle audit aar`. Stored as `AAR` structs in per-node `state.json` (keyed by task ID). AARs feed into the audit context for subsequent tasks, giving the next agent a view of what the previous agent did, what went well, and what remains uncertain.

**Spec Review:** When a task with `TaskType == "spec"` completes, the daemon auto-creates a sibling review task (`<taskID>-review`) of type `spec-review`. This blocks further work until the review passes. If the review is blocked (i.e., rejected), its feedback resets the original spec task to `not_started` for revision, creating an iterative review loop. Implemented in `internal/daemon/spec_review.go`.

**Auto-Archive:** Completed nodes are archived inline during `RunOnce`, throttled to a 5-minute poll interval. The daemon imports `internal/archive` directly for this. See ADR on auto-archive running inline rather than as a separate goroutine.

## Key Design Principles

- **cmd/ is thin.** Commands parse flags, call into `internal/`, and format output. No business logic lives in `cmd/`.
- **internal/ owns all logic.** Packages are organized by domain, not by layer.
- **Shared state flows through `cmdutil.App`.** The `App` struct holds config, resolver, and JSON output flag. It's passed to all subcommand `Register()` functions.
- **Atomic file operations.** All state writes use temp-file-then-rename via `atomicWriteJSON()`.
- **Three-tier file layering.** Config, prompts, rules, and audit scopes merge across `base/`, `custom/`, and `local/` tiers (ADR-009, ADR-018).

## Data Flow

```
User input → cmd/ → internal/ → filesystem (.wolfcastle/)
                                    ├── system/                    (ADR-077)
                                    │   ├── base/config.json       (defaults)
                                    │   ├── custom/config.json     (team overrides)
                                    │   ├── local/config.json      (personal overrides)
                                    │   ├── projects/{namespace}/
                                    │   │   ├── state.json         (root index)
                                    │   │   └── {node}/state.json  (per-node)
                                    │   └── logs/                  (NDJSON per-iteration)
                                    ├── archive/                   (completed work)
                                    └── docs/                      (ADRs, specs)
```

## Package Dependencies

Dependencies flow strictly downward. `cmd/` imports `internal/`, but `internal/` packages never import `cmd/`. Within `internal/`, the dependency graph is:

- `tui` → `config`, `daemon`, `instance`, `logging`, `logrender`, `pipeline`, `project`, `state`
- `daemon` → `archive`, `clock`, `config`, `errors`, `git`, `instance`, `invoke`, `knowledge`, `logging`, `output`, `pipeline`, `signals`, `state`, `tree`
- `validate` → `config`, `invoke`, `pipeline`, `state`, `tree`
- `pipeline` → `config`, `invoke`, `knowledge`, `state`, `tierfs`
- `archive` → `clock`, `config`, `state`
- `project` → `config`, `state`, `tierfs`
- `config` → `tierfs`
- `state` → `clock`, `invoke` (includes inbox and review types per ADR-058)
- `logging` → `output`
- `invoke` → `config`
- `tree` → `config`
- `knowledge` → (standalone)
- `logrender` → (standalone)
- `selfupdate` → (standalone)
- `clock` → (standalone)
- `git` → (standalone)
- `tierfs` → (standalone)
- `signals` → (standalone)
- `output` → (standalone)
- `instance` → `config` (ADR-099, ADR-100)
- `fsutil` → (standalone)
- `errors` → (standalone)
