# wolfcastle update

Updates the binary and regenerates `base/`.

## What It Does

Checks the release channel for the latest version. If you're already current, prints a message and exits. Otherwise, downloads and installs the new binary, then regenerates the `base/` directory from it (prompts, rules, script reference).

Does not touch `custom/`, `local/`, or any state files.

## Flags

| Flag | Description |
|------|-------------|
| `--json` | Output as structured JSON. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Updated successfully, or already at latest. |
| 1 | Network error, permission denied, or checksum mismatch. |

## Consequences

- Replaces the `wolfcastle` binary.
- Regenerates `base/` contents. If you installed skills via symlink ([`wolfcastle install skill`](install.md)), those update automatically.

## See Also

- [`wolfcastle init`](init.md) for initial setup.
- [`wolfcastle install skill`](install.md) for how skill updates work.
