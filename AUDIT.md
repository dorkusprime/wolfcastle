# Audit Checklist

A structured, repeatable audit for the Wolfcastle codebase. Work through each section in order. Fix issues as you find them.

## 1. Correctness

Does the code do what it claims to do?

- [ ] State transitions follow the four-state model (not_started, in_progress, complete, blocked). No invalid transitions exist.
- [ ] State propagation: every parent's state is derivable from its children. Verify with property-based tests.
- [ ] Failure escalation thresholds (decomposition at 10, hard cap at 50, max depth 5) are enforced and configurable.
- [ ] Audit task invariant: every leaf has an audit task as its last item. No path can delete or reorder it.
- [ ] Marker parsing: every WOLFCASTLE_* marker is handled. No marker is silently dropped.
- [ ] Config merge semantics: deep merge for objects, full replacement for arrays, null deletion. Verify with edge cases.
- [ ] Navigation: depth-first traversal visits nodes in the correct order. Self-healing picks up interrupted tasks.
- [ ] Branch verification: the daemon refuses to commit if the branch changed underneath it.

## 2. Go Best Practices

Does the code follow the Go style guide and community conventions?

- [ ] Every package has a package-level doc comment.
- [ ] Every exported type, function, constant, and variable has a doc comment starting with the name.
- [ ] Errors are wrapped with `%w` where callers might inspect the chain. Error messages are lowercase with no trailing punctuation.
- [ ] Intentionally discarded errors use `_ =` explicitly. No bare `err` returns without context.
- [ ] No stuttering in names (e.g., `state.StateManager`). ID not Id. URL not Url.
- [ ] Receiver names are consistent within each type (not mixing `s` and `self`).
- [ ] `context.Context` is propagated where applicable (model invocation, daemon loop, HTTP-like flows).
- [ ] No goroutine leaks. Every goroutine has a clear termination path.
- [ ] `gofmt` clean. `go vet` clean. `golangci-lint run ./...` reports 0 issues.

## 3. Error Handling and Resilience

Can the system recover from failures gracefully?

- [ ] Every `os.MkdirAll`, `os.WriteFile`, `SaveNodeState`, `SaveRootIndex` call has its error checked.
- [ ] File I/O uses atomic writes (temp file + rename) where corruption on crash would be catastrophic.
- [ ] The daemon self-heals after crashes: in-progress tasks are detected and handled on restart.
- [ ] API retry uses exponential backoff with configurable limits. Context cancellation short-circuits retries.
- [ ] Stale PID files and stop files are detected and cleaned up.
- [ ] File locking handles stale locks (dead PID detection) and lock timeout.

## 4. Security

Is the system safe from injection and data integrity issues?

- [ ] Model invocation: commands and arguments come from config, not from model output. No shell injection path.
- [ ] Tree addressing: addresses are validated and sanitized. No path traversal (e.g., `../../etc/passwd` as a node address).
- [ ] File writes are scoped to `.wolfcastle/`. No code writes outside the expected directory structure.
- [ ] No secrets in state files, logs, or committed config. API keys stay in environment variables.
- [ ] Marker parsing does not evaluate model output as code. Markers are line-prefix string matches only.

## 5. Architecture and Structure

Is the codebase well-organized and decomposed?

- [ ] Package responsibilities are clear and single-purpose. No package is a grab-bag of unrelated concerns.
- [ ] No circular dependencies between packages.
- [ ] The daemon is decomposed into focused files (daemon.go, iteration.go, stages.go, markers.go, retry.go, propagate.go).
- [ ] Platform-specific code uses build tags (`_unix.go`, `_windows.go`), not runtime checks.
- [ ] The three-tier resolution system (base, custom, local) is implemented once and shared (not duplicated per use site).
- [ ] Config defaults are centralized in a single `Defaults()` function. No scattered default initialization.
- [ ] Prompt assembly, fragment resolution, and script reference injection follow the documented pipeline.

## 6. Documentation

Is everything documented and accurate?

- [ ] README matches the current implementation. No stale feature descriptions.
- [ ] Every ADR in `docs/decisions/INDEX.md` matches the current code. No ADRs describe features that were changed post-decision without amendment.
- [ ] Every spec in `docs/specs/` tracks the current implementation. Specs that describe aspirational behavior are flagged.
- [ ] `docs/humans/` pages are accurate: command flags, exit codes, consequences, and cross-references all match reality.
- [ ] `docs/humans/cli/` has a page for every command. No command exists without documentation.
- [ ] `AGENTS.md` critical rules are current and correct.
- [ ] `docs/agents/` guides match the current package structure and patterns.
- [ ] No broken markdown links. Anchor links (`#section-name`) point to real headings.
- [ ] Package path references are current (e.g., no references to `internal/inbox` or `internal/review` which were merged into `internal/state`).

## 7. Voice and Copy

Does the writing sound like Wolfcastle?

