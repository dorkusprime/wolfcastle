# One-level hierarchical fallback for class prompt resolution

## Status
Accepted

## Date
2026-03-18

## Status
Accepted

## Context
ClassRepository resolves prompt files for class keys like "typescript/react" or "lang-go". When the exact prompt file is missing, the repository needs a strategy for falling back to a more general prompt.

## Options Considered
1. **No fallback.** Every class key requires its own prompt file. Simple, but forces boilerplate duplication when many sub-classes share a parent behavior.
2. **One-level fallback.** Strip the last segment (after "/" or "-") and try the parent key. "typescript/react" falls back to "typescript", "lang-go" falls back to "lang". No deeper recursion.
3. **Recursive fallback.** Walk the key upward through all separators until a file is found or the key is exhausted. "a/b/c" tries "a/b/c", then "a/b", then "a".
4. **Glob/wildcard matching.** Use pattern matching to find the closest prompt file.

## Decision
One-level fallback (option 2). The "/" separator is checked first (hierarchical keys), then "-" (hyphenated keys). Only one fallback attempt is made.

Recursive fallback adds complexity for a use case that doesn't exist today: class keys are at most two levels deep. If deeper nesting emerges, a future revision can extend the chain. The one-level approach keeps resolution predictable and easy to debug (you look at the exact key, then the parent, done).

Both "/" and "-" are supported because the codebase uses both conventions: "/" for hierarchical namespaces (typescript/react) and "-" for flat compound keys (lang-go). Checking "/" first means hierarchical keys resolve correctly even if they also contain hyphens.

## Consequences
Classes with keys deeper than two levels (e.g., "typescript/react/hooks") can only fall back one level to "typescript/react", not to "typescript". This is acceptable because no such keys exist in the current config schema. If they appear, the fallback chain should be revisited.
