# ADR-012: NDJSON Logs with Configurable Rotation

## Status
Accepted

## Date
2026-03-12

## Context
Wolfcastle runs as a long-lived daemon producing logs from multiple iterations across multiple model invocations. Logs need to support real-time streaming (`wolfcastle follow`), after-the-fact querying, and must not fill up disk over weeks of continuous operation.

We evaluated NDJSON, plain text, SQLite, and binary formats. NDJSON wins because it is append-only (crash-safe), streamable via `tail -f`, queryable with `jq` or DuckDB, and supported by every observability pipeline.

## Decision

### Format
Logs are NDJSON (newline-delimited JSON). Each line is a self-contained JSON record.

### Per-Iteration Files
Each daemon iteration produces its own log file in `.wolfcastle/system/logs/`, named with an iteration prefix and timestamp:

```
.wolfcastle/system/logs/0001-20260312T18-45Z.jsonl
.wolfcastle/system/logs/0002-20260312T18-47Z.jsonl
```

The iteration prefix provides ordering; the timestamp provides context. `wolfcastle follow` finds the highest-numbered file and tails it, watching for new files to appear when the next iteration starts. No symlinks: fully cross-platform compatible.

### Retention
Logs are retained by count and age, not by file size (since each iteration is its own file). Old log files can optionally be compressed.

### Configuration
Retention settings are configurable in `config.json` under a `logs` key, with sensible defaults:

```json
{
  "logs": {
    "max_files": 100,
    "max_age_days": 30,
    "compress": true
  }
}
```

All fields are optional: omitted fields use the defaults shown above.

### Querying
- **Real-time**: `wolfcastle follow` streams the active iteration's model output
- **After the fact**: `jq` for simple filters, DuckDB for SQL over log history
- No custom query tooling needed

## Consequences
- Each iteration is independently inspectable: easy to correlate logs with commits
- Cross-platform compatible: no symlinks
- Teams can tune retention via config: stricter for CI, looser for long-running projects
- NDJSON is the only log format; no plain-text fallback needed
- Log cleanup is handled by the Go binary, not external tools like logrotate
- Logs live in `.wolfcastle/system/logs/`, gitignored by the wildcard rule in ADR-009
