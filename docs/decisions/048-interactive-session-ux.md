# ADR-048: Interactive Session UX

**Status:** Accepted

**Date:** 2026-03-14

## Context

The interactive unblock session (ADR-028, Tier 2) uses a raw bufio.Scanner
loop with `fmt.Print("\n> ")` as the prompt. There's no readline support, no
command history, no line editing (arrow keys produce escape sequences), and no
graceful handling of terminal resize. For users accustomed to any modern REPL,
shell, or chat interface, this feels broken.

## Decision

Replace the raw scanner loop with the `github.com/chzyer/readline` library
(BSD license, no CGO, widely used):

1. **Features enabled.** Line editing (arrow keys, home/end, backspace),
   history within the session (up/down arrows), Ctrl+C cancels the current
   line (not the session), Ctrl+D exits the session.
2. **Prompt.** `wolfcastle> ` (clear, branded, no ambiguity about what's
   accepting input).
3. **History.** In-memory only (not persisted across sessions — unblock
   sessions are short-lived).
4. **Multi-line input.** Not supported (each Enter submits). If users need
   multi-line, they can pipe input or use the agent tier (Tier 3).
5. **Terminal width.** readline handles this natively.
6. **Color.** Model responses are printed with no color modification (the
   model may emit ANSI codes depending on the CLI used). The prompt itself
   uses no color.
7. **Graceful exit.** `quit`, `exit`, Ctrl+D, or empty input all end the
   session (same as current behavior).
8. **Dependency justification.** The readline dependency is the second
   external dependency (after Cobra). This is acceptable because:
   - The alternative (implementing readline from scratch) is not worth the
     effort.
   - readline is a stable, mature library with no transitive dependencies.
   - It's only imported by cmd/unblock.go — not pulled into the core
     library.

### Dependency Risk

ADR-056 deliberately evaluated and retained Cobra as the sole external
dependency, concluding that its features justify the cost for a 47+ command
CLI. Adding readline makes it the second external dependency for an
autonomous tool — a threshold worth acknowledging explicitly.

Mitigating factors:

- readline is imported only by `cmd/unblock.go`. It is not pulled into the
  daemon, the state layer, or any core library package. A `go build` of a
  hypothetical headless Wolfcastle (daemon-only, no interactive commands)
  would not include readline at all.
- readline has no transitive dependencies (pure Go, no CGO), so it adds
  exactly one entry to `go.sum`, not a dependency tree.
- The interactive unblock session is an optional feature — the agent tier
  (Tier 3, `--agent` flag) provides the same diagnostic context without
  readline.

**Fallback alternative:** If the dependency budget is exceeded, fall back
to `bufio.Scanner` with basic line editing via raw terminal mode —
approximately 150 lines of hand-rolled code using `golang.org/x/term`
(which is a golang.org/x module, not a third-party dependency). This
provides backspace handling and basic line editing without full readline
capabilities (no history, no Ctrl-A/E, no reverse search).

**Review trigger** (matching ADR-056's pattern): Re-evaluate the readline
dependency if any of these conditions become true:

- A CVE is filed against `github.com/chzyer/readline`
- The interactive unblock feature is rarely used in practice (measured by
  whether any user reports or telemetry indicate it's exercised)
- The total external dependency count exceeds 3
- A Go stdlib or golang.org/x package gains readline-equivalent
  functionality

## Consequences

- Interactive sessions feel like a real REPL, not a raw pipe.
- Arrow keys, backspace, and line editing work as expected.
- Users can recall previous inputs within a session.
- One new dependency, but well-scoped and stable.
- No behavior change for non-interactive use (agent tier, piped input).
