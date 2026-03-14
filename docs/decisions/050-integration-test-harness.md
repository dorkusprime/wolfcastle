# ADR-050: Integration Test Harness via CLI Dispatch

## Status
Accepted

## Date
2026-03-14

## Context
The project has strong unit test coverage in `internal/` but zero tests for the `cmd/` layer (~5,100 lines). Unit tests verify individual packages in isolation but cannot catch integration failures — wrong flag names, missing `RequireResolver()` calls, JSON envelope mismatches, or state propagation errors that only manifest when commands execute in sequence. This gap makes the `cmd/` layer the project's most significant testing blind spot.

An integration test pattern built around CLI dispatch — exercising complete workflows through the actual command entry point, with temp-directory isolation and injectable I/O — catches the wiring bugs that unit tests structurally cannot reach.

## Decision

Adopt an integration test pattern built around Cobra's `Execute()` entry point:

### Test Infrastructure

A `test/integration/helpers_test.go` file provides reusable test infrastructure:

- **`testApp(t *testing.T) (*cmdutil.App, string)`** — Creates a temp directory, initializes `.wolfcastle/` with a valid config, and returns an `App` context pointing at it. Cleans up on test completion.
- **`run(t *testing.T, dir string, args ...string) string`** — Executes wolfcastle as a subprocess (or via `rootCmd.Execute()` with reset) in the given directory, captures stdout, fails the test on non-zero exit.
- **`runExpectError(t *testing.T, dir string, args ...string) string`** — Same but expects failure; fails the test on exit 0.
- **`runJSON(t *testing.T, dir string, args ...string) output.Response`** — Runs with `--json`, unmarshals the JSON envelope.
- **`loadRootIndex(t *testing.T, dir string) *state.RootIndex`** — Reads root index from disk for assertions.
- **`loadNode(t *testing.T, dir, addr string) *state.NodeState`** — Reads node state from disk.

### Test Pattern

Tests follow a sequential-command pattern that mirrors real-world usage:

```go
func TestProjectLifecycle(t *testing.T) {
    dir := t.TempDir()
    run(t, dir, "init")
    run(t, dir, "project", "create", "my-feature")
    run(t, dir, "task", "add", "--node", "my-feature", "implement API")
    run(t, dir, "task", "claim", "--node", "my-feature/task-1")
    run(t, dir, "task", "complete", "--node", "my-feature/task-1")

    idx := loadRootIndex(t, dir)
    if idx.Nodes["my-feature"].State != state.StatusComplete {
        t.Errorf("expected complete, got %s", idx.Nodes["my-feature"].State)
    }
}
```

### State Corruption Testing

Tests can corrupt state directly on disk and verify that `doctor --fix` repairs it:

```go
func TestDoctorFixesMissingAuditTask(t *testing.T) {
    dir := t.TempDir()
    run(t, dir, "init")
    run(t, dir, "project", "create", "my-project")
    // Corrupt: remove audit task from node state
    ns := loadNode(t, dir, "my-project")
    ns.Tasks = ns.Tasks[:len(ns.Tasks)-1] // remove last (audit) task
    saveNode(t, dir, "my-project", ns)
    // Fix and verify
    run(t, dir, "doctor", "--fix")
    ns = loadNode(t, dir, "my-project")
    if !ns.Tasks[len(ns.Tasks)-1].IsAudit {
        t.Error("expected audit task restored as last task")
    }
}
```

### Build Tag

Integration tests use `//go:build integration` so `go test ./...` remains fast for local development. CI runs them explicitly.

## Consequences
- Integration tests catch the wiring bugs that unit tests miss (flag parsing, command sequencing, output formatting)
- The sequential-command pattern mirrors real-world usage, providing high-confidence coverage
- Direct disk verification (loading state files after commands) validates the full mutation→save→propagate pipeline
- Build tag separation keeps local `go test ./...` fast
- Target: 50+ integration test cases before beta, covering all critical command paths
