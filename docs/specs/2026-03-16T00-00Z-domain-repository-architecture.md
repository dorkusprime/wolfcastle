---
status: IMPLEMENTED
---

# Domain Repository Architecture

## Problem

Every package constructs `.wolfcastle/` paths via `filepath.Join` and performs its own filesystem I/O. A directory restructure (ADR-077) required changing 106+ files because the layout is encoded at every call site. The system's relationship with its filesystem is that of an application with a database, but there's no data access layer. Every package writes raw queries.

## Design Principle

Callers work with domain objects, not paths. A command that needs the merged config calls `configRepo.Load()`, not `filepath.Join(wolfcastleDir, "system", "base", "config.json")`. A pipeline stage that needs a class prompt calls `classRepo.Resolve("lang-go")`, not `ResolveFragment("prompts/classes/lang-go.md")`. The tier resolution, path construction, caching, and error handling live inside the repository.

Concrete types are used throughout (not interfaces) except where testability requires a seam. `GitService` and `TierFS` define interfaces because their implementations involve external processes or filesystem I/O that tests should be able to stub. Repositories accept these interfaces in their constructors. All other testing goes through `testutil.Environment`, which provides real repository instances backed by temp directories.

## Package Structure

Repositories live in their domain packages, not in a single `internal/repository/` package. This keeps code co-located with its consumers and avoids a grab-bag package. A shared `internal/tierfs/` package provides the tier resolution primitive that multiple repositories use.

```
internal/tierfs/         ← shared three-tier resolution (base < custom < local)
internal/config/         ← Repository, Identity
internal/pipeline/       ← PromptRepository, ClassRepository, ContextBuilder
internal/daemon/         ← DaemonRepository
internal/state/          ← Store (already exists, unchanged)
internal/git/            ← Service, Provider interface
internal/project/        ← ScaffoldService, MigrationService
internal/testutil/       ← Environment, NodeSpec builders
```

## Shared Tier Resolution (internal/tierfs)

The three-tier file resolution pattern is used by config.Repository, PromptRepository, and ClassRepository. A shared package provides the primitive.

```go
package tierfs

// Resolver is the interface for three-tier file resolution.
// The production implementation uses the filesystem; tests can
// provide in-memory implementations.
type Resolver interface {
    Resolve(relPath string) ([]byte, error)
    ResolveAll(subdir string) (map[string][]byte, error)
    WriteBase(relPath string, data []byte) error
    BasePath(subdir string) string
    TierDirs() []string
}

// FS implements Resolver using the filesystem. Tiers are always
// ["base", "custom", "local"] in that order. This is the only
// place that knows tier names and resolution order.
type FS struct {
    root string // e.g., .wolfcastle/system
}

func New(root string) *FS
```

The `tiers` field is not configurable. The three-tier system is a fixed architectural decision (ADR-063). If that changes, it changes here and nowhere else.

## Repositories

### Repository (formerly ConfigRepository)

Owns the three-tier config merge. Callers get a `*Config`, never see tier files.

```go
type Repository struct {
    tiers tierfs.Resolver
    root  string
}

// NewRepository creates a repository backed by the filesystem.
// Constructs a tierfs.FS internally from wolfcastleRoot + "/system".
func NewRepository(wolfcastleRoot string) *Repository

// NewRepositoryWithTiers creates a repository with an injected
// tierfs.Resolver (for testing with in-memory tier resolution).
func NewRepositoryWithTiers(tiers tierfs.Resolver, root string) *Repository

// Load reads and merges config across all tiers. Starts with Defaults(),
// then deep-merges each tier file that exists. Missing tier files are
// skipped (not errors). This means Load() succeeds even before scaffold
// has run, returning pure defaults.
func (r *Repository) Load() (*Config, error)

// WriteBase writes the full default config (minus identity) to the base
// tier. Accepts *Config because the base tier is a complete config
// snapshot, not a partial overlay.
func (r *Repository) WriteBase(cfg *Config) error

// WriteCustom writes team overrides to the custom tier. Accepts
// map[string]any because custom is a partial overlay that gets
// deep-merged on top of base. Only the keys present are overridden.
func (r *Repository) WriteCustom(data map[string]any) error

// WriteLocal writes personal overrides to the local tier. Same
// partial-overlay semantics as WriteCustom.
func (r *Repository) WriteLocal(data map[string]any) error
```