- [ ] README, human docs, error messages, and CLI help text follow `docs/agents/VOICE.md`.
- [ ] No LLM tropes (per `~/.claude/CLAUDE.md`): no "delve," "leverage," "robust," no em dashes, no sycophantic openings, no summary bows.
- [ ] Short sentences. Confidence. Violence as metaphor. Machines are simple; humans are the weird part.
- [ ] Technical accuracy is never sacrificed for personality. The voice dresses the facts, it doesn't replace them.

## 8. Testing

Is the test suite trustworthy?

- [ ] All tests pass with `go test -race ./...`. No race conditions.
- [ ] No flaky tests (time-dependent, order-dependent, platform-dependent without skip guards).
- [ ] Tests test behavior, not implementation details. Refactoring internals shouldn't break tests.
- [ ] Error paths are tested, not just happy paths. Every `if err != nil` should have a test that triggers it (where architecturally reachable).
- [ ] Table-driven tests are used where there are multiple similar cases.
- [ ] Test helpers use `t.Helper()`. Error messages in assertions are descriptive.
- [ ] Integration tests (`test/integration/`) exercise real command sequences.
- [ ] Smoke tests (`test/smoke/`) verify the binary builds and runs.
- [ ] Property-based propagation tests verify invariants with random tree mutations.
- [ ] Coverage is above 90% on Codecov. Gaps are justified (os.Exit, readline, process forking).

## 9. CI/CD and Infrastructure

Does the build pipeline catch problems?

- [ ] CI runs on every push: build, vet, gofmt, test (with race detector), lint, cross-compile.
- [ ] Coverage uploads to Codecov successfully. The badge shows a number.
- [ ] CodeQL security scanning runs on main pushes, PRs, and weekly.
- [ ] GoReleaser config matches `cmd/version.go` LDFLAGS (Version, Commit, Date).
- [ ] Makefile targets work: `make build`, `make test`, `make lint`.
- [ ] Pre-commit hook (`.githooks/pre-commit`) runs fmt, vet, build, lint.
- [ ] Integration and smoke tests run in CI with correct build tags.
- [ ] Dependencies are minimal and current. `go mod tidy` produces no changes.

## 10. Cross-Platform

Does the code compile and run on all targets?

- [ ] `GOOS=windows GOARCH=amd64 go build ./...` succeeds.
- [ ] `GOOS=linux GOARCH=amd64 go build ./...` succeeds.
- [ ] `GOOS=linux GOARCH=arm64 go build ./...` succeeds.
- [ ] `GOOS=darwin GOARCH=amd64 go build ./...` succeeds.
- [ ] `GOOS=darwin GOARCH=arm64 go build ./...` succeeds.
- [ ] Platform-specific code (`filelock_unix.go`, `filelock_windows.go`, `procattr_unix.go`, `procattr_windows.go`) provides equivalent behavior or documented degradation.
- [ ] Permission-based tests skip on Windows with `runtime.GOOS == "windows"`.

## 11. Code Coverage

Is the test suite comprehensive enough to trust?

- [ ] Weighted coverage (`go test -coverprofile=cover.out ./... && go tool cover -func=cover.out | tail -1`) is above 93%.
- [ ] Codecov reports above 90% (accounts for partial branch penalty).
- [ ] No internal/ package is below 85%.
- [ ] No cmd/ package is below 65%.
- [ ] Run `go tool cover -func=cover.out | awk` to find all functions below 80%. For each one, either write a test or document why it's untestable.
- [ ] Category A gaps (testable today) should have zero items. See `docs/coverage-roadmap.md`.
- [ ] Category B gaps (filesystem tricks) should be covered with chmod-based tests on Unix, skipped on Windows.
- [ ] Category C gaps (interface extraction) should be resolved by injecting mockable dependencies.
- [ ] Category E gaps (inherently untestable: os.Exit, readline, process forking) are documented and accepted.
- [ ] Every new function added since the last audit has test coverage.

## 12. Usability

Is it pleasant to use?

- [ ] Error messages tell you what went wrong and what to do about it.
- [ ] `--help` output is organized by command groups and includes examples.
- [ ] `--json` output is consistent across all commands (same envelope structure).
- [ ] Shell completions work for `--node` flags.
- [ ] `wolfcastle status` gives a clear picture of what's happening.
- [ ] `wolfcastle doctor` explains issues and offers fixes.
- [ ] The interactive unblock session has readline support (history, line editing, Ctrl+C/Ctrl+D).

## Running the Audit

For a full automated audit, use subagents scoped by area:

```
# One agent per section, or group related sections:
Agent 1: Sections 1-3 (correctness, Go practices, error handling)
Agent 2: Sections 4-5 (security, architecture)
Agent 3: Sections 6-7 (documentation, voice)
Agent 4: Sections 8-11 (testing, CI, cross-platform, coverage)
Agent 5: Section 12 (usability)
```

Each agent should read the relevant source files, check every item, and fix issues directly. Commit fixes with concise messages describing what was found and corrected.
