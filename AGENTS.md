# Wolfcastle Agent Guide

Wolfcastle is Ralph on steroids. It's what happens when you give an action hero a task backlog and tell them not to come back until the job is done. Deterministic by design: state is JSON on disk and mutations go through compiled scripts with the soul of a 90s sysadmin but with none of the typos. Models only get called when something actually needs thinking.

## Quick Orientation

- **Language:** Go 1.26+, single module `github.com/dorkusprime/wolfcastle`
- **Framework:** [Cobra](https://github.com/spf13/cobra) for CLI
- **Dependencies:** Minimal: Cobra/pflag + chzyer/readline + fsnotify/fsnotify (ADR-048)
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
| [Testing](docs/agents/testing.md) | Writing or debugging daemon integration tests, mock model configuration |
| [Audit](docs/agents/AUDIT.md) | Running a full codebase audit (correctness, style, security, docs, coverage, usability) |

## Codebase Knowledge Files

Each iteration, the daemon injects a codebase knowledge file into your context (after class guidance, before AARs). These are accumulated observations about the codebase: build quirks, undocumented conventions, things that look wrong but are intentional, hidden dependencies between modules. They live in `.wolfcastle/docs/knowledge/` as markdown files, one per engineer namespace.

When you discover something non-obvious about the codebase that isn't captured in the README, specs, or ADRs, record it:

```
wolfcastle knowledge add "the integration tests require docker compose up before running"
```

Entries should be concrete, durable, and non-obvious. "The config loader silently drops null values" is a good entry. "I'm working on the auth module" is not. The knowledge file has a token budget (`knowledge.max_tokens`); if the file exceeds it, `knowledge add` will fail and prompt you to prune.

## Design References

- [Architecture Decision Records](docs/decisions/INDEX.md) (89 ADRs) document every major design choice. Consult these before making architectural decisions.
- [Specifications](docs/specs/) (38 specs) describe the current system in detail. Consult these before modifying behavior.
- [Full documentation hub](docs/)

## Critical Rules

1. **All output through `output` package.** Never use `fmt.Println`/`fmt.Printf` for user-facing output in commands or the daemon. Use `output.PrintHuman()` and `output.PrintError()`.
2. **Status constants, not string literals.** Use `state.GapOpen`, `state.GapFixed`, `state.EscalationOpen`, `state.EscalationResolved`. Never raw `"open"`, `"fixed"`, `"resolved"` in source (test files may use literals).
3. **Explicit error handling.** Never ignore `os.Remove()` or other cleanup errors. Use `_ = os.Remove()` to mark intentional ignores.
4. **gofmt before committing.** Run `gofmt -w .`. The CI will reject unformatted code.
5. **Specs track implementation, not aspirations.** If you change behavior, update the corresponding spec. ADRs override specs when there's a conflict.
6. **Do not run git commands.** The daemon handles all git operations (add, commit, push). Agents must never run git commands directly.
7. **`.wolfcastle/system/` is off-limits (ADR-077).** Never write directly to `.wolfcastle/system/`. That directory contains config, state, logs, and prompts managed by the scaffold and daemon. Write model outputs to `.wolfcastle/docs/` (specs, ADRs) and `.wolfcastle/artifacts/` (research) only. Configuration is Go code (`internal/config/`), not JSON files.
8. **Planning is lazy, archive is lazier.** The daemon executes first (Step 1), plans only when no task is found (Step 2), checks for auto-archive-eligible nodes only when both execute and plan find nothing (Step 3), and idles only when all three fail (Step 4). Orchestrators get planned right before their subtree needs work, not before.
9. **selfHeal derives parents.** On startup, `selfHeal` resets stale in_progress tasks and derives parent task status from children for any parent whose state disagrees with what its children say.
10. **Pre-start repair before validation.** The start command runs `FixWithVerification` (multi-pass deterministic repair) before the startup validation gate. It omits `wolfcastleDir` to skip daemon artifact checks, since PID/stop files are expected at startup.