The asymmetry between `WriteBase(*Config)` and `WriteCustom/WriteLocal(map[string]any)` reflects intent: base is a complete snapshot regenerated from code; custom/local are partial overlays that preserve keys not present in the map.

Replaces: `config.Load()`, `configTiers`, all `filepath.Join` calls to tier config files in scaffold.go and rescaffold.go.

### Identity

Namespace derivation, identity detection, and projects directory mapping form a coherent concept extracted from `Repository`. This keeps `Repository` focused on config merging.

```go
type Identity struct {
    User      string
    Machine   string
    Namespace string // User + "-" + Machine
}

// IdentityFromConfig extracts identity from a loaded config.
// Returns error if identity is not configured.
func IdentityFromConfig(cfg *Config) (*Identity, error)

// DetectIdentity reads username and hostname from the system.
func DetectIdentity() *Identity

// ProjectsDir returns the projects directory for this identity's namespace.
func (id *Identity) ProjectsDir(wolfcastleRoot string) string
```

Replaces: namespace detection from `tree.Resolver`, `detectIdentity()` in scaffold.go, `ProjectsDir()` from `tree.Resolver`.

### PromptRepository

Owns prompt template resolution across tiers. Handles fragment merging, template expansion, and content retrieval.

```go
type PromptRepository struct {
    tiers tierfs.Resolver
}

// NewPromptRepository creates a repository backed by the filesystem.
// Constructs a tierfs.FS internally from wolfcastleRoot + "/system".
func NewPromptRepository(wolfcastleRoot string) *PromptRepository

// NewPromptRepositoryWithTiers creates a repository with an injected
// tierfs.Resolver (for testing with in-memory tier resolution).
func NewPromptRepositoryWithTiers(tiers tierfs.Resolver) *PromptRepository

// Resolve returns the highest-tier version of a prompt with Go
// text/template expansion applied. The name parameter is relative to
// the prompts/ subdirectory without extension: Resolve("execute", ctx)
// resolves "prompts/execute.md" across tiers. Use for stage prompts
// that contain {{.Field}} template syntax.
func (r *PromptRepository) Resolve(name string, ctx any) (string, error)

// ResolveRaw returns raw content from the highest tier without template
// expansion. Takes (category, name) where category is the subdirectory
// ("rules", "prompts", "audits") and name is the filename. Use for
// script references, rule fragments, and content included verbatim.
func (r *PromptRepository) ResolveRaw(category, name string) (string, error)

// ListFragments returns all fragment contents in a category, with tier
// override semantics (same-named files in higher tiers replace lower).
// The include list controls ordering; the exclude list filters.
func (r *PromptRepository) ListFragments(category string, include, exclude []string) ([]string, error)

// WriteBase writes a file to the base tier. relPath is relative to
// the base tier root: WriteBase("prompts/execute.md", data) writes
// to system/base/prompts/execute.md.
func (r *PromptRepository) WriteBase(relPath string, data []byte) error

// WriteAllBase extracts embedded templates from the given fs.FS to
// the base tier. The fs.FS is the embedded template filesystem from
// internal/project/templates/ (go:embed). Paths within the FS map
// directly to base tier paths: "prompts/execute.md" -> system/base/prompts/execute.md.
func (r *PromptRepository) WriteAllBase(templates fs.FS) error
```

`Resolve` takes a short name (e.g., `"execute"`) and appends `prompts/` prefix and `.md` extension internally. `ResolveRaw` takes explicit `(category, name)` for non-prompt content where the caller knows the category. This distinction reflects usage: stages always resolve prompts by name; rule fragment listing always specifies the category.

The script reference is generated at scaffold time by `scriptref.go` and written via `WriteBase("prompts/script-reference.md", data)`. `ScaffoldService` orchestrates this; the repository just stores and retrieves it.

Replaces: `pipeline.ResolveFragment`, `pipeline.ResolveAllFragments`, `pipeline.ResolvePromptTemplate`, `pipeline.Tiers`, `project.WriteBasePrompts`. The `Tiers` variable disappears; tier order is internal to `tierfs.FS`.

