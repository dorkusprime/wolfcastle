# ADR-037: Daemon Dual Output (Console + NDJSON)

## Status

Superseded by [ADR-097](097-unified-log-output.md)

## Date

2026-03-13

## Context

The daemon produces two kinds of output:

1. **Human-facing console messages**: lifecycle events (start, stop, iteration markers), errors, and warnings printed via `fmt.Printf` to stdout/stderr.
2. **Structured NDJSON log records**: per-iteration `.jsonl` files containing stage events, marker mutations, errors, and model output, managed by `internal/logging`.

All three implementations mixed these inconsistently: some events only went to console, others only to the logger.

## Decision

Both channels are intentional and serve different purposes:

- **Console** (`fmt.Print*`): for operators watching the daemon in a terminal. Kept terse and human-readable.
- **NDJSON logger** (`d.Logger.Log()`): for programmatic analysis, debugging, and audit trails. Contains all structured events.

Key lifecycle events (daemon start/stop, iteration start, errors) are emitted to **both** channels. Detailed structured events (marker parsing, stage timing, propagation) go only to the NDJSON logger.

## Consequences

- Operators get readable output without parsing JSON.
- Tooling can parse `.jsonl` files for monitoring, alerting, or replay.
- No event is lost: everything important reaches the NDJSON log.
