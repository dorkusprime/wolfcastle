# ADR-097: Unified Log Output (supersedes ADR-037)

## Status
Accepted (supersedes ADR-037)

## Date
2026-04-05

## Context

ADR-037 established a dual-output architecture: console messages via `output.PrintHuman` for operators watching a terminal, and NDJSON log records via `d.Logger.Log()` for programmatic analysis. Key lifecycle events went to both channels.

In practice, this split created a blind spot. The ~55 `output.PrintHuman` calls in the daemon wrote directly to stdout, bypassing the log system entirely. Foreground mode and `wolfcastle log` showed different content for the same session because the renderers only knew about the subset of events that reached the NDJSON files. Self-heal messages, iteration headers, idle reasons, retry attempts, inbox summaries, and lifecycle banners were visible in a terminal but invisible to `wolfcastle log`, the status screen's "Recent:" section, and any tooling that parsed the log files.

ADR-046's console filtering (log levels controlling what appears on the terminal) depended on the dual-output model. With two output paths carrying different events, level-based filtering couldn't bridge the gap: events that never reached the logger couldn't be filtered by level.

## Decision

All daemon output flows through the NDJSON log system. Every `output.PrintHuman` call in the daemon package is replaced by a `d.Logger.Log()` (or the nil-safe `d.log()` wrapper) call with an appropriate record type. The display layer (renderers in `internal/logrender`) decides what to show and how to format it.

New record types carry the events that previously only went to stdout:
- `daemon_lifecycle` (engaged, standing_down, drain, crash_restart)
- `self_heal` (scan, reset, derive, remediation)
- `iteration_header` (execute and plan iteration banners)
- `inbox_event` (watcher deployed, processing, intake results)
- `task_event` (superseded, audit remediation, deliverable warnings, no-progress)
- `retry_event` (attempt, delay, error)
- `idle_reason` (all-complete, empty tree, all-blocked)
- `archive_event`, `spec_event`, `knowledge_event`, `config_warning`, `git_event`

The `Record` struct in `internal/logrender/record.go` gains typed fields for these categories: `Event`, `Action`, `Iteration`, `Kind`, `Reason`, `Counter`, `Attempt`, `DelayS`, `Scope`. All use `omitempty` for backward compatibility with existing log files.

SummaryRenderer and InterleavedRenderer handle all new record types. ThoughtsRenderer ignores them (it only cares about `assistant` records, which is correct).

The single remaining `output.PrintHuman` in the daemon package is the config validation warning in `New()`, which runs before the logger is initialized. The `output` import stays for that call and for the idle spinner.

## Consequences

- `wolfcastle log` shows everything the daemon does: lifecycle banners, self-heal, iteration headers, intake summaries, idle reasons. No more blind spots between foreground and log modes.
- `wolfcastle status` "Recent:" section renders the new record types.
- Replay works: stopping the daemon and running `wolfcastle log` reconstructs the full session including events that were previously stdout-only.
- ADR-046's level-based filtering now applies uniformly. Console filtering (if re-added) can operate on the renderer's input stream rather than requiring a parallel output path.
- Old NDJSON files parse cleanly: the new fields default to zero values, and renderers skip unrecognized types.
- The idle spinner remains as direct terminal output since it is a UI animation, not a log event.