### ClassRepository

Owns task class prompt resolution. Built on top of PromptRepository but adds class-specific semantics: hierarchical key resolution, validation, caching.

```go
type ClassRepository struct {
    prompts *PromptRepository
    mu      sync.RWMutex            // protects classes
    classes map[string]ClassDef     // from config
}

func NewClassRepository(prompts *PromptRepository) *ClassRepository

// Reload updates the class list from a freshly loaded config.
// Goroutine-safe: acquires a write lock. Call after config loads
// or reloads. The daemon should call this once at startup, not
// during concurrent iteration.
func (r *ClassRepository) Reload(classes map[string]ClassDef)

// Resolve returns the behavioral prompt for a class key.
// Fallback chain: "lang-go" -> "lang" -> error.
// There is no "default" fallback file. If no prompt exists for the
// key or its parent prefix, Resolve returns an error describing the
// key and the tier directories searched.
// Goroutine-safe: acquires a read lock for the class list.
func (r *ClassRepository) Resolve(key string) (string, error)

// List returns all available class keys from the loaded config.
func (r *ClassRepository) List() []string

// Validate checks that every configured class has a prompt file
// in at least one tier. Returns the list of class keys missing
// prompt files.
func (r *ClassRepository) Validate() []string
```

Class prompts are resolved via `PromptRepository.ResolveRaw("prompts/classes", key+".md")`. The hierarchical fallback strips the last segment after the hyphen: `"lang-go"` tries `lang-go.md`, then `lang.md`. If neither exists, `Resolve` returns an error. There is no catch-all default file.

New code; no existing function replaced. Prevents class resolution from being inlined into `buildIterationContext` and `AssemblePrompt` as the task-classes feature lands. A new class is one `.md` file in a tier directory; the repository finds it automatically.

### DaemonRepository

Owns daemon lifecycle artifacts: PID file, stop file, log directory.

```go
type DaemonRepository struct {
    systemDir string  // derived: wolfcastleRoot + "/system"
}

// NewDaemonRepository creates a DaemonRepository. Internally derives
// systemDir as wolfcastleRoot + "/system". All paths are relative to
// systemDir.
func NewDaemonRepository(wolfcastleRoot string) *DaemonRepository

func (r *DaemonRepository) ReadPID() (int, error)
func (r *DaemonRepository) WritePID(pid int) error
func (r *DaemonRepository) RemovePID() error
func (r *DaemonRepository) HasStopFile() bool
func (r *DaemonRepository) WriteStopFile() error
func (r *DaemonRepository) RemoveStopFile() error

// LogDir returns the log directory path. This is an intentional escape
// hatch: the Logger manages its own file handles, rotation, and
// compression, so it needs the directory path rather than a repository
// method for each log operation.
func (r *DaemonRepository) LogDir() string
```

Replaces: `filepath.Join(wolfcastleDir, "system", "wolfcastle.pid")` and similar scattered across daemon.go, pid.go, start.go, validate/engine.go, validate/fix.go.

### Store (existing)

Already follows the repository pattern. `MutateNode`, `MutateIndex`, `MutateInbox` are domain operations, not path operations. Stays as-is. Its `dir` field is set by `Identity.ProjectsDir(root)` during init.

### ScaffoldService

Creates the repositories' backing storage. Takes repositories as dependencies.

```go
type ScaffoldService struct {
    config  *config.Repository
    prompts *pipeline.PromptRepository
    daemon  *daemon.DaemonRepository
    root    string  // .wolfcastle/ root, for creating docs/artifacts/archive
}

// Init creates the full .wolfcastle/ directory structure, writes default
// config, extracts embedded prompts, and creates the engineer namespace.
// Creates docs/, artifacts/, and archive/ via raw os.MkdirAll (these
// directories have no repository because they have no resolution
// semantics). Also writes .gitignore.
func (s *ScaffoldService) Init(identity *config.Identity) error

// Reinit runs pending migrations, then regenerates base tier files
// and refreshes identity. Migration errors are non-fatal (the layout
// may already be current).
func (s *ScaffoldService) Reinit() error
```

The `projects/<namespace>/` directory is created by `Init`: it calls `os.MkdirAll(identity.ProjectsDir(s.root))` and writes the empty root index there.

