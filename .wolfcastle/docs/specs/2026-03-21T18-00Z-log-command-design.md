# wolfcastle log: Design Spec

## What This Does

`wolfcastle log` displays daemon activity reconstructed from NDJSON log files. The default output is a groomed, human-readable summary of what the daemon did (or is doing). Flags control verbosity from summary-only to full agent output.

## Default Behavior

When invoked with no flags:

- If the daemon is running, follow the active session's log output in real time (equivalent to implicit `--follow`).
- If the daemon is stopped, display the last session's log output and exit.

## Sessions

Each daemon start is a session. Log files are per-iteration (`0001-20260321T18-04Z.jsonl`, `0002-20260321T18-05Z.jsonl`, etc.) stored in `.wolfcastle/system/logs/`. The iteration counter resets to 1 on each daemon start. Session boundaries are detected by finding the iteration-1 file or by timestamp discontinuity.

`--session 0` (default) is the current or most recent session. `--session 1` is the previous session.

## Output Modes

Three modes, mutually exclusive. Default is summary.

### Summary (default)

One line per completed stage: stage type, node address, duration.

```
[intake]  donut-stand-website                              (12s)
[plan]    donut-stand-website                              (45s)
[execute] donut-stand-website/site-specification/task-0001 (1m22s)
[execute] donut-stand-website/site-specification/task-0002 (58s)
[audit]   donut-stand-website/site-specification           (34s)
  report: .wolfcastle/system/projects/.../audit-2026-03-21T18-08.md
[execute] donut-stand-website/project-foundation/task-0001 (2m5s)
```

In follow mode, each stage prints a start line and a completion line:

```
▶ [execute] donut-stand-website/site-specification/task-0001
✓ [execute] donut-stand-website/site-specification/task-0001 (1m22s)
▶ [execute] donut-stand-website/site-specification/task-0002
✗ [execute] donut-stand-website/site-specification/task-0002 (3m41s)
```

`▶` marks a stage starting. `✓` marks completion. `✗` marks failure or block.

When replaying a completed session (no `--follow`), only the completion lines are shown (with `✓` or `✗` glyphs and durations).

### Thoughts (`--thoughts`)

Raw agent output only. No stage headers, no durations, no timestamps, no glyphs. Just what the model said, as it said it.

```
I'll start by creating the site specification document...
Reading the project requirements from the inbox item...
The inbox item asks for a donut stand website, so I'll need...
Now I need to write the HTML structure...
```

In follow mode, this tails the active NDJSON log file and extracts `"type": "assistant"` records in real time.

For historical sessions, all agent output from every iteration in the session is shown. Pipe to `less` if it's too much.

### Interleaved (`--interleaved`)

Stage headers and agent thoughts together, chronologically, with wall-clock timestamps and glyphs.

```
18:01:34 ▶ [execute] donut-stand-website/site-specification/task-0001
18:01:35     I'll start by creating the site specification document...
18:01:36     Reading the project requirements from the inbox item...
18:02:56 ✓ [execute] donut-stand-website/site-specification/task-0001 (1m22s)
18:02:57 ▶ [execute] donut-stand-website/site-specification/task-0002
18:02:58     Now I need to write the HTML structure...
```

Agent thoughts are indented to visually separate them from stage headers.

## Formatting Rules

- No sequence numbers.
- No terminal markers (`WOLFCASTLE_COMPLETE`, `WOLFCASTLE_BLOCKED`, etc.) in output.
- No log levels (`[INFO]`).
- Uniform formatting across all stage types. No visual difference between intake, planning, execution, and audit.
- Stage labels: `[intake]`, `[plan]`, `[execute]`, `[audit]`, `[remediate]`, `[spec-review]`.
- Duration: compact human shorthand with no spaces: `34s`, `1m22s`, `12m5s`, `1h3m`.
- Wall-clock timestamps only in `--interleaved` mode. Format: `HH:MM:SS` (local time).
- Audit report paths shown indented below the audit completion line when a report was generated.

## Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--follow` | `-f` | Follow live output. Default when daemon is running. No-op when daemon is stopped. |
| `--thoughts` | `-t` | Raw agent output only. No stage headers or durations. |
| `--interleaved` | `-i` | Stage headers and agent output together with timestamps. |
| `--json` | | Raw NDJSON output. No formatting, no filtering. Pipe to `jq`. |
| `--session` | `-s` | Session index (0 = latest, 1 = previous, etc.). Default: 0. |

`--thoughts`, `--interleaved`, and `--json` are mutually exclusive. If more than one is passed, the last one wins.

## Data Source

Display formatting and layout logic lives in the `wolfcastle log` command. The daemon writes NDJSON records with timestamps and pre-computes `duration_ms` (elapsed milliseconds as an integer) on `stage_complete` and `planning_complete` records for structured consumers. Renderers own display formatting: they may prefer the pre-computed `duration_ms` value or fall back to computing durations from timestamp diffs.

All output is reconstructed from the NDJSON log files in `.wolfcastle/system/logs/`. The command never reads the daemon's stdout directly. This means:

- `wolfcastle log` works whether the daemon is running or stopped.
- Historical sessions are replayable.
- The groomed output is a view over the structured data, not a capture of the daemon's console.

## NDJSON Records Used

The log command reads these record types from the log files:

| `type` field | Used for |
|--------------|----------|
| `iteration_start` | Stage header (node address, stage type) |
| `stage_start` | Stage start timestamp, stage type |
| `stage_complete` | Stage end timestamp (for duration), exit code (for `✓`/`✗`). Contains `duration_ms` (integer): pre-computed elapsed milliseconds for the stage. |
| `assistant` | Agent thoughts (debug level) |
| `audit_report_written` | Audit report path |
| `planning_start` / `planning_complete` | Planning stage boundaries. `planning_complete` contains `duration_ms` (integer): pre-computed elapsed milliseconds for the planning phase. |
| `daemon_lifecycle` | Engaged/standing-down banners, drain, crash-restart. Fields: `event`, `scope`, `reason`. |
| `self_heal` | Startup recovery: scan, reset, derive, remediation. Fields: `action`, `text`. |
| `iteration_header` | Per-iteration banner (execute or plan). Fields: `iteration`, `kind`, `text`. |
| `inbox_event` | Watcher deploy, intake processing, intake results. Fields: `action`, `counter`, `text`. |
| `task_event` | Superseded, audit remediation, deliverable warnings, no-progress. Fields: `action`, `task`, `text`. |
| `retry_event` | Invocation retry with backoff. Fields: `attempt`, `delay_s`, `error`, `text`. |
| `idle_reason` | Why the daemon is idle (all-complete, empty tree, all-blocked). Fields: `reason`, `text`. |
| `archive_event` | Auto-archive success or failure. Fields: `action`, `node`, `text`. |
| `spec_event` | Spec review queued or failed. Fields: `action`, `node`, `text`. |
| `knowledge_event` | Knowledge budget exceeded. Fields: `action`, `node`, `text`. |
| `config_warning` | Configuration warnings (e.g., not a git repo). Fields: `text`. |

Renderers degrade gracefully for unrecognized record types (skip, don't crash). Old NDJSON files without the new fields parse to zero values.

## Non-Daemon Mode

When wolfcastle runs in non-daemon (foreground) mode (e.g., `wolfcastle execute`, `wolfcastle intake`), the default renderer is summary: one line per completed stage, matching the format described in the Summary section above. This keeps foreground output compact by default, showing progress without flooding the terminal with agent thoughts.

Foreground mode accepts the same output-mode flags as `wolfcastle log`: `--thoughts`, `--interleaved`, and `--json`. These flags follow the same last-wins semantics described in the Flags section. If no output-mode flag is passed, summary is the default.

The daemon itself does not format this output. It writes NDJSON to the log file as usual, and a goroutine tails that file and renders the selected view to stdout. The rendering logic is shared with `wolfcastle log`, so the display code for each mode exists in one place.

## What This Does Not Cover

- Filtering by node address (use `grep`).
- Custom format strings.
- Log file management (rotation, retention, compression are handled by the logging package).

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success (or follow mode interrupted by Ctrl+C). |
| 1 | No log files found, or not initialized. |
