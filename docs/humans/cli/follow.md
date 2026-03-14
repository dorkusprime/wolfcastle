# wolfcastle follow

Tails the daemon's output in real time. Like `tail -f` for your work.

## What It Does

Finds the highest-numbered [log file](../collaboration.md#logging) in `.wolfcastle/logs/`, prints the last `n` lines (parsed from NDJSON into human-readable output with timestamps and stage names), then streams new lines as they appear.

When the daemon moves to a new iteration and creates a new log file, `follow` detects the switch, prints a separator, and starts tailing the new file. Continues until you press Ctrl+C or the daemon exits.

Periodically checks the PID file and process status. When the daemon stops, prints a final message and exits.

## Flags

| Flag | Description |
|------|-------------|
| `--lines <n>` | Number of historical lines to show before streaming. Default: 20. |
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | User interrupted (Ctrl+C) or daemon exited. |
| 1 | Not initialized, or no logs/daemon found. |

## Consequences

None. This command is strictly read-only.

## See Also

- [`wolfcastle start`](start.md) to launch the daemon.
- [`wolfcastle status`](status.md) for a snapshot instead of a stream.
