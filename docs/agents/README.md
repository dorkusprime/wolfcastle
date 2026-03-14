# Agent Guides

Technical reference for AI agents and developers working on the Wolfcastle codebase. Each guide covers a specific domain. Consult the relevant guide before making changes in its area.

The root-level [AGENTS.md](../../AGENTS.md) provides quick orientation (language, build, test, lint) and critical rules. These guides go deeper.

## Guides

**[Architecture](architecture.md)** describes the package structure, dependency graph, and data flow. Read this when modifying package boundaries, adding packages, or understanding how components connect.

**[Code Standards](code-standards.md)** covers Go conventions, error handling patterns, naming, testing, and the linting policy. Read this when writing or reviewing any Go code.

**[Commands](commands.md)** explains how CLI commands are structured, how to add new ones, flag conventions, output formatting, and the Register pattern. Read this when adding or modifying commands.

**[Daemon](daemon.md)** covers the daemon loop, pipeline stage execution, model invocation, marker parsing, retry logic, and signal handling. Read this when touching anything in `internal/daemon/` or `internal/invoke/`.

**[State & Types](state-and-types.md)** describes the state machine, node types, propagation rules, distributed state layout, and file locking. Read this when modifying state files, types, or propagation logic.

**[Documentation](documentation.md)** explains how to write specs, ADRs, and update existing docs. Covers naming conventions, the ADR format, and the relationship between specs and ADRs.

**[Voice](VOICE.md)** defines how Wolfcastle talks. Read this when writing user-facing copy, error messages, README text, or any prose that represents the product's personality.

**[Audit](AUDIT.md)** is a 12-section structured checklist for comprehensive codebase audits. Covers correctness, Go best practices, error handling, security, architecture, documentation, voice, testing, CI/CD, cross-platform, code coverage, and usability. Read this when running a full audit or reviewing a major change.