### MigrationService

Handles layout and config migrations separately from scaffold. Split from `ScaffoldService` because migrations serve upgrading users (different audience, different error profile) and can be implemented and tested independently.

```go
type MigrationService struct {
    config *config.Repository
    root   string
}

// MigrateDirectoryLayout moves pre-ADR-077 flat directories (base/,
// custom/, local/, projects/, logs/) into system/. Also moves
// wolfcastle.pid and stop files. No-op if system/ already exists.
func (m *MigrationService) MigrateDirectoryLayout() error

// MigrateOldConfig moves pre-ADR-063 config files (root config.json
// to system/custom/config.json, config.local.json to system/local/config.json).
// Delegates to config.Repository for the actual file operations.
func (m *MigrationService) MigrateOldConfig() error
```

`Reinit` in `ScaffoldService` calls both migration methods before regenerating base files:
```go
func (s *ScaffoldService) Reinit() error {
    migrator := &MigrationService{config: s.config, root: s.root}
    // Migration errors are non-fatal: the layout may already be current.
    // Log but don't fail, so rescaffold proceeds even on a clean install.
    _ = migrator.MigrateDirectoryLayout()
    _ = migrator.MigrateOldConfig()
    // ... regenerate base, refresh identity
}
```

Replaces: `project.Scaffold`, `project.ReScaffold`, `project.WriteBasePrompts`, `project.migrateOldConfig`, `project.migrateToSystemLayout`.

## GitService

GitService is a service, not a repository. It wraps an external tool (the git binary) rather than managing stored data. It lives at the same organizational level as the repositories because the daemon depends on it the same way, but it has no tier resolution or storage semantics.

```go
package git

// Provider is the interface for git operations. The production
// implementation shells out to git; tests can provide stubs.
type Provider interface {
    CurrentBranch() (string, error)
    HEAD() string
    HasProgress(sinceCommit string) bool
    IsRepo() bool
    IsDirty(excludePaths ...string) bool
    CreateWorktree(path, branch string) error
    RemoveWorktree(path string) error
}

// Service implements Provider by shelling out to the git binary.
type Service struct {
    repoDir string
}

func NewService(repoDir string) *Service
```

`HasProgress` checks both `HEAD() != sinceCommit` (new commits) and `IsDirty(".wolfcastle/")` (uncommitted changes excluding `.wolfcastle/`). The `.wolfcastle/` exclusion is internal.

Replaces: `daemon.currentBranch`, `daemon.gitHEAD`, `daemon.checkGitProgress`.

## Directory Layout (internal to implementations)

```
.wolfcastle/
  system/                          ← tierfs root for Config/Prompt repos
    base/config.json               ← config.Repository.WriteBase
    base/prompts/*.md              ← PromptRepository.WriteBase
    base/prompts/classes/*.md      ← ClassRepository (via PromptRepository)
    base/rules/*.md                ← PromptRepository.WriteBase
    base/audits/*.md               ← PromptRepository.WriteBase
    custom/config.json             ← config.Repository.WriteCustom
    custom/prompts/*.md            ← user overrides (PromptRepository reads)
    custom/prompts/classes/*.md    ← user class overrides (ClassRepository reads)
    local/config.json              ← config.Repository.WriteLocal
    local/prompts/*.md             ← user overrides (PromptRepository reads)
    projects/<namespace>/          ← Store (created by ScaffoldService.Init)
    logs/                          ← DaemonRepository.LogDir (escape hatch)
    wolfcastle.pid                 ← DaemonRepository
    stop                           ← DaemonRepository
  docs/                            ← model output (raw os.MkdirAll by ScaffoldService, CLI writes)
  artifacts/                       ← model output (raw os.MkdirAll by ScaffoldService, CLI writes)
  archive/                         ← completed projects (raw os.MkdirAll by ScaffoldService, archive cmd)
  .gitignore                       ← written by ScaffoldService.Init
```

No caller outside the repositories constructs `system/` paths. `docs/`, `artifacts/`, and `archive/` are created by `ScaffoldService` via raw `os.MkdirAll` because they have no resolution semantics. The `archive` command also performs raw filesystem operations (moving directories); this is acceptable because archive is a simple move with no tier logic.

