# ADR-046: Structured Log Levels

**Status:** Accepted (amended by ADR-097)

**Date:** 2026-03-14

## Context

The daemon currently logs everything at the same level: stage starts, stage
completions, errors, retries, breadcrumbs, and debug output all go into the
same NDJSON stream with no severity distinction (ADR-012, ADR-037). As the
daemon runs longer sessions (dozens to hundreds of iterations), the log volume
makes it difficult to find important events. Operators need to distinguish
"the daemon is working normally" from "something needs attention."

## Decision

Add a `level` field to all NDJSON log records with four severity tiers:
`debug`, `info`, `warn`, `error`.

1. **Level mapping:**
   - `debug`: stage skip reasons, inbox state checks, iteration context
     details, model output streaming.
   - `info`: stage start/complete, iteration start, daemon start/stop,
     expand/file item counts.
   - `warn`: non-fatal stage errors, retry attempts, stale PID detection,
     validation warnings.
   - `error`: fatal errors, invocation failures after retry exhaustion,
     state corruption.
2. **Logger.Log() signature.** Gains an optional level parameter (backward
   compatible: if missing, defaults to info).
3. **AssistantWriter.** Model output streaming logs at debug level.
4. **wolfcastle log.** Renderers receive all levels from the NDJSON stream
   and decide what to display.

## Consequences

- Log records are machine-parseable by level for monitoring/alerting tools.
- NDJSON files remain the complete, unfiltered record.
- Backward compatible: existing log records without a level field are
  treated as info.

## Amendment: v0.5.0

ADR-097 eliminated the dual-output console path. The daemon no longer writes
to stdout via `output.PrintHuman`; all events flow through NDJSON. The
original items 1-4 of this ADR (console filtering, `log_level` config,
`--verbose` flag) are obsolete. Level-based filtering, if reintroduced, would
operate on the renderer's input stream rather than a parallel output channel.
The level field itself and its mapping remain unchanged.
