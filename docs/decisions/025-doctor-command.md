# ADR-025: Wolfcastle Doctor — Structural Validation and Repair

## Status
Accepted

## Date
2026-03-13

## Context
With distributed state files across a tree of nodes (ADR-024), structural issues are a first-class concern: orphaned files, missing index entries, state inconsistencies, stale In Progress states, missing audit tasks. These need a dedicated diagnostic and repair mechanism, not ad-hoc fixes.

## Decision

### wolfcastle doctor
A dedicated command that validates structural integrity and offers to fix issues.

### Flow
1. **Go code runs structural validation** — walks the tree, checks invariants, identifies specific issues with precise locations (node path, file, field)
2. **Reports findings** — lists every issue with its location and severity
3. **User confirms** — fix all, fix selected, or abort
4. **Deterministic fixes** (missing audit task, stale index entry, orphaned files) — Go code fixes directly, no model needed
5. **Ambiguous fixes** (conflicting state, unclear intent) — a configurable model reasons about the right resolution, proposes a fix, Go code validates before applying

### Configuration
```json
{
  "doctor": {
    "model": "mid",
    "prompt_file": "doctor.md"
  }
}
```

### Structural Validation as Infrastructure
The validation engine is a core piece of Wolfcastle, not just a doctor feature. A subset of checks runs on daemon startup to catch obvious issues early. The doctor command runs the full suite.

### Error Location
The Go code identifies the approximate error location for every issue — node path, file path, and field name where possible. The model receives this context when reasoning about ambiguous fixes.

## Consequences
- Structural issues are caught and fixable without manual JSON editing
- Report-then-fix with confirmation means no surprise mutations
- Deterministic fixes don't waste model tokens
- Model only involved for genuinely ambiguous cases, with heavy guardrails
- Startup validation catches common issues before the daemon begins work
- The validation engine is reusable infrastructure for testing and CI
