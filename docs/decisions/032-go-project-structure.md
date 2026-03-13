# ADR-032: Go Project Structure and Cobra CLI Framework

## Status
Accepted

## Date
2026-03-13

## Context
Wolfcastle is a Go binary (ADR-002) that needs a well-organized project structure and a CLI framework capable of handling 25+ commands with subcommands, flag parsing, dynamic help, and shell completions.

## Decision

### Project Layout
Standard Go project structure with `cmd/` for cobra commands and `internal/` for private packages:

```
main.go                    # Entry point, calls cmd.Execute()
cmd/                       # Cobra command definitions (one file per command)
  root.go                  # Root command, config loading, global flags
  task.go                  # Parent command for task subcommands
  task_add.go              # wolfcastle task add
  ...
internal/                  # Private packages (not importable externally)
  config/                  # Config loading, merge, validation
  state/                   # State types, I/O, mutations, propagation, navigation
  tree/                    # Tree addressing, slug validation, resolution
  project/                 # Scaffolding, project creation, embedded templates
  pipeline/                # Fragment resolution, prompt assembly, context
  invoke/                  # CLI shell-out, streaming, retry
  daemon/                  # Loop, signals, PID, branch verification
  logging/                 # NDJSON per-iteration files, retention
  archive/                 # Markdown rollup generation
  inbox/                   # Inbox types and I/O
  output/                  # JSON envelope, formatters
```

### Cobra CLI Framework
Cobra (`github.com/spf13/cobra`) is used for the entire CLI surface because:
- Native subcommand tree (`wolfcastle task add`, `wolfcastle audit breadcrumb`)
- `PersistentPreRunE` on the root command loads config before any subcommand runs
- Commands that don't need config (`init`, `version`) skip loading in the pre-run hook
- Built-in help generation with command grouping
- `RegisterFlagCompletionFunc` enables context-aware shell completions for `--node` flags
- Auto-generates shell completion scripts (`wolfcastle completion bash/zsh/fish`)

### Global Flags
The root command defines `--json` as a persistent flag, available to all subcommands for structured JSON output.

### Minimal Dependencies
Only one external dependency: `github.com/spf13/cobra` (which brings `pflag`). Everything else uses the Go standard library.

## Consequences
- Clear separation: `cmd/` is thin (flag parsing, output formatting), `internal/` has all logic
- Adding a command is one file in `cmd/` — cobra handles registration and help
- `internal/` packages can't be imported by external code, enforcing encapsulation
- Shell completions work out of the box for node addresses via the root index
- Single external dependency minimizes supply chain risk
