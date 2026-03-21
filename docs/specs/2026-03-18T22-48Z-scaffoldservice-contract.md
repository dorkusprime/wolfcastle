# ScaffoldService Contract

## Overview

`ScaffoldService` in `internal/project` owns the creation and regeneration of the `.wolfcastle/` directory tree. It replaces the current package-level `Scaffold()`, `ReScaffold()`, and `WriteBasePrompts()` functions with a struct that holds its dependencies explicitly, making them injectable for testing.

`Init` builds the full directory structure from nothing (`wolfcastle init`). `Reinit` tears down and rebuilds the base tier while preserving custom and local content (`wolfcastle update`). Both methods delegate file operations to their respective repositories rather than constructing paths and writing files directly.

## Type

```go
type ScaffoldService struct {
    config  *config.ConfigRepository
    prompts *pipeline.PromptRepository
    daemon  *daemon.DaemonRepository
    root    string // path to .wolfcastle/
}
```

The `config` field provides tier-aware config reads and writes. The `prompts` field provides embedded template extraction via `WriteAllBase`. The `daemon` field provides daemon-related filesystem operations (PID files, stop files, log paths). The `root` field is the absolute path to the `.wolfcastle/` directory, from which all scaffold paths are derived.

## Constructor

### NewScaffoldService(config \*config.ConfigRepository, prompts \*pipeline.PromptRepository, daemon \*daemon.DaemonRepository, root string) \*ScaffoldService

Stores the provided dependencies and returns the service. No filesystem work happens at construction time.

## Methods

### Init(identity \*config.Identity) error

Creates the full `.wolfcastle/` directory structure for a fresh `wolfcastle init`. This is the only entry point for first-time setup.

**Algorithm:**

1. Create the directory tree. Each path is joined against `root`:
   - `system/base/prompts`
   - `system/base/rules`
   - `system/base/audits`
   - `system/custom`
   - `system/local`
   - `system/projects`
   - `system/logs`
   - `archive`
   - `artifacts`
   - `docs/decisions`
   - `docs/specs`

   All directories are created with `0755` permissions via `os.MkdirAll`. A failure on any directory returns immediately with a wrapped error.

2. Write `.gitignore` at `{root}/.gitignore` with `0644` permissions. The gitignore content ignores everything by default, then whitelists `system/custom/`, `system/projects/`, `archive/`, and `docs/` (each with the multi-level unignore pattern Git requires).

3. Write base config. Call `config.Defaults()` to obtain a fresh `*Config`, set `Identity` to nil (identity belongs only in the local tier), then write it via `s.config.WriteBase(defaults)`.

4. Write empty custom config. Call `s.config.WriteCustom(map[string]any{})` to place an empty JSON object at `system/custom/config.json`, giving teams a file to edit.

5. Write local config with identity. Build a partial overlay `map[string]any{"identity": {"user": identity.User, "machine": identity.Machine}}` and write it via `s.config.WriteLocal(overlay)`.

6. Create the namespace projects directory. Call `os.MkdirAll(identity.ProjectsDir(root), 0755)` to create `system/projects/{user}-{machine}/`.

7. Write the empty root index. Create a `state.NewRootIndex()`, marshal it to indented JSON, and write it to `{identity.ProjectsDir(root)}/state.json` with `0644` permissions.

8. Extract embedded prompts. Call `s.prompts.WriteAllBase(project.Templates)` to walk the embedded `templates/` filesystem and write each file into the base tier. The `Templates` variable is the `embed.FS` already present in the project package.

9. Return nil on success.

### Reinit() error

Regenerates the base tier and refreshes identity for `wolfcastle update`. Preserves custom and local content. Runs migrations first so that users upgrading from older directory structures land in the correct layout before regeneration begins.

**Algorithm:**

1. Run migrations. Construct a `MigrationService{config: s.config, root: s.root}` and call both `MigrateDirectoryLayout()` and `MigrateOldConfig()`. Discard both return values (migrations are best-effort; errors are logged but do not abort rescaffold).

