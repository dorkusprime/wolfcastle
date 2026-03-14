# wolfcastle adr create

Creates a new Architecture Decision Record.

## What It Does

Generates a timestamped filename (`{YYYY}-{MM}-{DD}T{HH}-{mm}Z-{slug}.md`), writes it to `.wolfcastle/docs/decisions/`. If you provide body content via `--stdin` or `--file`, uses that. Otherwise, generates a template with Status, Date, Context, Decision, and Consequences sections.

## Usage

```
wolfcastle adr create "Use NDJSON for daemon logs"
wolfcastle adr create --stdin "Use NDJSON for daemon logs" < body.md
wolfcastle adr create --file rationale.md "Use NDJSON for daemon logs"
```

## Flags

| Flag | Description |
|------|-------------|
| `--stdin` | Read body content from stdin. |
| `--file <path>` | Read body content from a file. |
| `--json` | Output as structured JSON. |

`--stdin` and `--file` are mutually exclusive.

## Arguments

| Argument | Description |
|----------|-------------|
| `title` | **(Required)** ADR title. Gets slugified for the filename. |

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | ADR created. |
| 1 | Not initialized. |
| 2 | Empty title. |
| 3 | File not found (with `--file`). |
| 4 | Both `--stdin` and `--file` specified. |

## Consequences

- Creates a new Markdown file in `.wolfcastle/docs/decisions/`.

## See Also

- [Architecture Decisions](../../decisions/INDEX.md) for the existing ADR index.
