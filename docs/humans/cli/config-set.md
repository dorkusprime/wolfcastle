# wolfcastle config set

Sets a configuration value in a tier overlay.

## What It Does

Writes a value into the specified tier file using dot-notation paths. The value is parsed as JSON first (numbers, booleans, null, objects, arrays). If JSON parsing fails, it falls back to a plain string.

## Usage

```
wolfcastle config set daemon.stall_timeout_seconds 120
wolfcastle config set pipeline.stages.summary.model "claude-3-opus"
wolfcastle config set audit.enabled true
wolfcastle config set --tier custom daemon.max_retries 5
```

## Flags

| Flag | Description |
|------|-------------|
| `--tier <name>` | Target tier. One of: `local` (default), `custom`. |

## Arguments

| Argument | Description |
|----------|-------------|
| `<key>` | Dot-notation path to the configuration key. |
| `<value>` | Value to set. Parsed as JSON, falling back to plain string. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Value set. |
| 1 | Not initialized or invalid arguments. |

## Consequences

- Mutates the target tier's configuration file on disk.

## See Also

- [`wolfcastle config show`](config-show.md) to inspect the result.
- [`wolfcastle config unset`](config-unset.md) to remove a key.
- [Configuration](../configuration.md) for the tier merging model.