## Error Handling

Repositories wrap errors with the repository name and operation:

```go
return fmt.Errorf("config: loading base tier: %w", err)
return fmt.Errorf("prompts: resolving %q: %w", name, err)
return fmt.Errorf("daemon: reading PID file: %w", err)
```

Default behavior for missing data:
- `config.Repository.Load()`: missing tier files skipped; returns `Defaults()` merged with whatever exists.
- `PromptRepository.Resolve/ResolveRaw`: missing file returns a wrapped `os.ErrNotExist`.
- `ClassRepository.Resolve`: missing class prompt returns an error naming the key and tier directories searched.
- `DaemonRepository.ReadPID`: missing PID file returns a wrapped `os.ErrNotExist`.
- `DaemonRepository.HasStopFile`: missing file returns `false` (not an error).

## Context Assembly Refactor

`pipeline/context.go:buildIterationContext` is a 150-line function that knows about every field on `NodeState`, `Task`, `AuditState`, failure thresholds, deliverable lists, summary requirements, and breadcrumbs. When task classes land, it grows again. When any data model field changes, this function changes.

Each domain type owns its own context rendering:

```go
// In internal/state
func (t *Task) RenderContext() string { ... }
func (a *AuditState) RenderContext() string { ... }
func (ns *NodeState) RenderContext(taskID string) string { ... }
```

The builder becomes a compositor with repository dependencies:

```go
// In internal/pipeline
type ContextBuilder struct {
    prompts *PromptRepository
    classes *ClassRepository
}

func NewContextBuilder(prompts *PromptRepository, classes *ClassRepository) *ContextBuilder

func (cb *ContextBuilder) Build(nodeAddr string, ns *state.NodeState, taskID string, cfg *config.Config) string {
    var b strings.Builder
    b.WriteString(ns.RenderContext(taskID))

    if task := ns.FindTask(taskID); task != nil {
        b.WriteString(task.RenderContext())

        if task.Class != "" {
            if classPrompt, err := cb.classes.Resolve(task.Class); err == nil {
                b.WriteString("\n## Class Guidance\n\n")
                b.WriteString(classPrompt)
            }
        }
    }

    b.WriteString(ns.Audit.RenderContext())

    if cb.shouldIncludeSummary(ns, taskID, cfg) {
        b.WriteString(cb.renderSummaryRequired())
    }

    if task := ns.FindTask(taskID); task != nil && task.FailureCount > 0 {
        b.WriteString(cb.renderFailureContext(task, cfg))
    }

    return b.String()
}
```

`cfg` is passed to `shouldIncludeSummary` and `renderFailureContext` (both need config for thresholds). The `ContextBuilder` holds `prompts` and `classes` as struct fields; `cfg` is a parameter because it may change between calls (config reload).

### Where render methods live

| Type | Package | Method |
|------|---------|--------|
| `NodeState` | `internal/state` | `RenderContext(taskID string) string` |
| `Task` | `internal/state` | `RenderContext() string` |
| `AuditState` | `internal/state` | `RenderContext() string` |
| Failure headers | `internal/pipeline` | `ContextBuilder.renderFailureContext(task, cfg)` |
| Summary section | `internal/pipeline` | `ContextBuilder.renderSummaryRequired()` (uses PromptRepository) |
| Class prompt | `internal/pipeline` | `ContextBuilder.Build` (uses ClassRepository) |

## App Struct Refactor

Current:

```go
type App struct {
    WolfcastleDir string
    Cfg           *config.Config
    Resolver      *tree.Resolver
    Store         *state.Store
    Clock         clock.Clock
    Invoker       invoke.Invoker
    JSONOutput    bool
    Version       string
    Commit        string
}
```

After:

```go
type App struct {
    Config   *config.Repository
    Identity *config.Identity          // nil if identity not configured
    Prompts  *pipeline.PromptRepository
    Classes  *pipeline.ClassRepository
    Daemon   *daemon.DaemonRepository
    State    *state.Store         // nil if identity not configured
    Git      git.Provider
    Clock    clock.Clock
    Invoker  invoke.Invoker
    JSON     bool
    Version  string
}
```

