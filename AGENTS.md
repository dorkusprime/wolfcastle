# Wolfcastle Agent Guide

Wolfcastle is a model-agnostic autonomous project orchestrator — a Go CLI that breaks complex work into a persistent tree of projects, sub-projects, and tasks, then executes them through configurable multi-model pipelines.

## Quick Orientation

- **Language:** Go 1.26+, single module `github.com/dorkusprime/wolfcastle`
- **Framework:** [Cobra](https://github.com/spf13/cobra) for CLI
- **Dependencies:** Minimal — only Cobra/pflag
- **Build:** `make build` / `go build ./...`
- **Test:** `make test` / `go test ./...`
- **Lint:** `make lint` (runs `go vet` + `gofmt`)

## Detailed Guides

Consult these topic-specific files before making changes in their domain:

| Guide | When to read |
|-------|-------------|
| [Architecture](docs/agents/architecture.md) | Modifying package structure, adding packages, understanding data flow |
| [Code Standards](docs/agents/code-standards.md) | Writing or reviewing any Go code |
| [Commands](docs/agents/commands.md) | Adding or modifying CLI commands |
| [Daemon](docs/agents/daemon.md) | Touching daemon loop, pipeline execution, or model invocation |
| [State & Types](docs/agents/state-and-types.md) | Modifying state files, types, or propagation logic |
| [Documentation](docs/agents/documentation.md) | Writing specs, ADRs, or updating existing docs |
| [Voice](docs/agents/VOICE.md) | Writing user-facing copy, error messages, README text, or any prose that represents Wolfcastle's personality |

## Critical Rules

1. **All output through `output` package.** Never use `fmt.Println`/`fmt.Printf` for user-facing output in commands or the daemon. Use `output.PrintHuman()` and `output.PrintError()`.
2. **Status constants, not string literals.** Use `state.GapOpen`, `state.GapFixed`, `state.EscalationOpen`, `state.EscalationResolved` — never raw `"open"`, `"fixed"`, `"resolved"` in source (test files may use literals).
3. **Explicit error handling.** Never ignore `os.Remove()` or other cleanup errors — use `_ = os.Remove()` to mark intentional ignores.
4. **gofmt before committing.** Run `gofmt -w .` — the CI will reject unformatted code.
5. **Specs track implementation, not aspirations.** If you change behavior, update the corresponding spec. ADRs override specs when there's a conflict.
