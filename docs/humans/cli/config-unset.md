# wolfcastle config unset

Removes a configuration value from a tier overlay.

## What It Does

Deletes a key (and any nested structure beneath it) from the specified tier file using dot-notation paths. Succeeds silently if the key does not exist.

## Usage

```
wolfcastle config unset daemon.stall_timeout_seconds
wolfcastle config unset --tier custom pipeline.stages.summary
```

## Flags

| Flag | Description |
|------|-------------|
| `--tier <name>` | Target tier. One of: `local` (default), `custom`. |

## Arguments

| Argument | Description |
|----------|-------------|
| `<key>` | Dot-notation path to the configuration key to remove. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Key removed (or did not exist). |
| 1 | Not initialized or invalid arguments. |

## Consequences

- Mutates the target tier's configuration file on disk.

## See Also

- [`wolfcastle config set`](config-set.md) to write a value.
- [`wolfcastle config show`](config-show.md) to inspect the result.
- [Configuration](../configuration.md) for the tier merging model.