```go
func (a *App) Init() error {
    root, err := findWolfcastleDir()
    if err != nil {
        return err
    }
    a.Config = config.NewRepository(root)
    a.Prompts = pipeline.NewPromptRepository(root)
    a.Daemon = daemon.NewDaemonRepository(root)
    a.Git = git.NewService(filepath.Dir(root))
    a.Classes = pipeline.NewClassRepository(a.Prompts)

    cfg, err := a.Config.Load()
    if err != nil {
        return err
    }

    id, err := config.IdentityFromConfig(cfg)
    if err != nil {
        // Identity not configured. State, Identity remain nil.
        // Commands that need identity call RequireIdentity().
        return nil
    }
    a.Identity = id
    a.State = state.NewStore(id.ProjectsDir(root), state.DefaultLockTimeout)
    a.Classes.Reload(cfg.TaskClasses)
    return nil
}

// RequireIdentity returns an error if identity is not configured.
// Commands that operate on the project tree should call this early.
func (a *App) RequireIdentity() error {
    if a.Identity == nil {
        return fmt.Errorf("identity not configured. Run 'wolfcastle init' first")
    }
    return nil
}
```

`Identity` and `State` are explicitly nil-able. Commands that need them call `RequireIdentity()` (which replaces the current `RequireResolver()`). Commands that don't need identity (`init`, `version`, `update`) skip it. No nil-pointer panics because the gate is explicit.

## Test Environment

```go
// NodeSpec describes a node for test setup.
type NodeSpec struct {
    Name     string
    Type     state.NodeType
    Tasks    []string   // task IDs (descriptions auto-generated as "Task: <id>")
    Children []NodeSpec // for orchestrators
}

// Leaf creates a leaf NodeSpec with the given task IDs.
func Leaf(name string, taskIDs ...string) NodeSpec {
    return NodeSpec{Name: name, Type: state.NodeLeaf, Tasks: taskIDs}
}

// Orchestrator creates an orchestrator NodeSpec with children.
func Orchestrator(name string, children ...NodeSpec) NodeSpec {
    return NodeSpec{Name: name, Type: state.NodeOrchestrator, Children: children}
}

// Environment holds pre-configured repositories backed by a temp dir.
// Construction order: temp dir -> repositories -> scaffold -> App.
// The App holds the same repository instances as the Environment fields.
type Environment struct {
    Root     string
    Config   *config.Repository
    Prompts  *pipeline.PromptRepository
    Classes  *pipeline.ClassRepository
    Daemon   *daemon.DaemonRepository
    State    *state.Store
    Git      git.Provider
    App      *cmdutil.App
}

// NewEnvironment creates a scaffolded .wolfcastle/ in a temp dir
// with default config, identity ("test"/"dev"), and an empty root index.
// Construction order: creates temp dir, constructs repositories from it,
// runs ScaffoldService.Init, constructs App from the same repositories.
func NewEnvironment(t *testing.T) *Environment

// WithConfig applies config overrides (deep-merged into base).
func (e *Environment) WithConfig(overrides map[string]any) *Environment

// WithProject creates a project with the given node structure.
// Writes node state files and updates the root index.
func (e *Environment) WithProject(name string, root NodeSpec) *Environment

// WithPrompt writes a prompt file to the base tier.
// relPath is relative to prompts/: WithPrompt("execute.md", content)
// writes to system/base/prompts/execute.md.
func (e *Environment) WithPrompt(relPath string, content string) *Environment

// WithRule writes a rule fragment to the base tier.
func (e *Environment) WithRule(name string, content string) *Environment
```

`NewEnvironment` builds repos first, then `App` from them. Both `env.Config` and `env.App.Config` point to the same `config.Repository` instance.

### Migration for test helpers

| Current | Replaced by |
|---------|-------------|
| `cmd/testhelper_test.go:newTestEnv` | `testutil.NewEnvironment(t)` |
| `internal/daemon/daemon_test.go:testDaemon` | `testutil.NewEnvironment(t)` + daemon construction |
| `internal/testutil/helpers.go:SetupWolfcastle` | `testutil.NewEnvironment(t)` |
| `test/integration/helpers_test.go` | `testutil.NewEnvironment(t)` |

## Resolver Consolidation

