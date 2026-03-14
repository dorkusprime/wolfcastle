# Wolfcastle Agent Guide

Wolfcastle is Ralph on steroids. It's what happens when you give an action hero a task backlog and tell them not to come back until the job is done. Deterministic by design: state is JSON on disk and mutations go through compiled scripts with the soul of a 90s sysadmin but with none of the typos. Models only get called when something actually needs thinking.

## Quick Orientation

- **Language:** Go 1.26+, single module `github.com/dorkusprime/wolfcastle`
- **Framework:** [Cobra](https://github.com/spf13/cobra) for CLI
- **Dependencies:** Minimal — Cobra/pflag + chzyer/readline (ADR-048)
- **Build:** `make build` / `go build ./...`
- **Test:** `make test` / `go test ./...`
- **Lint:** `make lint` (runs `go vet` + `gofmt`), `golangci-lint run` (full lint suite per ADR-049)

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
| [Audit](docs/agents/AUDIT.md) | Running a comprehensive codebase audit (correctness, style, security, docs, coverage, usability) |

## Design References

- [Architecture Decision Records](docs/decisions/INDEX.md) (61 ADRs) document every major design choice. Consult these before making architectural decisions.
- [Specifications](docs/specs/) (14 specs) describe the current system in detail. Consult these before modifying behavior.
- [Full documentation hub](docs/)

## Critical Rules

1. **All output through `output` package.** Never use `fmt.Println`/`fmt.Printf` for user-facing output in commands or the daemon. Use `output.PrintHuman()` and `output.PrintError()`.
2. **Status constants, not string literals.** Use `state.GapOpen`, `state.GapFixed`, `state.EscalationOpen`, `state.EscalationResolved` — never raw `"open"`, `"fixed"`, `"resolved"` in source (test files may use literals).
3. **Explicit error handling.** Never ignore `os.Remove()` or other cleanup errors — use `_ = os.Remove()` to mark intentional ignores.
4. **gofmt before committing.** Run `gofmt -w .` — the CI will reject unformatted code.
5. **Specs track implementation, not aspirations.** If you change behavior, update the corresponding spec. ADRs override specs when there's a conflict.
6. **Never rebase main.** Use `git pull` (merge), not `git pull --rebase`. Rebasing rewrites commit SHAs, which breaks Codecov and any other service that tracks by commit hash.
