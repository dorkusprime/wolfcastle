# wolfcastle log

Reads the daemon's logs. Without flags, shows recent output and exits. With `--follow`, streams in real time.

## What It Does

Finds the highest-numbered [log file](../collaboration.md#logging) in `.wolfcastle/logs/`, parses NDJSON into human-readable output with timestamps, stage names, and model activity, then prints the last `n` lines.

With `--follow` (`-f`), continues streaming new lines as they appear. When the daemon moves to a new iteration and creates a new log file, `log -f` detects the switch, prints a separator, and starts tailing the new file. Continues until Ctrl+C.

The old command name `wolfcastle follow` still works as an alias.

## Flags

| Flag | Description |
|------|-------------|
| `--follow`, `-f` | Stream output in real time (like `tail -f`). Without this flag, prints recent lines and exits. |
| `--lines <n>` | Number of lines to show. Default: 20. |
| `--level`, `-l` | Minimum log level to display: `debug`, `info`, `warn`, `error`. Default: `info`. |
| `--json` | Output as structured JSON. |

## Examples

```
wolfcastle log                  # show last 20 lines, exit
wolfcastle log --lines 50       # show last 50 lines, exit
wolfcastle log -f               # stream in real time
wolfcastle log -f -l debug      # stream everything including debug
```

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Output complete (no --follow) or user interrupted (Ctrl+C). |
| 1 | Not initialized, or no logs found. |

## Consequences

None. This command is strictly read-only.

## See Also

- [`wolfcastle start`](start.md) to launch the daemon.
- [`wolfcastle status`](status.md) for a tree view snapshot.
- [`wolfcastle status --watch`](status.md) for a refreshing dashboard.
