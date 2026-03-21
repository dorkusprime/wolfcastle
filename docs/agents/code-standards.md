# Code Standards

## Go Style

Follow the [Go Style Guide](https://google.github.io/styleguide/go/) and these project-specific conventions:

### Naming
- Package names: lowercase, single word, no underscores (`cmdutil`, not `cmd_util`)
- File names: `snake_case.go`, even when the Cobra command name uses hyphens (`fix_gap.go` → `fix-gap`)
- Exported types/functions: doc comments required (per Go convention)
- Package-level doc comments: required on at least one file per package

### Error Handling
- Wrap errors with `fmt.Errorf("context: %w", err)`. Always use `%w`, never `%v` for error wrapping
- Never silently ignore errors. Use `_ = os.Remove(...)` to explicitly mark intentional ignores
- Return errors to callers rather than logging-and-continuing, unless the operation is advisory
- Error messages: lowercase, no trailing punctuation, include context about what failed

### Output
- **All user-facing output** goes through `internal/output`:
  - `output.PrintHuman(format, args...)` for human-readable messages
  - `output.PrintError(format, args...)` for error messages to stderr
  - `output.Print(response)` for JSON envelope output
- **Never use `fmt.Println` or `fmt.Printf` for user-facing output.** The only exception is interactive prompts that require raw I/O (e.g., `fmt.Print("\n> ")` in the unblock session)
- The `--json` flag on the root command controls whether output is JSON or human-readable. Commands must respect this flag.

### Constants Over String Literals
- Use typed constants for status values:
  - `state.StatusNotStarted`, `StatusInProgress`, `StatusComplete`, `StatusBlocked`
  - `state.AuditPending`, `AuditInProgress`, `AuditPassed`, `AuditFailed`
  - `state.GapOpen`, `GapFixed`
  - `state.EscalationOpen`, `EscalationResolved`
  - `state.NodeOrchestrator`, `NodeLeaf`
- Test files may use string literals for readability, but source code must use constants

### File I/O
- State files: always use `state.SaveNodeState()` / `state.SaveRootIndex()` (atomic write)
- Temp files: use `os.CreateTemp()` with `.wolfcastle-tmp-*` prefix
- Directory creation: `os.MkdirAll(dir, 0755)` before writing
- File permissions: `0644` for files, `0755` for directories

### Testing
- Table-driven tests preferred
- Use `t.TempDir()` for filesystem tests
- Test files live alongside source (`foo.go` → `foo_test.go`)
- Shared test helpers live in `internal/testutil/`. Use these for common patterns (e.g., temp dir setup, state file creation)

### Formatting
- Run `gofmt -w .` before every commit
- Run `go vet ./...`. Must pass cleanly
- Run `golangci-lint run`. Configured in `.golangci.yml` (v2 format) per ADR-049. Linters: errcheck, ineffassign, staticcheck, govet, unused, misspell, nolintlint. Formatter: gofmt. CI enforces this as a hard gate.