2. Remove and recreate `system/base/`. Call `os.RemoveAll(filepath.Join(root, "system", "base"))`, then create the subdirectories `system/base/prompts`, `system/base/rules`, and `system/base/audits` with `os.MkdirAll`. Errors from removal or creation propagate immediately.

3. Regenerate base config. Same logic as `Init` step 3: `config.Defaults()` with `Identity` set to nil, written via `s.config.WriteBase(defaults)`.

4. Extract embedded prompts. Same as `Init` step 8: `s.prompts.WriteAllBase(project.Templates)`.

5. Ensure custom config exists. If `system/custom/config.json` does not exist (checked via `os.Stat`), call `s.config.WriteCustom(map[string]any{})`. If it already exists, leave it untouched.

6. Refresh identity in local config. Read the existing `system/local/config.json` (if present) into `map[string]any`. Overlay the current identity by setting `localCfg["identity"]` to `DetectIdentity()`'s result (as a map with `"user"` and `"machine"` keys). Write back via `s.config.WriteLocal(merged)`. If the local tier directory or file does not exist, create it fresh with only the identity key.

7. Return nil on success.

## Error Behavior

All errors returned by ScaffoldService are prefixed with `"scaffold:"` for consistent identification at call sites.

- **Init**: any filesystem or repository error halts the method and propagates immediately. A partial scaffold may remain on disk; rerunning `Init` is not idempotent (it may collide with existing files). The caller (`wolfcastle init`) should check for an existing `.wolfcastle/` directory before calling `Init`.
- **Reinit**: migration errors are discarded (step 1). All subsequent errors propagate immediately. Because `Reinit` removes `system/base/` before rebuilding it, a failure partway through step 2-4 leaves the base tier incomplete. Rerunning `Reinit` recovers cleanly since `os.RemoveAll` followed by `os.MkdirAll` is idempotent.

## Thread Safety

`ScaffoldService` is called at daemon startup (`Init` for first run, `Reinit` for updates) before any concurrent iteration work begins. No synchronization is needed. The repository fields (`config`, `prompts`, `daemon`) are themselves safe for the reads and writes performed here, as no other goroutine is active during scaffolding.

## Invariants

- `Init` writes identity only to the local tier, never to base or custom. This ensures `system/base/config.json` contains `Defaults()` with `Identity: nil`, and the local tier is the single source of truth for who is running the daemon.
- `Reinit` never modifies `system/custom/`. If custom config already exists, it is left exactly as the user or team configured it. The only custom-tier action is creating an empty file when none exists.
- `Reinit` always regenerates `system/base/` from scratch. There is no diffing or partial update; the entire base tier is removed and rebuilt. This guarantees that base always reflects the current version's defaults and embedded templates.
- `Reinit` always refreshes identity. Even if the identity has not changed, the local config is rewritten with the current OS-detected values. This handles machine renames and username changes without user intervention.
- The namespace projects directory (`system/projects/{user}-{machine}/`) is created by `Init` but not by `Reinit`. `Reinit` does not touch the projects directory or its contents; project state survives updates.
- Embedded prompts flow through `PromptRepository.WriteAllBase`, not through direct filesystem writes. This ensures the repository's `tierfs.Resolver` governs where base files land, keeping the path logic in one place.

## Relationship to Existing Functions

`ScaffoldService` replaces the following package-level functions:

| Current function | Replaced by | Notes |
|---|---|---|
| `Scaffold(wolfcastleDir)` | `ScaffoldService.Init(identity)` | Identity is now a `*config.Identity` parameter rather than detected internally via `detectIdentity()` |
| `ReScaffold(wolfcastleDir)` | `ScaffoldService.Reinit()` | Migrations delegate to `MigrationService` rather than calling `migrateToSystemLayout` and `migrateOldConfig` directly |
| `WriteBasePrompts(wolfcastleDir)` | `PromptRepository.WriteAllBase(templates)` | Prompt extraction moves to the repository that owns prompt files |
| `detectIdentity()` | `config.DetectIdentity()` | Identity detection moves to the config package where the `Identity` type lives |

The `Templates` embedded filesystem (`embed.FS`) remains in the `project` package. `ScaffoldService` passes it to `PromptRepository.WriteAllBase` at scaffold time.
