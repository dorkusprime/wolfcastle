# Audit

Verify all work in this node is complete, correct, and high-quality.

For leaf nodes, your scope is the files touched by tasks in this node.
For orchestrator nodes, your scope is everything touched by all descendant nodes.

## Phase 0: Read the AARs

Before touching any code, read the After Action Reviews (AARs) in the iteration context. Each task that ran before you produced an AAR with:
- **Objective**: what the task set out to do
- **What happened**: the actual outcome
- **Went well**: things worth preserving
- **Improvements**: things the task author flagged as suboptimal
- **Action items**: follow-ups the author identified

Pay particular attention to **Improvements** and **Action items**. These are the task authors' own flags about where quality may be thin. They are leads, not verdicts; verify each one against the code.

## Phase 1: Read the code

Read every file this node touched. Don't check boxes yet. Just read, and write down what you notice. What feels wrong? What would you question in a code review? What makes you uneasy? Follow your instincts before following the rubric.

Look for things like:
- A file read or database query that takes an unvalidated path or input
- A discarded error or ignored return value that could mask a real failure
- A function that's doing two unrelated things
- A race condition hiding behind "this probably won't happen concurrently"
- Dead code that nobody calls
- A test that tests the mock, not the behavior

Write your findings as a numbered list before moving to Phase 2.

## Phase 2: Rubric deep dive

Now work through each rubric section. The rubric may surface issues your initial read missed. Add any new findings to your list.

### Build and test verification

Run the project's build and test commands. Look for whatever build system the project uses (Makefile, go.mod, package.json, Cargo.toml, pyproject.toml, etc.).

- The project builds without errors
- All tests pass (include failing test name and error if not)
- No formatting violations (run the project's formatter; fix and commit if needed)
- Static analysis clean (run the linter if configured; new warnings from this node's work are findings)

**If tests cannot run due to environment issues (missing dependencies, private registries, unavailable tools):**

Check the project configuration for `require_tests`. If it is set to `"warn"` or `"skip"`, note the limitation in a breadcrumb and continue. If it is `"block"` or not set, you MUST file a gap:

    wolfcastle audit gap --node {node} "test environment not functional: {describe the specific error}"

Then emit WOLFCASTLE_BLOCKED. An audit that cannot verify test results must not pass.

A code-review-only audit is categorically weaker than a test-verified audit. AST parsing and structural review cannot catch cross-module wiring gaps, import-time side effects, or runtime type mismatches. Do not substitute code review for test execution.

### Correctness

For each file this node changed:

- Nil/null safety: every pointer, optional, or interface field is checked or initialized before use
- Error handling: every error or exception is returned with context, logged, or explicitly discarded with justification. Search for discarded errors in files this node wrote or modified (e.g. `_ =` in Go, bare `except: pass` in Python, unchecked promises in JS); each one needs a reason
- Edge cases: empty inputs, zero values, nil collections, boundary conditions
- Concurrency safety: shared mutable state is protected. Run the race detector if available
- Security: inputs from external sources (file paths, user input, model output) are validated. No path traversal, injection, or unescaped interpolation

### Code quality

- No duplication: search for copy-pasted logic across files
- No dead code: unused functions, variables, imports, commented-out blocks
- No overly complex functions: 50+ lines or 3+ levels of nesting should be decomposed
- Clear naming: no abbreviations that obscure meaning, no stuttering (a `config` module with a `ConfigManager` class)
- Minimal public surface: only expose what callers need

### Modularity and architecture

- Single responsibility: each file, function, type does one thing
- No circular dependencies
- Consistent patterns: similar problems solved the same way
- Clean interfaces: minimal method sets, no god interfaces

### Documentation

- Every non-obvious choice has an ADR in `.wolfcastle/docs/decisions/`
- Every contract (type, interface, package API) has a spec in `.wolfcastle/docs/specs/`
- Placeholder specs (title only, fewer than 10 lines) count as missing

## Phase 3: Weigh the findings

You now have a combined list from Phase 1 and Phase 2. For each finding, decide:

**Is it introduced by this node's work, or pre-existing?**

Check git blame, commit history, or whether the file was modified by any task in this node.

- Introduced by this node: could be a gap (blocks audit) or acceptable (document and move on)
- Pre-existing: record as an escalation with `wolfcastle audit escalate`. Escalations don't block the audit.

**For issues introduced by this node, is it worth remediating?**

- Security vulnerabilities, data loss, crash paths: always remediate
- Silent error swallowing on I/O operations: remediate
- Race conditions in production code: remediate
- Dead code, naming issues, minor style: fix it yourself during the audit if quick, otherwise accept
- Coverage ceilings from untestable code paths: document and accept
- Hypothetical improvements with no concrete evidence: discard

Record remediations as gaps with `wolfcastle audit gap`. Record escalations with `wolfcastle audit escalate`. Fix trivial issues directly and commit.

## Phase 4: Write the audit summary

Before emitting your verdict, write a summary with `wolfcastle audit breadcrumb`. This summary becomes the narrative core of the audit report. Make it worth reading.

Structure your summary as:

1. **Scope**: what you reviewed (files, packages, line count).
2. **Verdict rationale**: why you're passing or remediating. Name the strongest evidence for your decision.
3. **Findings**: each finding with its disposition (remediated, escalated, accepted, discarded) and why.
4. **Risks**: anything that's technically correct but fragile, or correct today but likely to break under future changes. These aren't gaps; they're notes for the next auditor.
5. **Quality assessment**: one paragraph on the overall quality of the work. Be honest. Good work deserves recognition. Weak work deserves candor.

```
wolfcastle audit breadcrumb --node <your-node> "Reviewed 4 files (312 lines). PASS. Two findings: (1) retry.go line 45 discards context error during backoff sleep, accepted because the next iteration checks ctx.Err() immediately. (2) Escalated: stale TODO in invoke.go:89 predates this node. Quality: clean implementation, good test coverage, the exponential backoff is well-factored."
```

## Verdicts

- **PASS**: No findings that warrant remediation. Escalations and accepted tradeoffs are fine. Emit WOLFCASTLE_COMPLETE.
- **REMEDIATE**: Concrete, verifiable issues introduced by this node that need fixing. Record each as a gap, then emit WOLFCASTLE_BLOCKED. Every gap must cite specific evidence (file, line, test output).

PASS is the expected outcome for well-executed work. Only REMEDIATE when you have evidence of real problems this node caused.
