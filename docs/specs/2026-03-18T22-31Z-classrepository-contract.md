# ClassRepository Contract

## Overview

`ClassRepository` in `internal/pipeline` owns task class prompt resolution. It sits atop `PromptRepository`, adding class-specific semantics: a goroutine-safe class list, hierarchical key fallback, validation of prompt availability, and sorted listing. Where `PromptRepository` resolves arbitrary prompt files by path, `ClassRepository` maps a class key like `"typescript/react"` to the right prompt file(s) through a defined fallback chain.

## Type

```go
type ClassRepository struct {
    prompts *PromptRepository
    mu      sync.RWMutex
    classes map[string]config.ClassDef
}
```

The `prompts` field delegates all file resolution to `PromptRepository.ResolveRaw` and `PromptRepository.ListFragments`. The `mu` field guards the `classes` map for concurrent access: `Reload` acquires a write lock; `Resolve`, `List`, and `Validate` acquire read locks. The `classes` map holds the current class definitions from config (using `config.ClassDef`), keyed by class name.

## Constructor

### NewClassRepository(prompts *PromptRepository) *ClassRepository

Creates a `ClassRepository` that delegates prompt resolution to the given `PromptRepository`. The internal class map starts empty. Callers must call `Reload` to populate it from a loaded config before `Resolve`, `List`, or `Validate` will return meaningful results.

## Methods

### Reload(classes map[string]config.ClassDef)

Replaces the internal class map with the provided definitions. Goroutine-safe: acquires a write lock on `mu`. The daemon calls this once at startup after loading config, and again if config is reloaded. The map is stored by reference; the caller should not mutate it after passing it in.

### Resolve(key string) (string, error)

Returns the behavioral prompt content for a class key by resolving it through `PromptRepository`. Goroutine-safe: acquires a read lock on `mu` for the class map lookup.

**Algorithm:**

1. Acquire read lock. Check that `key` exists in the `classes` map. If not, return an error indicating the key is not a configured class.
2. Attempt `prompts.ResolveRaw("prompts/classes", key+".md")`. If the file exists, record its content and note the resolved key.
3. If the file does not exist (`os.ErrNotExist`), compute the parent key by stripping the last path segment: for `"typescript/react"`, the parent is `"typescript"`; for `"lang-go"`, the parent is `"lang"` (strip after the last hyphen). The separator is `/` for hierarchical keys and `-` for hyphenated keys.
4. Attempt `prompts.ResolveRaw("prompts/classes", parentKey+".md")`. If the file exists, record its content and note the parent as the resolved key.
5. If neither file exists, return an error describing the key and the resolution attempts made.
6. **Subdirectory fragment assembly:** Call `prompts.ListFragments("prompts/classes/"+resolvedKey, nil, nil)` to collect any supplementary `.md` files in a subdirectory matching the resolved key. If fragments are found, append them (newline-joined) to the main prompt content. A missing or unreadable subdirectory is silently ignored; the main file content is always sufficient on its own.

The fallback is one level deep. `"typescript/react"` falls back to `"typescript"`, but `"typescript"` does not fall back further. There is no catch-all default prompt file.

### List() []string

Returns all class keys from the loaded config, sorted lexicographically. Goroutine-safe: acquires a read lock on `mu`. Returns an empty slice (not nil) if no classes are loaded.

### Validate() []string

Checks every configured class key for the existence of a corresponding prompt file in at least one tier. Returns a slice of class keys whose prompts are missing from all tiers. If every class has a resolvable prompt, returns an empty slice.

**Algorithm:**

1. Acquire read lock on `mu`.
2. For each key in the `classes` map, attempt `prompts.ResolveRaw("prompts/classes", key+".md")`.
3. If the file is not found, also attempt the parent key fallback (same logic as `Resolve`).
4. If neither the exact key nor the parent key resolves to a file, add the key to the missing list.
5. Sort the missing list lexicographically and return it.

This method is intended for daemon startup validation. The daemon can log warnings for missing prompts without blocking startup, since a missing prompt degrades gracefully (tasks with unresolvable classes get no class section in their assembled prompt).

## Error Behavior

All errors returned by ClassRepository are prefixed with `"classes:"` for consistent identification at call sites.

- **Resolve**: returns a prefixed error when the key is not in the configured class map. Returns a prefixed error describing both attempted paths when neither the exact key nor the parent key resolves to a file. Propagates `PromptRepository` errors (permission errors, I/O failures) for cases other than `os.ErrNotExist`.
- **List**: does not return errors. An empty class map yields an empty slice.
- **Validate**: does not return errors. I/O failures during resolution are treated the same as missing files for validation purposes (the class is reported as missing).

## Thread Safety

`ClassRepository` uses `sync.RWMutex` to protect the `classes` map. `Reload` takes a write lock; `Resolve`, `List`, and `Validate` take read locks. The underlying `PromptRepository` is itself safe for concurrent reads (immutable resolver, no mutable state). This means multiple goroutines can call `Resolve` concurrently without contention beyond the shared read lock on the class map.

The daemon should call `Reload` once at startup (before spawning iteration goroutines) or during a config reload pause. Calling `Reload` while `Resolve` calls are in flight is safe (the write lock blocks until readers finish) but will briefly stall active resolutions.

## Invariants

- `Resolve` delegates to `PromptRepository.ResolveRaw("prompts/classes", ...)` for the main prompt file and `PromptRepository.ListFragments("prompts/classes/"+resolvedKey, ...)` for supplementary fragments. ClassRepository never constructs tier paths or reads files directly.
- The fallback chain is exactly one level deep: exact key, then parent key, then error. No recursive or multi-level fallback.
- `List` returns keys sorted lexicographically. The order is deterministic regardless of map iteration order.
- `Validate` uses the same fallback logic as `Resolve`. A class that `Resolve` would successfully serve will not appear in `Validate`'s missing list.
- The repository does not cache prompt content. Each `Resolve` call reads through `PromptRepository`, which in turn reads through `tierfs.Resolver`. Caching decisions belong to `PromptRepository`, not here.
- An empty class map (before `Reload` or after `Reload({})`) causes `Resolve` to return an error for any key, `List` to return an empty slice, and `Validate` to return an empty slice.
