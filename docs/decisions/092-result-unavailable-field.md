# Use Result.Unavailable field over sentinel error for stub updater

## Status
Accepted

## Date
2026-03-20

## Status
Accepted

## Context
The stubUpdater needs to signal that no release channel was queried, rather than returning AlreadyCurrent: true. Two approaches were considered.

## Options Considered
1. **Sentinel error** (e.g., ErrUpdateUnavailable): callers must check errors, and it conflates "cannot check" with "something went wrong." Errors typically mean transient failures worth retrying; permanent unavailability is a different category.
2. **Result.Unavailable bool field**: callers inspect the result alongside Updated and AlreadyCurrent. The zero value (false) means "check occurred normally," preserving backwards compatibility for all existing callers that never examine the new field.

## Decision
Option 2: add an Unavailable bool to the Result struct. When set, both Updated and AlreadyCurrent are false, and LatestVersion is empty.

## Consequences
Callers that don't check Unavailable will see a result where nothing happened (no update, not current), which is safe but uninformative. Callers that care (like the update command) can branch on the field to display a distinct message.
