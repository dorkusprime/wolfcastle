# ADR-029: Codebase Audit Command with Discoverable Scopes

## Status
Accepted

## Date
2026-03-13

## Context
Wolfcastle's node-level audit tasks verify that each project's work is correct. But there's no mechanism for a full codebase-level audit: identifying DRY violations, modularity issues, decomposition opportunities, code quality problems, etc. These findings naturally feed into new Wolfcastle projects.

## Decision

### wolfcastle audit
A standalone command that runs a model-driven codebase audit and generates repair projects from findings.

### Flow
1. `wolfcastle audit [--scope <id>,<id>,...]`: run specified scopes (or all if omitted)
2. Model audits the codebase against each scope's prompt
3. Generates prioritized findings
4. Presents findings to the user for approval: the gate between "here's what I found" and "here's what I'll fix"
5. Approved findings become projects/tasks in the tree automatically

### Discoverable Scopes
Audit scopes are identified by enum-like IDs, not free text. Each scope is backed by a prompt fragment:

- `base/audits/dry.md`. DRY violations
- `base/audits/modularity.md`: module boundary issues
- `base/audits/decomposition.md`: overly complex code needing breakdown
- `base/audits/comments.md`: documentation and commenting gaps
- etc.

Users add custom scopes in `custom/audits/` and `local/audits/`. The Go code discovers all available scopes by scanning the three tiers.

### Discovery and Help
```
wolfcastle audit --list              # shows all available scopes with descriptions
wolfcastle audit --scope dry         # run specific scope
wolfcastle audit --scope dry,modularity  # run multiple scopes
wolfcastle audit                     # run all scopes
wolfcastle audit -h                  # help text includes dynamically discovered scopes
```

Scope descriptions are extracted from the prompt fragments (e.g., a frontmatter line or first paragraph). `--list` and `-h` display them dynamically.

### Prompt Decomposition
Each scope has its own focused prompt fragment. The audit command assembles only the relevant prompts for the requested scopes. This keeps individual runs focused and token-efficient rather than sending one massive audit prompt.

### Configuration
```json
{
  "audit": {
    "model": "heavy",
    "prompt_file": "audit.md"
  }
}
```

The `prompt_file` is the base audit prompt (framing, output format, approval workflow). Scope prompts are assembled on top of it.

### Approval Step
Findings are presented interactively. The user can:
- Approve all → projects created for every finding
- Review individually → approve/reject each finding
- Reject all → nothing created

This is the gate that prevents runaway project creation from an overzealous audit.

## Consequences
- Wolfcastle can proactively discover and plan its own work
- Scopes are composable and extensible via the three-tier system
- Custom scopes let teams audit domain-specific concerns (e.g., "security", "accessibility")
- The approval step keeps the user in control
- Token cost is managed by scoping: run only what you need
- `--list` and `-h` are always current because they discover scopes at runtime
