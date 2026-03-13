# ADR-005: Composable Rule Fragments with Sensible Defaults

## Status
Accepted

## Date
2026-03-12

## Context
Ralph injected project rules via a single `CLAUDE-ralph.md` file concatenated with the orchestrator prompt. Different projects have different needs — git conventions, code style, testing strategies, ADR policies.

## Decision
Project rules are composable fragments. Wolfcastle ships sensible defaults (e.g. git conventions, commit format, ADR checking). Users can override defaults or add custom fragments. Fragments are merged in a predictable order defined by config.

Fragments are referenced in the JSON config and resolved at prompt assembly time.

## Consequences
- Users don't need to write rules from scratch — defaults cover common cases
- Overriding a single concern (e.g. commit message format) doesn't require rewriting everything
- Fragment ordering is explicit and predictable
- Wolfcastle updates can ship improved defaults without clobbering user customizations
