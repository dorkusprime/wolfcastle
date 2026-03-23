# ADR-002: JSON for Configuration and State

## Status
Accepted

## Date
2026-03-12

## Context
Ralph used Markdown files (STATUS.md, PROMPT.md) for both configuration and runtime state. The model directly edited these files, which led to potential issues: malformed tables, wrong status strings, forgotten updates. We need a more reliable approach for Wolfcastle.

## Decision
Wolfcastle uses JSON for both configuration and state, split into two categories:

1. **Configuration (JSON)**. User-owned. Wolfcastle reads but never writes. Covers model selection, validation commands, branch conventions, project rules, pipeline definitions, etc.
2. **State (JSON)**. Wolfcastle-owned. Modified only through deterministic scripts, never by the model directly and never by the user (though they can inspect it).

YAML was explicitly rejected. No YAML anywhere in Wolfcastle.

## Consequences
- Models call scripts (`wolfcastle task claim`, `wolfcastle task complete`, etc.) instead of editing state files
- Eliminates an entire class of state corruption bugs from Ralph
- State mutations are validated and atomic
- JSON is universally parseable by both scripts and models
