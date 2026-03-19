# Architecture

## Module Layout

```
wolfcastle/
├── main.go                  # Minimal: calls cmd.Execute()
├── cmd/                     # CLI command layer (Cobra)
│   ├── root.go              # Root command, PersistentPreRunE for config loading
│   ├── cmdutil/             # Shared App context, completions, overlap detection
│   ├── audit/               # Audit subcommands (approve, reject, gap, etc.)
│   ├── daemon/              # start, stop, status, log
│   ├── inbox/               # add, list, clear
│   ├── project/             # create
│   └── task/                # add, claim, complete, block, unblock, deliverable
├── internal/                # Core logic (not importable outside the module)
│   ├── archive/             # Archive entry rollup (Markdown generation)
│   ├── clock/               # Time abstraction for deterministic testing (ADR-052)
│   ├── config/              # Config loading, merging, validation, types
│   ├── daemon/              # Daemon loop, pipeline execution, marker parsing
│   ├── errors/              # Typed error categories (ADR-065)
│   ├── invoke/              # Model CLI invocation (buffered + streaming)
│   ├── logging/             # Per-iteration NDJSON logging
│   ├── output/              # Structured JSON envelopes + human-readable printing
│   ├── pipeline/            # Prompt assembly, iteration context, fragment resolution
│   ├── project/             # Project scaffolding and embedded templates
│   ├── selfupdate/          # Binary self-update logic
│   ├── state/               # State types, I/O, mutations, navigation, propagation, inbox, review (ADR-058)
│   ├── testutil/            # Shared test helpers
│   ├── tree/                # Tree addressing, slug generation, resolver
│   └── validate/            # Structural validation engine and auto-fix
├── docs/
│   ├── decisions/           # ADRs (001-076)
│   ├── specs/               # Implementation specs (timestamped)
│   └── agents/              # This directory (agent guidance)
└── Makefile
```

## Orchestrator Planning Pipeline

The daemon uses lazy, recursive planning for orchestrator nodes. Rather than decomposing the entire project tree up front during intake, each orchestrator is planned on demand, right before its subtree needs work. The full design is in [docs/specs/2026-03-17T00-00Z-orchestrator-planning-pipeline.md](../specs/2026-03-17T00-00Z-orchestrator-planning-pipeline.md) (draft).

## Key Design Principles

- **cmd/ is thin.** Commands parse flags, call into `internal/`, and format output. No business logic lives in `cmd/`.
- **internal/ owns all logic.** Packages are organized by domain, not by layer.
- **Shared state flows through `cmdutil.App`.** The `App` struct holds config, resolver, and JSON output flag. It's passed to all subcommand `Register()` functions.
- **Atomic file operations.** All state writes use temp-file-then-rename via `atomicWriteJSON()`.
- **Three-tier file layering.** Config, prompts, rules, and audit scopes merge across `base/`, `custom/`, and `local/` tiers (ADR-009, ADR-018).

## Data Flow

```
User input → cmd/ → internal/ → filesystem (.wolfcastle/)
                                    ├── base/config.json (defaults)
                                    ├── custom/config.json (team overrides)
                                    ├── local/config.json (personal overrides)
                                    ├── projects/{namespace}/
                                    │   ├── state.json (root index)
                                    │   └── {node}/state.json (per-node)
                                    ├── archive/ (completed work)
                                    ├── logs/ (NDJSON per-iteration)
                                    ├── base/ (Wolfcastle-managed)
                                    ├── custom/ (team-shared)
                                    └── local/ (personal, gitignored)
```

## Package Dependencies

Dependencies flow strictly downward. `cmd/` imports `internal/`, but `internal/` packages never import `cmd/`. Within `internal/`, the dependency graph is:

- `daemon` → `config`, `errors`, `invoke`, `logging`, `output`, `pipeline`, `state`, `tree`
- `pipeline` → `config`, `state`
- `validate` → `state`
- `archive` → `config`, `state`
- `project` → `state`, `tree`
- `selfupdate` → (standalone)
- `config` → (standalone)
- `state` → (standalone, includes inbox and review types per ADR-058)
- `clock` → (standalone)
- `tree` → (standalone)
- `output` → (standalone)
- `errors` → (standalone)
- `invoke` → `config`
