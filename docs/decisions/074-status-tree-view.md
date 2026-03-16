# ADR-074: Status Tree View

## Status
Accepted

## Date
2026-03-16

## Context

`wolfcastle status` printed aggregate counts: how many tasks were complete, in progress, blocked, failed. This told you the shape of the battlefield but not where anything was. If three tasks were blocked, you had no idea which three without running separate queries. The command answered "how many" but never "which ones" or "where."

The TUI spec established a glyph vocabulary for representing node and task states visually. The status command was not using it.

## Decision

`wolfcastle status` is rewritten to display a tree view of the entire project structure. Each node and task appears in its hierarchical position with a state glyph.

Node glyphs: `●` (complete), `◐` (in progress), `◯` (not started), `☢` (blocked).

Task glyphs: `✓` (complete), `→` (in progress), `○` (not started), `✖` (blocked/failed).

The tree renders with indentation showing parent-child relationships. Each line shows the glyph, the node or task name, and its current state.

`--watch` or `-w` refreshes the display at a configurable interval, replacing the previous output in place. This gives a live dashboard of the daemon's progress without streaming logs.

When stdout is a terminal, output is colored: green for complete, yellow for in progress, default for not started, red for blocked or failed. When piped, colors are suppressed and raw text is emitted.

## Consequences

- `wolfcastle status` answers "what is happening and where" in a single glance. The tree structure mirrors the project hierarchy, making blocked or stalled tasks immediately visible.
- The glyph vocabulary is shared with the TUI spec, so the terminal status view and the interactive TUI present a consistent visual language.
- `--watch` mode provides a lightweight alternative to the full TUI for operators who want visibility without interactivity.
- Color detection uses standard terminal capability checks. Piped output remains parseable by scripts.
- The old aggregate-counts format is gone. Any tooling that parsed the previous output format will need to adapt.
