# wolfcastle log

Shows daemon activity reconstructed from NDJSON log files.

## What It Does

Finds the latest session in `.wolfcastle/system/logs/` and renders it. Default output is a summary: one line per completed stage showing what the daemon did (or is doing).

When the daemon is running, output streams in real time (implicit `--follow`). When the daemon is stopped, the last session's output is displayed and the command exits. `--follow` is a no-op when the daemon is not running.

The old command name `wolfcastle follow` still works as an alias.

Four output modes are available, mutually exclusive. When multiple mode flags appear, the last one wins:

- **(default)** Summary: one line per stage with duration.
- `--thoughts` / `-t`: Raw agent output only.
- `--interleaved` / `-i`: Stage headers and agent output with timestamps.
- `--json`: Raw NDJSON, no formatting.

## Flags

| Flag | Description |
|------|-------------|
| `--follow`, `-f` | Stream output in real time. Implicit when the daemon is running and viewing the latest session. |
| `--session <n>`, `-s` | Session index to display. `0` is the latest (default), `1` is the previous, etc. |
| `--thoughts`, `-t` | Show raw agent output only. |
| `--interleaved`, `-i` | Show stage headers and agent output with timestamps. |
| `--json` | Show raw NDJSON output with no formatting. |

## Examples

```
wolfcastle log                  # summary of latest session
wolfcastle log --thoughts       # raw agent output
wolfcastle log -i -f            # interleaved, streaming
wolfcastle log --session 1      # previous session
wolfcastle log --json | jq '.type'
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
