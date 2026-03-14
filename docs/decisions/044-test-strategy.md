# ADR-044: Test Strategy — Unit, Integration, and Smoke

**Status:** Accepted

**Date:** 2026-03-14

## Context

The `internal/` packages have strong unit test coverage (`state`, `tree`,
`config`, `pipeline`, `validate` are all well-tested). But the entire `cmd/`
layer (~5,100 lines across 8 packages) has zero tests, and `internal/invoke`
has no tests despite handling subprocess execution. The current test suite
validates correctness of individual components but cannot detect integration
failures — wrong flag names, missing `RequireResolver()` calls, JSON envelope
mismatches, or subprocess behavior on different platforms.

## Decision

Three test tiers, each with distinct scope and execution characteristics:

**Tier 1: Unit tests** (existing, expand)

- Continue the current pattern: `_test.go` files alongside source,
  table-driven where possible, `t.TempDir()` for filesystem tests.
- Add tests for `internal/invoke` using `exec.Command` test patterns (test
  binary that echoes stdin, exits with configurable code).
- Add tests for `cmd/cmdutil` (`App.LoadConfig`, `App.PropagateState`,
  completions).
- Use a shared test helper package at `internal/testutil` for common
  operations: `writeTestJSON(t, path, v)`, `setupTestTree(t)`,
  `newTestApp(t)`.

**Tier 2: Integration tests** (new)

- A new `test/integration/` directory with tests that exercise full command
  paths against a real `.wolfcastle/` directory.
- Each test: creates a temp dir, runs `wolfcastle init`, then exercises a
  sequence of commands via `exec.Command`.
- Validates: exit codes, JSON output structure, state file mutations, file
  creation/deletion.
- Integration tests are tagged with `//go:build integration` so they don't
  run on every `go test ./...`.
- CI runs them with `go test -tags integration ./test/integration/...`.
- Cover the critical paths: init → project create → task add → task claim →
  task complete → status; daemon start/stop lifecycle; audit approve/reject
  workflow; doctor --fix.

**Tier 3: Smoke tests** (new)

- A small set of tests that build the binary and run it with `--help`,
  `version`, and `init` in a temp dir.
- Validates that the binary compiles, runs, and produces expected output.
- Lives in `test/smoke/`.
- Runs on every CI push (fast — under 10 seconds).

**Coverage targets** (aspirational, not gated):

- `internal/`: 85%+
- `cmd/`: 60%+ (integration tests cover most of this indirectly)
- Overall: 75%+

## Consequences

- Integration tests catch the class of bugs that unit tests miss (flag
  wiring, output formatting, command sequencing).
- Smoke tests provide a fast sanity check that the binary works at all.
- Build tag separation means `go test ./...` stays fast for local
  development.
- Shared test helpers reduce duplication as the test suite grows.
- Coverage targets are aspirational guideposts, not hard gates — prevents
  gaming.
