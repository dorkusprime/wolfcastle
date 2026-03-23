# tierfs.Resolver Contract

## Overview

Package tierfs provides three-tier file resolution for Wolfcastle's base < custom < local override hierarchy. It is the shared foundation that all domain repositories build on.

## Types

### Resolver (interface)

The primary abstraction. Consumers depend on this interface; production code uses the FS implementation.

### FS (struct)

Concrete implementation rooted at a directory (typically `.wolfcastle/system`). Created via `New(root string) *FS`.

## Methods

### Resolve(relPath string) ([]byte, error)
Returns content from the highest-priority tier containing the file. Checks local first, then custom, then base. Returns a wrapped `os.ErrNotExist` when no tier has the file. Permission errors from any tier propagate immediately (not swallowed).

### ResolveAll(subdir string) (map[string][]byte, error)
Collects every `.md` file in the given subdirectory across all tiers. Iterates base to local so higher-tier files overwrite lower-tier entries with the same filename. Keys are filenames, values are file contents. Directories and non-`.md` files are skipped. Returns an empty map (not an error) when the subdirectory is missing from all tiers.

### WriteBase(relPath string, data []byte) error
Writes data to the base tier, creating parent directories as needed. Uses 0755 for directories, 0644 for files.

### BasePath(subdir string) string
Returns the absolute path to a subdirectory within the base tier. Pure path computation, no I/O.

### TierDirs() []string
Returns absolute paths to all three tier directories in resolution order: base, custom, local.

## Error Behavior
- All errors are wrapped with a `tierfs:` prefix for identification
- `Resolve` wraps `os.ErrNotExist` so callers can use `errors.Is`
- Permission errors propagate, never silently ignored
- `ResolveAll` treats missing subdirectories as empty (continues to next tier)

## CachingResolver

`CachingResolver` wraps a `Resolver` with TTL-based caching for `Resolve` and `ResolveAll` calls. Write operations pass through to the underlying resolver and invalidate relevant cache entries.

```go
func NewCachingResolver(inner Resolver, ttl time.Duration) *CachingResolver
func (c *CachingResolver) Invalidate(relPath string)
func (c *CachingResolver) InvalidateAll()
```

`CachingResolver` implements `Resolver`. Production repositories (`NewRepository`, `NewPromptRepository`) wrap `tierfs.FS` in a `CachingResolver` with a 30-second TTL. Cached entries are copied on read to prevent callers from mutating shared data. `WriteBase` automatically invalidates the written path and all `ResolveAll` caches.

## Exported Constants and Variables

- `TierNames = []string{"base", "custom", "local"}`: canonical tier name list.
- `SystemPrefix = "system"`: the directory under the wolfcastle root containing tiers.
- `SystemTierPaths() []string`: returns `["system/base", "system/custom", "system/local"]`.

## Thread Safety
FS holds only an immutable root string. All operations are stateless filesystem reads/writes with no shared mutable state. Safe for concurrent use. CachingResolver uses a `sync.RWMutex` to protect its cache maps; safe for concurrent use.
