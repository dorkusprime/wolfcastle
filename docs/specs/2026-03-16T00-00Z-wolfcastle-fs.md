---
status: DRAFT
---

# Wolfcastle Filesystem Abstraction

## Problem

Every package in the codebase constructs `.wolfcastle/` paths independently via `filepath.Join`. A directory restructure (ADR-077) required changing 106 files because paths are hard-coded at every call site. There is no single source of truth for where files live inside `.wolfcastle/`.

Packages that perform direct filesystem I/O against `.wolfcastle/`:
- `internal/config` (tier config loading)
- `internal/pipeline` (fragment resolution, prompt templates, script reference)
- `internal/project` (scaffold, rescaffold, migration)
- `internal/daemon` (logs, PID file, stop file, deliverable checks)
- `internal/validate` (stale PID/stop detection, orphan scanning)
- `internal/tree` (resolver, namespace, node paths)
- `internal/state` (StateStore, already partially abstracted)
- `cmd/*` (init, doctor, status, install, unblock preamble)
- `test/*` (dozens of test helpers constructing paths manually)

## Proposed Design

A `WolfcastleFS` interface that owns all `.wolfcastle/` I/O. Callers request operations by intent, not by path.

```go
// WolfcastleFS provides access to the .wolfcastle/ directory structure.
// All path construction and filesystem I/O is internal to the implementation.
// Callers never build .wolfcastle/ paths directly.
type WolfcastleFS interface {
    // Root returns the .wolfcastle/ directory path (escape hatch for
    // edge cases, but callers should prefer named methods).
    Root() string

    // Config
    LoadConfig() (*config.Config, error)

    // Tier resolution (rules, prompts, audits)
    ReadFragment(category, name string) (string, error)
    ListFragments(category string, include, exclude []string) ([]string, error)
    WriteBaseFile(relPath string, data []byte) error

    // Scaffold
    EnsureDirectories() error
    WriteGitignore() error

    // Daemon artifacts
    ReadPID() (int, error)
    WritePID(pid int) error
    RemovePID() error
    WriteStopFile() error
    RemoveStopFile() error
    HasStopFile() bool
    LogDir() string

    // Model outputs (docs/, artifacts/)
    WriteSpec(name string, data []byte) error
    WriteADR(name string, data []byte) error
    WriteArtifact(name string, data []byte) error
    ReadSpec(name string) ([]byte, error)
    ListSpecs() ([]string, error)
    ListADRs() ([]string, error)

    // State delegation (wraps StateStore)
    StateStore(namespace string) *state.StateStore
}
```

## Implementation

### DiskFS

The production implementation. Takes a root path and constructs all paths internally.

```go
type DiskFS struct {
    root string  // .wolfcastle/
}

func NewDiskFS(root string) *DiskFS { return &DiskFS{root: root} }

func (fs *DiskFS) systemDir() string { return filepath.Join(fs.root, "system") }
func (fs *DiskFS) LogDir() string    { return filepath.Join(fs.systemDir(), "logs") }
// ... etc
```

The directory layout is encoded once in these private methods. Public methods compose them.

### TestFS

Test helper that creates a temp directory and provides the same interface. Tests call `NewTestFS(t)` instead of building paths with `filepath.Join(t.TempDir(), ".wolfcastle", "system", "base", ...)`.

```go
func NewTestFS(t *testing.T) *DiskFS {
    t.Helper()
    dir := filepath.Join(t.TempDir(), ".wolfcastle")
    fs := NewDiskFS(dir)
    fs.EnsureDirectories()
    return fs
}
```

### Migration Path

1. Create `internal/wolffs/` package with the interface and `DiskFS` implementation.
2. Add `TestFS` helper.
3. Refactor one package at a time to accept `WolfcastleFS` instead of building paths. Start with `internal/pipeline` (fragment resolution) since it's the most scattered.
4. Update `internal/config` to use `WolfcastleFS.LoadConfig()` instead of walking tier files directly.
5. Update `internal/daemon` to use named methods for PID, stop, log paths.
6. Update `internal/project` scaffold to use `WolfcastleFS.EnsureDirectories()` and `WriteBaseFile()`.
7. Update `cmd/*` to receive `WolfcastleFS` via `App` instead of constructing paths.
8. Update tests last: replace `filepath.Join` path construction with `TestFS` methods.

Each step is independently mergeable. No big-bang rewrite.

## Consequences

- Directory layout changes in one file (`DiskFS` method implementations), not across the codebase.
- Tests become shorter and more readable: `fs.WriteBaseFile("prompts/execute.md", data)` instead of `os.WriteFile(filepath.Join(dir, "system", "base", "prompts", "execute.md"), data, 0644)`.
- The `system/` vs model-output boundary is enforced by the interface: there's a `WriteBaseFile` for system files and `WriteSpec`/`WriteArtifact` for model outputs. No method exposes raw system paths to callers.
- The `Resolver` and `StateStore` become subordinate to `WolfcastleFS` rather than operating independently.
- Fragment resolution, config loading, and tier merging become implementation details of `WolfcastleFS`, not separate package responsibilities.

## Open Questions

- Should `WolfcastleFS` also own the repo-dir side (deliverable checks, git operations)? Or is that a separate concern?
- Should `StateStore` be folded into `WolfcastleFS` or remain separate (it already has its own locking and mutation semantics)?
- Should prompt assembly move into `WolfcastleFS` (it combines fragments, templates, and context) or stay in `pipeline` with `WolfcastleFS` as a dependency?
