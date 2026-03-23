# ADR-009: Distribution, Project Layout, and Three-Tier File Layering

## Status
Accepted

## Date
2026-03-12

## Context
Wolfcastle needs to be language-agnostic, support multiple engineers on the same repo without merge conflicts, allow teams to share configuration while preserving personal overrides, and ensure new engineers can get started with minimal friction. Ralph was single-operator and committed all state to git, which would cause untenable merge conflicts in multi-engineer use.

We researched distribution patterns across Claude Code, Lefthook, Mise, Task, Nx, Turborepo, and others. The dominant pattern for language-agnostic CLI tools is a single binary distributed via curl installer + brew, with a project-local directory created by an init command.

## Decision

### Distribution
- **Primary install**: `curl` installer script (zero dependencies)
- **Secondary**: Homebrew tap (`brew install wolfcastle`)
- **Optional**: npm wrapper for teams already in that ecosystem
- **Self-update**: `wolfcastle update` built into the CLI

### Project Layout
`wolfcastle init` creates a `.wolfcastle/` directory in the project root:

```
.wolfcastle/
  .gitignore           # Controls what's committed (see below)
  base/                # Wolfcastle-managed defaults, prompts, rules (gitignored)
    config.json        # Compiled defaults (gitignored)
  custom/              # Team-shared overrides and additions (committed)
    config.json        # Team-shared configuration (committed)
  local/               # Personal overrides (gitignored)
    config.json        # Personal overrides, incl. identity (gitignored)
  projects/            # Live work tree state, namespaced per engineer (committed)
    wild-macbook/      # Auto-resolved from identity in local/config.json
    dave-workstation/
  archive/             # Completed work summaries as Markdown (committed)
  docs/                # Wolfcastle-managed documentation (committed)
    decisions/         # ADRs created during execution
    specs/             # Living system specs maintained during execution
```

See ADR-024 for the internal structure within each engineer's namespace directory under `projects/`.

### Three-Tier Merge
Prompts, rules, and configuration resolve in a predictable merge order:

1. **`base/`**. Wolfcastle-managed. Regenerated from the installed version on `wolfcastle init` or `wolfcastle update`. Never edited by users, never committed. Provides sensible defaults.
2. **`custom/`**. Team-owned. Committed to git. Overrides or extends base. Shared across all engineers on the project.
3. **`local/`**. Engineer-owned. Gitignored. Overrides custom and base for personal preferences. Never shared.

A same-named file in a more specific tier replaces the more general tier (`local/` overrides `custom/`, which overrides `base/`). This mirrors the Mise/Lefthook config layering pattern.

### Engineer Identity and Project Namespacing
`wolfcastle init` auto-populates `local/config.json` with the engineer's identity:

```json
{
  "identity": {
    "user": "wild",
    "machine": "macbook"
  }
}
```

Values are derived from `whoami` and `hostname` at init time. The user can override them. At runtime, Wolfcastle concatenates `user-machine` to resolve the engineer's project directory (e.g. `projects/wild-macbook/`). This is transparent: the engineer just runs `wolfcastle start`.

Because each engineer only writes to their own namespace within `projects/`, there are no merge conflicts on state files. Everyone can see what everyone else is working on. When a project completes, a summary graduates to `archive/` and the engineer's subtree can be cleaned up.

### Git Strategy

`.wolfcastle/.gitignore`:
```
*
!.gitignore
!custom/
!custom/**
!projects/
!projects/**
!archive/
!archive/**
!docs/
!docs/**
```

This means:
- **Committed**: `custom/`, `projects/`, `archive/`, `docs/`
- **Gitignored**: `base/`, `local/`, everything else

### Archive (Merge-Conflict-Proof History)
Completed work produces Markdown summaries in `archive/` with unique filenames (date + hash + slug). This is append-only: two engineers completing different work produce different files that coexist without conflicts. The archive serves as searchable, human-readable history without needing to spelunk git log.

### New Engineer Experience
1. Clone the repo (gets `custom/`, `archive/`, `docs/`)
2. `brew install wolfcastle` (or curl installer)
3. `wolfcastle init` (creates `local/config.json` with identity, generates `base/`)
4. `wolfcastle start` (begins work)

## Consequences
- `base/` is never vendored in git: reduces noise in diffs when Wolfcastle updates
- Teams share config and rules via `custom/` without manual coordination
- Personal preferences in `local/` (including `local/config.json`) never leak to teammates
- Project state in `projects/` is committed but namespaced per engineer: no merge conflicts, no lost state
- Everyone can see what everyone else is working on by inspecting `projects/`
- Archive provides conflict-free, append-only history of completed work
- Engineers must install Wolfcastle to use it, but they'd need to anyway
- `wolfcastle update` safely regenerates `base/` without touching `custom/`, `local/`, or `projects/`
- Identity auto-detection means zero manual config for the common case
