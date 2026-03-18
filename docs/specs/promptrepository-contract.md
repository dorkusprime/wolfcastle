# PromptRepository Contract

## Overview

`PromptRepository` in `internal/pipeline` owns three-tier prompt and fragment resolution. Callers request prompt templates by short name and fragments by category; the repository handles tier lookup, override semantics, include/exclude filtering, and optional Go `text/template` execution. It replaces the standalone `ResolveFragment`, `ResolveAllFragments`, and `ResolvePromptTemplate` functions in `internal/pipeline/fragments.go` with a struct-based design backed by `tierfs.Resolver`.

## Type

```go
type PromptRepository struct {
    tiers tierfs.Resolver
}
```

The `tiers` field provides file resolution across base, custom, and local tiers. PromptRepository holds no other state; it is a thin delegation layer over `tierfs.Resolver` with prompt-specific path conventions and template execution bolted on.

## Constructors

### NewPromptRepository(wolfcastleRoot string) *PromptRepository

Production constructor. Builds a `tierfs.FS` rooted at `filepath.Join(wolfcastleRoot, "system")` and wraps it. This is the only place `tierfs.New` is called for prompt resolution.

### NewPromptRepositoryWithTiers(tiers tierfs.Resolver) *PromptRepository

Testing constructor. Accepts an injected `tierfs.Resolver`, allowing tests to supply an in-memory or fixture-backed implementation without touching the filesystem.

## Methods

### Resolve(name string, ctx any) (string, error)

Resolves a prompt template by short name and optionally executes it as a Go `text/template`.

**Algorithm:**

1. Construct the tier-relative path: `"prompts/" + name + ".md"`.
2. Call `tiers.Resolve(path)` to retrieve content from the highest-priority tier that contains the file.
3. If `ctx` is nil, return the raw content as a string. No template processing occurs.
4. Parse the content with `template.New(name).Parse(content)`. If parsing fails, return the error wrapped with the `"prompts:"` prefix.
5. Execute the parsed template with `ctx` into a `strings.Builder`. If execution fails, return the error wrapped with the `"prompts:"` prefix.
6. Return the executed result.

The short-name convention keeps callers decoupled from file layout. A call like `Resolve("execute", data)` resolves `prompts/execute.md` through all three tiers, then renders it as a template against `data`.

### ResolveRaw(category string, name string) (string, error)

Resolves raw file content by category and filename with no template processing.

**Algorithm:**

1. Construct the tier-relative path: `category + "/" + name`.
2. Call `tiers.Resolve(path)` to retrieve content from the highest-priority tier.
3. Return the content as a string.

This method exists for content that should never be template-executed, or where the caller needs an explicit category rather than the implicit `"prompts/"` prefix that `Resolve` applies.

### ListFragments(category string, include []string, exclude []string) ([]string, error)

Collects all `.md` fragments in a category across tiers with override semantics, applies include/exclude filtering, and returns their contents.

**Algorithm:**

1. Call `tiers.ResolveAll(category)` to collect all `.md` files. The returned map uses filenames as keys and file contents as values. Higher-tier files silently replace lower-tier files with the same name.
2. Build an exclude set from the `exclude` slice for O(1) lookup.
3. Determine the filename ordering:
   - If `include` is non-empty, use it as the ordered filename list. This gives callers explicit control over fragment ordering. If an included name is not present in the resolved map, return an error (the include list is a contract, not a hint).
   - If `include` is empty, collect all filenames from the resolved map and sort them lexicographically.
4. Walk the ordered names. Skip any name present in the exclude set. For each remaining name, append its content (as a string) to the result slice.
5. Return the contents slice.

The include/exclude semantics match the existing `ResolveAllFragments` behavior: include is an explicit ordered allowlist, exclude is a denylist, and exclude is applied after include.

### WriteBase(relPath string, data []byte) error

Delegates directly to `tiers.WriteBase(relPath, data)`. Writes a single file to the base tier, creating parent directories as needed. Used to write generated content like the script reference prompt.

### WriteAllBase(templates fs.FS) error

Walks an `fs.FS` and writes each file to the base tier.

**Algorithm:**

1. Walk the provided `fs.FS` using `fs.WalkDir`.
2. Skip directories.
3. For each file, read its content from the `fs.FS`.
4. Call `tiers.WriteBase(path, content)` where `path` is the file's relative path within the walked filesystem.

This method is used during scaffold to seed default prompts from embedded templates. The `fs.FS` parameter is typically an `embed.FS` sub-tree containing the default prompt and rule files.

## Error Behavior

All errors returned by PromptRepository are prefixed with `"prompts:"` for consistent identification at call sites.

- **Resolve**: propagates `tiers.Resolve` errors (including wrapped `os.ErrNotExist` when no tier has the file). Template parse errors and template execution errors are wrapped with the prompt name for diagnostics.
- **ResolveRaw**: propagates `tiers.Resolve` errors directly (wrapped with prefix).
- **ListFragments**: missing tier directories are silently skipped by `tiers.ResolveAll`. Permission errors propagate. If an `include` entry names a file not found in any tier, returns an error.
- **WriteBase**: propagates errors from `tiers.WriteBase`.
- **WriteAllBase**: propagates errors from `fs.WalkDir`, file reads on the source `fs.FS`, and `tiers.WriteBase`. Stops on first error.

## Thread Safety

PromptRepository holds an immutable `tierfs.Resolver` reference and no mutable state. All read methods (`Resolve`, `ResolveRaw`, `ListFragments`) perform only filesystem reads through the resolver and are safe for concurrent use. `WriteBase` and `WriteAllBase` mutate the filesystem; concurrent writes to overlapping paths are not synchronized by the repository. Callers requiring concurrent writes must coordinate externally.

## Invariants

- `Resolve` always prepends `"prompts/"` and appends `".md"` to the short name. Callers never construct prompt paths.
- `ResolveRaw` performs no path decoration beyond joining category and name. It is the caller's responsibility to provide the full filename including extension.
- `ListFragments` returns contents, not paths. The caller receives an ordered slice of file bodies ready for concatenation into a prompt.
- `WriteAllBase` is idempotent: calling it twice with the same `fs.FS` produces the same base tier state.
- Template execution in `Resolve` uses standard `text/template` semantics. No custom template functions are registered.
- The repository does not cache. Each call reads from disk through the resolver. Custom and local tiers may be edited between iterations, so freshness is preferred over performance.
