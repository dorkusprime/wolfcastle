# wolfcastle config append

Appends a value to a configuration array in a tier overlay.

## What It Does

Adds a value to the end of an array at the specified dot-notation path. The value is parsed as JSON first, falling back to a plain string. If the key does not exist, a new single-element array is created. Returns an error if the key exists but is not an array.

## Usage

```
wolfcastle config append audit.scopes "security"
wolfcastle config append --tier custom pipeline.stage_order "custom-stage"
```

## Flags

| Flag | Description |
|------|-------------|
| `--tier <name>` | Target tier. One of: `local` (default), `custom`. |

## Arguments

| Argument | Description |
|----------|-------------|
| `<key>` | Dot-notation path to the array. |
| `<value>` | Value to append. Parsed as JSON, falling back to plain string. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Value appended. |
| 1 | Not initialized, invalid arguments, or key is not an array. |

## Consequences

- Mutates the target tier's configuration file on disk.

## See Also

- [`wolfcastle config remove`](config-remove.md) to remove a value from an array.
- [`wolfcastle config show`](config-show.md) to inspect the result.
- [Configuration](../configuration.md) for the tier merging model.
- [Config Reference](../config-reference.md) for every field, its type, and default value.
