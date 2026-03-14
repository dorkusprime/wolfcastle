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

## Consequences

- Interactive sessions feel like a real REPL, not a raw pipe.
- Arrow keys, backspace, and line editing work as expected.
- Users can recall previous inputs within a session.
- One new dependency, but well-scoped and stable.
- No behavior change for non-interactive use (agent tier, piped input).
