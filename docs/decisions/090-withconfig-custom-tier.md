# WithConfig writes to custom tier

## Status
Accepted

## Date
2026-03-18

## Status
Accepted

## Context
Environment.WithConfig needs to merge caller-provided overrides into the three-tier config system. The question is which tier receives the overrides.

## Options Considered
1. Write overrides to base tier (lowest precedence, merged under everything)
2. Write overrides to custom tier (middle precedence, overrides base but not local)
3. Write overrides to local tier (highest precedence, overrides everything)

## Decision
Write to custom tier. Base tier holds immutable defaults written by NewEnvironment. Local tier holds identity config. Custom tier is the natural home for per-test configuration overrides: it takes precedence over base defaults while leaving identity (local) untouched.

## Consequences
- Multiple WithConfig calls accumulate by deep-merging into the same custom/config.json
- Tests that need to override identity must write to local tier directly rather than using WithConfig
- Config resolution order (base < custom < local) matches production behavior
