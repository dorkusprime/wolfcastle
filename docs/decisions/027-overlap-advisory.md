# ADR-027: Cross-Engineer Overlap Advisory

## Status
Accepted

## Date
2026-03-13

## Context
Multiple engineers may independently create projects that touch the same areas of the codebase. Since each engineer's work tree is fully isolated (ADR-024), there is no mechanism to detect this overlap. Without awareness, two engineers could do redundant or conflicting work and only discover it at merge time.

## Decision

### Overlap Advisory
When a new project is created, Wolfcastle optionally runs a lightweight model check that reads other engineers' project description Markdown files and compares them against the new project's intended scope. The comparison is based on the actual changes the projects intend to make (files, systems, codebase areas), not on project names.

### Behavior
- Runs at **project creation time** only, not on every start
- **Read-only** — scans other engineers' `.wolfcastle/projects/*/` directories for project description Markdown files
- **Informational only** — prints an advisory to the console. No errors, no blocking, no state changes
- The engineer can ignore it entirely

### Implementation
This is a configurable pipeline stage:

```json
{
  "overlap_advisory": {
    "enabled": true,
    "model": "fast"
  }
}
```

The model receives the new project's description and all other engineers' active (non-Complete) project descriptions. It identifies potential scope overlap and returns a brief advisory.

### Opt-Out
Disabled by setting `overlap_advisory.enabled` to `false` in config. Disabled by default in `config.local.json` if an engineer doesn't want the extra model call. Enabled by default in team `config.json` if the team values the coordination.

## Consequences
- Engineers get early awareness of potentially overlapping work
- No enforcement, no blocking — purely advisory
- Model cost is minimal (one cheap model call at project creation, reading only descriptions)
- Fully opt-out for cost-sensitive or solo engineers
- Could prevent significant wasted effort and merge pain on teams
- Read-only cross-engineer access is consistent with `wolfcastle status --all` (visibility, not mutation)
