# ADR-033: Embedded Templates via go:embed

## Status
Accepted

## Date
2026-03-13

## Context
Wolfcastle ships default prompts, rule fragments, and audit scopes in `base/` (ADR-009). These files are regenerated from the installed version on `wolfcastle init` and `wolfcastle update`. The templates need to travel with the binary — they can't be downloaded at runtime or require a separate data directory.

## Decision

### Templates Embedded in Binary
All default templates are stored as files under `internal/project/templates/` and embedded into the Go binary using the `go:embed` directive:

```go
//go:embed all:templates
var Templates embed.FS
```

The embedded filesystem mirrors the `base/` directory structure:
```
internal/project/templates/
  prompts/execute.md
  prompts/expand.md
  prompts/file.md
  prompts/summary.md
  prompts/audit.md
  prompts/unblock.md
  prompts/script-reference.md
  rules/git-conventions.md
  rules/adr-policy.md
  audits/dry.md
  audits/modularity.md
  audits/decomposition.md
  audits/comments.md
```

### Extraction on Init/Update
`WriteBasePrompts()` walks the embedded FS and writes each file to `.wolfcastle/system/base/`:

```go
fs.WalkDir(Templates, "templates", func(path string, d fs.DirEntry, err error) error {
    relPath := strings.TrimPrefix(path, "templates/")
    destPath := filepath.Join(wolfcastleDir, "base", relPath)
    // mkdir + write
})
```

This runs during `wolfcastle init` (first setup) and `wolfcastle update` (refresh base).

### Why Not Runtime Embedding
The three-tier merge (ADR-009, ADR-018) requires files on disk — `base/`, `custom/`, `local/` are resolved at the filesystem level. Embedding them in the binary but never extracting would break the merge mechanism. The extraction step bridges embedded distribution with filesystem-based merging.

## Consequences
- Single binary distribution — no external data files needed
- Templates are maintainable as real Markdown files, not string literals in Go
- `wolfcastle update` always has the latest templates regardless of what's on disk
- Adding a new template is: create the file, rebuild the binary
- Template content is visible in the source tree for review
- Binary size increases slightly (~20KB for current templates) — negligible
