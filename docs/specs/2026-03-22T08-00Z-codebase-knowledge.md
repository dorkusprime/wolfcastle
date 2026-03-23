# Codebase Knowledge

## Problem

Each agent invocation starts with clean context and a task description. It has specs, ADRs, and AARs from prior work. But none of these capture the informal, accumulating knowledge that developers build up by working in a codebase: build quirks, undocumented conventions, things that look wrong but are intentional, test patterns that work well, dependencies between modules that aren't obvious from the code.

Ralph loops solve this with an `AGENT.md` file that gets updated each iteration and read at the start of the next. The agent learns something ("the config loader silently drops null values" or "the payment module tests need a running Redis") and records it. Future agents benefit.

Wolfcastle's ADRs record design decisions. Specs record contracts. AARs record per-task retrospectives. Breadcrumbs record execution events. None of these serve as a living, growing "what you need to know about this codebase" document.

## Solution

A new artifact type: **codebase knowledge files**. Markdown documents that accumulate codebase-specific knowledge across tasks. Agents read them at the start of every task and update them when they learn something new.

### File location

```
.wolfcastle/docs/knowledge/
  <namespace>.md     # one file per engineer namespace (e.g., wild-macbook-pro.md)
```

Each namespace gets its own file. Knowledge from different engineers running on different machines stays separate (different machines may have different build environments, paths, tooling versions). A shared `common.md` can hold knowledge that applies to all engineers.

### Content structure

Knowledge files are free-form markdown. No rigid schema. The agent appends entries as it discovers them. Entries should be:

- **Concrete**: "the integration tests require `docker compose up` before running" not "the tests need some setup."
- **Durable**: information that will still be true next week. Not "I'm currently working on the auth module."
- **Non-obvious**: things you wouldn't learn from reading the README, CONTRIBUTING.md, or the code itself.

Example entries:

```markdown
## Build and Environment

- The `make test` target runs with `-short` by default. Full integration tests need `make test-integration` which requires a running Postgres on port 5432.
- Go 1.26 changed the loop variable semantics. All `for range` closures in this codebase rely on the new behavior. Do not add `v := v` captures.

## Architecture Quirks

- The `state.Store` serializes all mutations through a file lock. Never hold a Store lock while calling `pipeline.AssemblePrompt` because prompt assembly reads from the tierfs cache, which also acquires locks.
- `daemon.RunOnce` uses a goto statement at line 569. This is intentional and reviewed. Do not refactor it.

## Testing Patterns

- Tests that need a git repo should use `initTestGitRepo(t, dir)` from `edge_cases_test.go`. Do not call `git init` directly.
- Property-based tests in `propagation_property_test.go` use `testing/quick` with custom generators. These are slow. Run them with `-count=1` during development.

## Known Issues

- The `selfupdate` package is a stub. It reports "unavailable" honestly. Do not try to implement it.
- Archived nodes' state files live under `.archive/` but the validation engine's orphan scan skips that directory. If you move archive files, validation will break.
```

### Context injection

The `ContextBuilder` reads the knowledge file for the current namespace and injects it into the iteration context under a `## Codebase Knowledge` section. It appears after the universal/class guidance and before the audit context. If no knowledge file exists, the section is omitted.

The file is read every iteration (not cached) so updates from one task are immediately visible to the next.

### Agent updates

The execute prompt instructs the agent to update the knowledge file when it discovers something non-obvious about the codebase. The instruction belongs in `execute.md` (system mechanics), not in class prompts:

"If you discover something about this codebase that would be useful to future tasks and isn't documented elsewhere (in the README, CONTRIBUTING.md, specs, or ADRs), append it to the codebase knowledge file using `wolfcastle knowledge add \"<entry>\"`."

### CLI

A new command: `wolfcastle knowledge add "<entry>"`. Appends a markdown bullet to the current namespace's knowledge file. Creates the file if it doesn't exist. The command handles formatting (adds a `- ` prefix if missing, appends a newline).

`wolfcastle knowledge show` displays the current namespace's knowledge file.

`wolfcastle knowledge edit` opens it in `$EDITOR`.

### Relationship to other artifacts

| Artifact | Scope | Lifespan | Purpose |
|----------|-------|----------|---------|
| ADR | One decision | Permanent | Why we chose X over Y |
| Spec | One component | Until superseded | What X does and how |
| AAR | One task | Archived with task | What happened during this task |
| Breadcrumb | One event | Archived with node | Timestamped execution record |
| Knowledge | Entire codebase | Grows indefinitely | What you need to know to work here |

### Size management

Knowledge files have a configurable token budget (`knowledge.max_tokens`, default 2000). The entire file is injected into every task's context, so the budget directly controls context usage.

`wolfcastle knowledge add` checks the file size before appending. If the new entry would push the file over budget, the command fails with an error: "Knowledge file exceeds budget (N/M tokens). Run `wolfcastle knowledge prune` to review and consolidate." No entry is silently lost to truncation. Every entry in the file gets injected.

When the file exceeds budget, the daemon creates a maintenance task: "Review the codebase knowledge file, remove stale entries, consolidate related entries, and bring it under budget." The agent prunes with full context of the file and the current codebase. This is an auditable activity with a commit and an AAR, not a silent background operation.

The `wolfcastle knowledge prune` command opens the file in `$EDITOR` for manual pruning, or can be run by the daemon's maintenance task. Git history preserves anything removed.

### Documentation pass

After implementation, review and update all docs that reference context or knowledge:

- `docs/humans/how-it-works.md`: add a section on codebase knowledge files, how they fit into the execution context alongside ADRs/specs/AARs.
- `docs/humans/configuration.md` (and new config pages): document the `knowledge` config section (`max_tokens`).
- `docs/humans/cli/`: add pages for `wolfcastle knowledge add`, `wolfcastle knowledge show`, `wolfcastle knowledge prune`.
- `AGENTS.md` and `docs/agents/`: explain knowledge files as a context source, when to update them.
- Execute prompt (`execute.md`): add the instruction for agents to update knowledge when they discover something non-obvious.
- README: mention knowledge files in the context section (alongside ADRs, specs, AARs).

## What This Does Not Cover

- Automatic knowledge extraction from AARs (potential future feature: after a task, the daemon could suggest promoting AAR lessons to the knowledge file).
- Cross-namespace knowledge sharing (engineers can read each other's files via the filesystem, but there's no merge mechanism).
- Knowledge categories or tags (free-form markdown is sufficient; structure emerges from usage).
