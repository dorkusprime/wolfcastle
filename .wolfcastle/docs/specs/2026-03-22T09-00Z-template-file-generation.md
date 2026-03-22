# Template-Based File Generation

## Problem

Several files are generated from string literals and `strings.Builder` chains scattered across Go source files:

- `scaffold_service.go`: `.gitignore`, READMEs (5+ files) from raw string literals
- `cmd/adr_create.go`: ADR markdown from `strings.Builder` with hardcoded section headers (Status, Date, Context, Decision, Consequences)
- `cmd/spec.go`: spec markdown from `fmt.Sprintf` with hardcoded structure
- `project_create.go`: audit.md from embedded data
- `cmd/task/add.go`: task markdown files from `strings.Builder`

This creates three problems:

1. **Testing burden.** Every string-building code path needs its own tests. Template loading can be tested once.
2. **No user customization.** Users can't change the ADR format, spec structure, or scaffold README content without modifying Go source.
3. **DRY violation.** The file-generation pattern (build string, write to path, handle error) is reimplemented at each call site.

## Solution

Move all generated file content into template files under `internal/project/templates/`. Load them through a shared template loader. Users override them via the three-tier system (`system/custom/templates/`, `system/local/templates/`).

### Template inventory

| Current location | Template file | Variables |
|-----------------|---------------|-----------|
| `scaffold_service.go` gitignore | `templates/scaffold/gitignore.tmpl` | none |
| `scaffold_service.go` READMEs | `templates/scaffold/readme-root.md.tmpl`, `readme-system.md.tmpl`, etc. | none (static content) |
| `cmd/adr_create.go` | `templates/artifacts/adr.md.tmpl` | `Title`, `Date`, `Body` |
| `cmd/spec.go` | `templates/artifacts/spec.md.tmpl` | `Title`, `Body` |
| `project_create.go` audit.md | `templates/artifacts/audit-task.md.tmpl` | `NodeName` |
| `cmd/task/add.go` task .md | `templates/artifacts/task.md.tmpl` | `ID`, `Title`, `Description`, `Body`, `Type`, `Class`, `Deliverables`, `Constraints`, `References`, `AcceptanceCriteria` |

### Template loading

`PromptRepository` in `internal/pipeline/repository.go` already provides three-tier resolution + `text/template` execution via `Resolve(name, ctx)`. It's currently hardcoded to the `prompts/` prefix, but the underlying `tierfs.Resolver` is generic.

Extend `PromptRepository` (or extract a general `TemplateRepository`) to support a `templates/` prefix alongside `prompts/`. Do NOT create a parallel loading system. The existing `Resolve` method already does tier resolution, template parsing, and execution. Artifact templates should use the same code path.

Each template receives a typed struct, not a `map[string]any`. This keeps the variables discoverable and type-safe.

### Shared render-to-file function

Add a convenience method that resolves a template, executes it, and writes the result atomically:

```go
func (r *TemplateRepository) RenderToFile(tmplName string, data any, destPath string) error
```

Every call site replaces its `strings.Builder` / `fmt.Sprintf` / `os.WriteFile` chain with one `RenderToFile` call. This consolidates template loading, execution, and file writing into the existing infrastructure.

### Migration

This is a refactor with no behavioral change. Every generated file should produce identical output before and after. Verify with snapshot tests: capture the output of each generation path before the refactor, then assert the template version matches byte-for-byte.

### User customization

After migration, users can:

- Change the ADR format (add a "Supersedes" section, change the header structure)
- Change the spec boilerplate
- Customize scaffold READMEs with project-specific content
- Modify the gitignore rules for new projects
- Change task markdown format

All by dropping a `.tmpl` file in `system/custom/templates/`.

### Documentation pass

After implementation:

- `docs/humans/configuration.md` (or new config pages): document the template override mechanism. Show how to customize ADR format, spec format, scaffold files.
- `docs/humans/cli/adr-create.md`, `spec-create.md`: mention that the output format is customizable via templates.
- `CONTRIBUTING.md`: update the "Adding a CLI Command" guide to mention templates for commands that generate files.

## What This Does Not Cover

- Prompt templates (already handled by the prompt assembly pipeline).
- State file format (JSON, not templated).
- Log file format (NDJSON, not templated).