`tree.Resolver` currently owns:
- Namespace detection: moves to `config.Identity`
- `ProjectsDir()`: moves to `config.Identity.ProjectsDir(root)`
- `NodeStatePath(addr)`: moves to `Store`
- `NodeDir(addr)`, `NodeDefPath(addr)`, `TaskDocPath(addr, taskID)`: move to `Store`
- `RootIndexPath()`: moves to `Store`
- `LoadRootIndex()`: already delegated to `state.LoadRootIndex`

After migration, `Resolver` has no remaining responsibilities and can be removed.

## Migration Path

Ordered by value: test infrastructure first so every subsequent step is easier to validate.

1. **`internal/tierfs/`**: `FS` struct + `Resolver` interface. The foundation. No dependencies.
2. **`internal/testutil/Environment`**: test infrastructure built on `tierfs`. Provides `NewEnvironment(t)`, `NodeSpec` builders, `WithConfig`/`WithPrompt`/`WithRule` helpers. Initially wraps `tierfs.FS` directly; later steps add repository fields as repositories are built. Every step after this one writes tests using `Environment` instead of manual path construction.
3. **`internal/daemon/DaemonRepository`**: smallest repository. Validates the pattern against the test infra. No dependency on `tierfs` (constructs paths from `systemDir` directly).
4. **`internal/git/Service`** + `Provider` interface: no dependencies. Parallel with (3).
5. **`internal/config/Identity`** extraction: standalone type, no dependencies beyond existing `config` package. Parallel with (3, 4).
6. **`internal/config/Repository`**: depends on (1) for tier resolution. Uses `Environment` for tests.
7. **`internal/pipeline/PromptRepository`**: depends on (1). Uses `Environment` for tests.
8. **`internal/pipeline/ClassRepository`**: depends on (7). Needed for task-classes feature.
9. **Context render methods** on `NodeState`, `Task`, `AuditState` in `internal/state/`: no repository dependencies. Parallel with (3-8).
10. **`internal/pipeline/ContextBuilder`**: depends on (7, 8, 9). Replaces monolithic `buildIterationContext`.
11. **`internal/project/ScaffoldService`** + **`MigrationService`**: depends on (3, 6, 7). Absorbs scaffold.go and migration logic.
12. **Refactor `App` struct**: depends on (3, 4, 5, 6, 7, 8). Replace `WolfcastleDir`, `Resolver`, raw `Cfg` with repository references. Add `RequireIdentity()`.
13. **Remove `tree.Resolver`** and migrate remaining tests to `Environment`: depends on (5, 6, 12). Final cleanup.

Parallelizable groups:
- Group A: steps 1, 2 (foundation + test infra)
- Group B (depends on A): steps 3, 4, 5, 9 (independent repositories + render methods)
- Group C (depends on B): steps 6, 7 (tier-dependent repositories)
- Group D (depends on C): steps 8, 10, 11 (features that build on repositories)
- Group E (depends on D): steps 12, 13 (cleanup)

Each step is independently mergeable. During transition, old functions delegate to new repositories internally. Step 2 (`Environment`) evolves across the migration: it starts with just `tierfs` and gains repository fields as each repository is built.

## Decisions (promoted from open questions)

- **`docs/`, `artifacts/`, `archive/` do not get repositories.** They have no tier resolution, no merging, no complex lookup. The CLI commands that write to them (`spec create`, `adr create`, `archive add`) perform simple file operations. The audit system's directory walk for specs is a minor coupling that doesn't justify a repository. If this changes (e.g., spec versioning), revisit.
- **PromptRepository caches base tier content for the lifetime of the daemon process.** Base tier files are scaffold-generated and never change during a run. Custom and local tiers are read fresh each time (users may edit them mid-run). The cache is a `map[string][]byte` on the `PromptRepository` struct, populated lazily on first access per path. `Resolve` and `ResolveRaw` check the cache before calling `tiers.Resolve`. The cache is not in `tierfs.FS` (which stays stateless); the caching decision is a `PromptRepository` concern. Cache lifetime is per-process; the daemon constructs `PromptRepository` once at startup.
- **`git.Provider` is an interface.** `git.Service` is the production implementation. Tests inject a stub `Provider` that returns canned values, avoiding git process spawning in unit tests. Integration tests use `git.Service` with real repos.
