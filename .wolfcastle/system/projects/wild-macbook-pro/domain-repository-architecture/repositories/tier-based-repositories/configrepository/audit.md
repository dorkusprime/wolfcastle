# Audit

Verify all work in this node is complete, correct, and high-quality. This audit covers both leaf nodes (verifying specific tasks) and orchestrator nodes (verifying the aggregate of all children).

For leaf nodes, your scope is the files touched by tasks in this node.
For orchestrator nodes, your scope is everything touched by all descendant nodes.

## 1. Completeness

- [ ] All tasks marked complete actually did what they claimed
- [ ] Deliverables exist and contain meaningful content
- [ ] No files were left in a broken state
- [ ] Breadcrumbs describe what was done and why
- [ ] No gaps remain open

## 2. Build and test verification

Run the project's build and test commands. If you don't know what they are, look for a Makefile, package.json, Cargo.toml, go.mod, or equivalent. At minimum:

- [ ] **The project builds without errors.** Run the build command. If it fails, the audit verdict is REMEDIATE.
- [ ] **All tests pass.** Run the test suite. If any test fails, the audit verdict is REMEDIATE. Include the failing test name and error in your gap report.
- [ ] **No formatting violations.** Run the project's formatter (gofmt, prettier, rustfmt, clang-format, etc.). If files need formatting, fix them and commit.
- [ ] **Static analysis clean.** Run the project's linter if one is configured. New warnings introduced by this node's work are audit findings.

## 3. Correctness

Read the code changes made by tasks in this node. For each file changed:

- [ ] **Nil/null safety.** Every pointer, optional, or interface field is checked or initialized before use. Struct constructors and Init methods set all fields. No nil dereference paths exist.
- [ ] **Error handling.** Every error is either returned with context, logged, or explicitly discarded with a comment explaining why. Bare `_ = someFunc()` without justification is a finding. Errors from I/O operations (file writes, network calls, directory creation) are never silently ignored.
- [ ] **Edge cases.** Empty inputs, zero values, nil collections, boundary conditions. Functions handle these gracefully rather than panicking.
- [ ] **Concurrency safety.** Shared mutable state is protected by locks or channels. No race conditions are introduced. If the project has a race detector, run it.

## 4. Code quality

- [ ] **No duplication.** Search for logic that appears in more than one place. If two functions render the same output, format the same data, or implement the same algorithm, one should delegate to the other. Copy-paste with minor variations is a finding.
- [ ] **No dead code.** Functions, variables, or imports that are never called or used. Commented-out code blocks. Unreachable branches. Remove them.
- [ ] **No overly complex functions.** Functions longer than 50 lines or with deeply nested conditionals (3+ levels) should be decomposed. High cyclomatic complexity is a finding.
- [ ] **Clear naming.** Types, functions, and variables have descriptive names. No abbreviations that obscure meaning. No stuttering (e.g., a package named `config` with a type named `ConfigManager`).
- [ ] **Minimal public surface.** Only export what callers need. Unexported helpers should stay unexported. If a type or function was exported but has no external callers, it should be unexported.

## 5. Modularity and architecture

- [ ] **Single responsibility.** Each file, function, and type does one thing. Files that mix unrelated concerns are a finding.
- [ ] **No circular dependencies.** Packages/modules do not import each other. If a dependency needs to flow both ways, an interface should break the cycle.
- [ ] **Consistent patterns.** Similar problems are solved the same way throughout the codebase. If a new pattern was introduced, verify it doesn't contradict an existing one without good reason.
- [ ] **Clean interfaces.** Interfaces have the minimal set of methods their consumers need. No "god interfaces" with 10+ methods when callers use 2.

## 6. Documentation: ADRs (WHY) and Specs (WHAT/HOW)

ADRs and specs together explain the system. Missing documentation is a REMEDIATE finding.

- [ ] **Every choice has an ADR.** Read the code changes. For each place where the developer chose between alternatives (concrete type vs interface, caching strategy, error handling approach, sync vs async, package structure), an ADR should exist in `.wolfcastle/docs/decisions/`. If a reasonable developer would ask "why was it done this way?" and there's no ADR answering that question, the verdict is REMEDIATE. Create the ADR yourself if you can determine the reasoning from the code; otherwise record it as a gap.
- [ ] **Every contract has a spec.** If a task created a type, interface, or package that other code depends on, a spec should exist in `.wolfcastle/docs/specs/` documenting: what it does, its methods/API, error behavior, and usage patterns. Placeholder specs (title only, fewer than 10 lines) count as missing. Delete placeholders and create real specs.
- [ ] **Specs and ADRs are in the right place.** Specs in `.wolfcastle/docs/specs/`, ADRs in `.wolfcastle/docs/decisions/`, research in `.wolfcastle/artifacts/`.

## Verdicts

After completing all checks:

- **PASS**: Everything checks out. No findings. Emit WOLFCASTLE_COMPLETE.
- **REMEDIATE**: You found concrete, verifiable issues. Record each one as an audit gap with `wolfcastle audit gap`, then emit WOLFCASTLE_BLOCKED with a summary of what needs fixing. Every remediation must cite specific evidence (file, line, or test output). No hypothetical improvements, style preferences, or "could-be-betters."

PASS is the expected outcome for well-executed work. Only REMEDIATE when you have evidence.
