# wolfcastle config remove

Removes a value from a configuration array in a tier overlay.

## What It Does

Finds and removes the first matching value from an array at the specified dot-notation path. The value is parsed as JSON first, falling back to a plain string. Comparison uses JSON equality: both values are marshaled to JSON and the resulting strings are compared.

## Usage

```
wolfcastle config remove audit.scopes "security"
wolfcastle config remove --tier custom pipeline.stage_order "custom-stage"
```

## Flags

| Flag | Description |
|------|-------------|
| `--tier <name>` | Target tier. One of: `local` (default), `custom`. |

## Arguments

| Argument | Description |
|----------|-------------|
| `<key>` | Dot-notation path to the array. |
| `<value>` | Value to remove. Parsed as JSON, falling back to plain string. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Value removed. |
| 1 | Not initialized, invalid arguments, key is not an array, or value not found. |

## Consequences

- Mutates the target tier's configuration file on disk.

## See Also

- [`wolfcastle config append`](config-append.md) to add a value to an array.
- [`wolfcastle config show`](config-show.md) to inspect the result.
- [Configuration](../configuration.md) for the tier merging model.
