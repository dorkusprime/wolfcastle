# ADR-077: System Directory Restructure

**Status:** Accepted

## Context

The `.wolfcastle/` directory mixes system internals (config, state, logs, prompts) with model outputs (specs, ADRs, research artifacts). During real-model testing, an executing agent wrote to `base/config.json` thinking it was a project file, when it's actually a scaffold-generated config that should never be hand-edited. The flat layout provides no structural signal about what the model can and cannot touch.

## Decision

Introduce a `system/` subdirectory inside `.wolfcastle/` for all system-managed files. Model-writable directories (`docs/`, `artifacts/`) remain at the `.wolfcastle/` top level.

Before:
```
.wolfcastle/
  base/          config, prompts, rules (scaffold)
  custom/        team overrides (committed)
  local/         personal config (gitignored)
  projects/      state files, inbox
  logs/          daemon logs
  wolfcastle.pid
  stop
  docs/          specs, ADRs (model output)
  artifacts/     research (model output)
```

After:
```
.wolfcastle/
  system/
    base/        config, prompts, rules (scaffold)
    custom/      team overrides (committed)
    local/       personal config (gitignored)
    projects/    state files, inbox
    logs/        daemon logs
    wolfcastle.pid
    stop
  docs/          specs, ADRs (model output)
  artifacts/     research (model output)
```

The prompt rule for executing agents becomes: "Write to `.wolfcastle/docs/` and `.wolfcastle/artifacts/` only. Never touch `.wolfcastle/system/`."

## Migration

`ReScaffold` detects old-layout directories (`base/` at `.wolfcastle/base/` without a `system/` parent) and moves them automatically. Existing `.wolfcastle/` directories migrate on the next `wolfcastle init` or daemon startup.

## Consequences

- Every path reference in the codebase adds a `system/` segment for config, state, logs, scaffold, and resolver paths.
- Prompt templates and documentation update to reference the new layout.
- Old `.wolfcastle/` layouts auto-migrate; no manual intervention required.
- The boundary between "system territory" and "model territory" is physical, not just documented.
