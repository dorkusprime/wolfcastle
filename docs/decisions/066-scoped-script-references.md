# ADR-066: Scoped Script References per Pipeline Stage

## Status
Accepted

## Date
2026-03-15

## Context

Every pipeline stage receives the full script reference in its assembled prompt. The intake model sees `task claim`, `task complete`, `archive add`, and every other command it has no business calling. The execute model sees `project create` and `inbox add`, commands the daemon handles or that belong to other stages. Giving a model commands it shouldn't use is like leaving the keys in every car on the lot: nothing stops it from driving the wrong one.

The script reference is a single markdown file structured with `### wolfcastle <name>` headers for each command and `## Section Name` headers grouping related commands. Filtering at the section level is too coarse (the "Task Commands" section contains both `task add` and `task claim`, which belong to different stages), so the filter needs command-level granularity.

## Decision

Add an `AllowedCommands` field to `PipelineStage` that lists individual command names matching `### wolfcastle <name>` headers in the script reference. During prompt assembly, a filter function parses the reference into blocks and includes only matching command blocks. Section headers (`##` lines) are kept when at least one command beneath them survives; otherwise the entire section is elided.

An empty or nil `AllowedCommands` list means "include everything," preserving backward compatibility for custom stages that don't opt into filtering.

Default scoping:

- **intake**: `project create`, `task add`
- **execute**: `task add`, `task block`, `audit breadcrumb`, `audit escalate`, `status`, `spec list`

The script reference files themselves are not modified. No per-stage copies exist. Filtering happens at assembly time in Go code, keeping the single-source-of-truth property from ADR-017 intact.

## Consequences

- Models receive only the commands they're authorized to use, reducing the surface for unintended operations.
- Adding a new command to a stage requires updating `AllowedCommands` in the defaults (or in the user's config overlay), not editing prompt files.
- Custom pipeline stages defined in config overlays continue to receive the full reference unless they explicitly set `allowed_commands`.
- The filter function is pure (string in, string out) and straightforward to test without filesystem setup.
