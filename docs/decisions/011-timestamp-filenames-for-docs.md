# ADR-011: ISO 8601 Timestamp Filenames for ADRs and Specs

## Status
Accepted

## Date
2026-03-12

## Context
Sequential numbering for ADRs (001, 002, ...) causes conflicts when multiple engineers create ADRs concurrently. UUIDs and long hashes are too verbose to reference in conversation. We need filenames that are unique, sortable, concise, and human-referenceable.

## Decision
ADRs and specs use ISO 8601 timestamps with minute precision (UTC) as filename prefixes. Colons are replaced with hyphens for filesystem safety.

Format: `{timestamp}-{slug}.md`

Example: `2026-03-12T18-45Z-title-of-decision.md`

This applies to both `docs/decisions/` and `docs/specs/`.

## Consequences
- Files sort chronologically by default in any file listing
- Conflicts require two engineers to create a doc in the same UTC minute — vanishingly rare
- Referenceable in conversation: "see ADR 2026-03-12T18-45Z"
- No renumbering needed when docs are added concurrently
- Consistent pattern across ADRs and specs
