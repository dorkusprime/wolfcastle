# ADR-046: Structured Log Levels

**Status:** Accepted

**Date:** 2026-03-14

## Context

The daemon currently logs everything at the same level — stage starts, stage
completions, errors, retries, breadcrumbs, and debug output all go into the
same NDJSON stream with no severity distinction (ADR-012, ADR-037). The
human-readable console output (via output.PrintHuman) similarly has no level
control. As the daemon runs longer sessions (dozens to hundreds of
iterations), the log volume makes it difficult to find important events.
Operators need to distinguish "the daemon is working normally" from "something
needs attention."

## Decision

Add a `level` field to all NDJSON log records with four severity tiers:
`debug`, `info`, `warn`, `error`.

1. **Console filtering.** A configurable log level in the daemon config
   filters console output. NDJSON always captures everything regardless of
   level.
2. **Default console level.** `info` (suppresses debug-level output like
   stage skip reasons).
3. **Config.** `"daemon": { "log_level": "info" }` — accepts debug, info,
   warn, error.
4. **Verbose flag.** `--verbose` / `-v` on `wolfcastle start` overrides
   log_level to `debug`.
5. **Level mapping:**
   - `debug`: stage skip reasons, inbox state checks, iteration context
     details.
   - `info`: stage start/complete, iteration start, daemon start/stop,
     expand/file item counts.
   - `warn`: non-fatal stage errors, retry attempts, stale PID detection,
     validation warnings.
   - `error`: fatal errors, invocation failures after retry exhaustion,
     state corruption.
6. **Logger.Log() signature.** Gains an optional level parameter (backward
   compatible: if missing, defaults to info).
7. **AssistantWriter.** Model output streaming logs at debug level — model
   output is always available in the NDJSON file but only shown on console
   with `-v`.
8. **wolfcastle follow.** Always shows all levels (it tails the NDJSON file,
   which captures everything).

## Consequences

- Operators can run the daemon at info level for clean output and drop to
  debug when investigating.
- NDJSON files remain the complete, unfiltered record — log level only
  affects console.
- `--verbose` is the quick escape hatch for "show me everything."
- Log records are now machine-parseable by level for monitoring/alerting
  tools.
- Backward compatible — existing log records without a level field are
  treated as info.
