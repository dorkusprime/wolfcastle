# Task Classes

> **DRAFT. NOT ACCEPTED.** This spec describes a classification system for tasks that routes each task to a behavioral prompt tailored to its nature. It does not propose adoption. It maps the terrain so we can decide whether to march.

Tasks are not all the same shape. Writing Go code requires different instincts than researching POS systems or drafting documentation. Today, every task gets the same execute prompt regardless of what it actually involves. Task classes fix that: the intake model classifies each task, and the daemon selects a behavioral prompt that tells the execute model how to think, not what tools it has.

## Governing ADRs

- ADR-063: Three-Tier Configuration (class definitions merge across tiers)
- ADR-066: Scoped Script References (AllowedCommands per stage, unchanged by classes)
- ADR-067: Terminal Markers Only (class prompts don't change the marker protocol)
- ADR-069: Task Deliverables (deliverable verification is class-agnostic)

---

## Core Concepts

### What a class is

A class is a behavioral modifier. It provides a `.md` prompt that is injected into the assembled system prompt alongside (not replacing) the execute stage prompt, script reference, and iteration context. The behavioral prompt tells the model what kind of work this is and how to approach it.

A class does NOT change:
- Available tools or allowed commands
- The terminal marker protocol (COMPLETE/YIELD/BLOCKED)
- Deliverable verification logic
- State transitions or propagation

A class CAN change:
- The behavioral prompt section (required)
- The model used for execution (optional override)

### What a class is not

Classes are not capability gates. A "go-coding" task still has access to web search. A "research" task can still write files. The behavioral prompt shapes the model's approach, priorities, and quality standards; it doesn't restrict its toolbox.

---

## Config Structure

Classes are defined as an object in the config, keyed by class name. Object keys merge cleanly across the three-tier config system (base < custom < local). Users can add new classes in `custom/config.json` or `local/config.json` without touching the defaults.

```json
{
  "task_classes": {
    "go-coding": {
      "description": "Writing or modifying Go source code"
    },
    "typescript-coding": {
      "description": "Writing or modifying TypeScript/JavaScript source code"
    },
    "architecture": {
      "description": "System design, ADRs, decomposition, dependency analysis",
      "model": "heavy"
    },
    "research": {
      "description": "Information gathering, comparison, analysis",
      "model": "light"
    },
    "writing": {
      "description": "Documentation, specs, guides, prose",
      "model": "light"
    },
    "design": {
      "description": "UI/UX design, wireframes, interaction patterns"
    },
    "audit": {
      "description": "Verification and review of completed work"
    }
  }
}
```

The behavioral prompt file is derived from the class key: `classes/<key>.md`, resolved through the three-tier system. No `prompt_file` field needed.

### Field definitions

| Field | Required | Description |
|-------|----------|-------------|
| `description` | Yes | One-line description shown to the intake model so it can classify accurately |
| `model` | No | Model key override. If set, this class uses a different model than the execute stage default. Must reference a key in the top-level `models` map. |

### Validation

At daemon startup, the config loader validates:
1. Every class has a non-empty `description`.
2. If `model` is set, it references a valid key in `config.models`.
3. The prompt file `classes/<key>.md` resolves to an existing file in at least one tier.

Unknown classes on tasks (e.g., from a hallucinating intake model) are caught at `task add` time: the CLI rejects `--class` values not present in the config's `task_classes` map.

---

## Task Struct

Add a `Class` field to the Task struct:

```go
type Task struct {
    ID                 string            `json:"id"`
    Title              string            `json:"title,omitempty"`
    Description        string            `json:"description"`
    Class              string            `json:"class,omitempty"`
    State              NodeStatus        `json:"state"`
    // ... rest unchanged
}
```

### CLI

```
wolfcastle task add "Implement auth middleware" --node my-project --class go-coding
wolfcastle task add "Research POS systems" --node pizza-docs --class research --deliverable "docs/pos-research.md"
```

The `--class` flag is validated against the config at invocation time. If the class doesn't exist in `task_classes`, the command fails with a clear error listing the valid classes.

### Audit tasks

Audit tasks auto-assign `Class: "audit"` if their class is empty. The daemon sets this at claim time, not at creation time, so the `audit` class entry is only required when the daemon runs (not when the project is scaffolded). The `IsAudit` field remains the authoritative marker for audit task identity; `Class` is purely for prompt routing.

---

## Prompt Assembly

The assembled system prompt gains a new section between the script reference and the execute stage prompt:

```
# Project Rules
[rule fragments]

---

# Wolfcastle Script Reference
[filtered script reference]

---

# Task Class: Go Coding
[contents of classes/go-coding.md]

---

# Execute Stage
[execute.md]

---

# Current Task Context
[iteration context with node, task, deliverables, breadcrumbs]
```

The class section is inserted only when the task has a class and a matching config entry exists. Tasks with no class (or an empty class) get the prompt assembled exactly as today.

### Prompt file resolution

Class prompt files live in a `classes/` subdirectory and follow the same three-tier resolution as all other prompts. For a class named `go-coding`:

1. `local/prompts/classes/go-coding.md` (highest priority)
2. `custom/prompts/classes/go-coding.md`
3. `base/prompts/classes/go-coding.md` (ships with Wolfcastle)

Users override a built-in class's behavior by placing a file with the same name in `custom/` or `local/`. Adding a new class means adding an entry in the config and dropping a `.md` file in the appropriate tier.

---

## Intake Classification

The intake prompt is updated to include the list of available classes with their descriptions. The model is instructed to:

1. Assign exactly one class per task via `--class`.
2. If a task spans multiple classes (e.g., "research POS systems and then write the implementation"), split it into separate tasks, one per class.
3. Choose the most specific applicable class. "Go Coding" over "Writing" for a task that produces Go source files, even though it also involves writing.

### Intake prompt additions

```markdown
## Task Classes

Every task must be assigned a class. Use the `--class` flag when adding tasks.

Available classes:
- `go-coding`: Writing or modifying Go source code
- `typescript-coding`: Writing or modifying TypeScript/JavaScript source code
- `architecture`: System design, ADRs, decomposition, dependency analysis
- `research`: Information gathering, comparison, analysis
- `writing`: Documentation, specs, guides, prose
- `design`: UI/UX design, wireframes, interaction patterns

Rules:
- Assign exactly one class per task.
- If work spans multiple classes, split it into separate tasks.
- Choose the most specific class that fits.
```

This section is generated dynamically from the config's `task_classes` map (excluding the `audit` class, which is daemon-managed). The intake prompt template receives the class list as context, not as hardcoded text.

---

## Daemon Dispatch

In `runIteration`, after claiming the task, the daemon looks up the task's class:

```
1. Read task.Class from node state
2. If class is empty and task.IsAudit, set class = "audit"
3. Look up class in config.TaskClasses
4. If found:
   a. Resolve classes/<key>.md via three-tier system
   b. If model override is set, use that model for invocation
   c. Pass the behavioral prompt to AssemblePrompt as the class section
5. If not found (empty class or missing config entry):
   a. Assemble prompt without a class section (today's behavior)
```

No changes to the execute stage's `AllowedCommands`, script reference filtering, or terminal marker handling. The class only affects which behavioral prompt is injected and optionally which model runs.

---

## Default Classes

The default class set ships with Wolfcastle and covers the languages and disciplines most users will encounter. These are not stubs. Most users will never configure their own classes, so the defaults must be production-quality: deep, language-specific, and informed by each ecosystem's actual conventions and tooling.

### Language classes

Based on the TIOBE Index, Stack Overflow surveys, and GitHub language statistics, the following programming languages warrant dedicated classes. Each prompt must include language-specific guidance on: idiomatic style, error handling patterns, testing conventions, build/compile/lint commands, package management, common pitfalls, and the verification steps to run before signaling completion.

| Class key | Language | Notes |
|-----------|----------|-------|
| `python` | Python | Type hints, virtual environments, pytest, ruff/black, PEP 8 |
| `javascript` | JavaScript | ESM vs CJS, Node vs browser, eslint, testing frameworks |
| `typescript` | TypeScript | tsconfig strictness, type-only imports, declaration files |
| `java` | Java | Maven/Gradle, JUnit, checked exceptions, Spring conventions |
| `csharp` | C# | .NET SDK, NuGet, xUnit/NUnit, nullable reference types |
| `go` | Go | gofmt, go vet, table-driven tests, error wrapping |
| `rust` | Rust | cargo clippy, ownership/borrowing guidance, Result/Option patterns |
| `cpp` | C++ | CMake, clang-tidy, RAII, smart pointers, UB avoidance |
| `c` | C | Makefile conventions, valgrind, buffer safety, POSIX portability |
| `ruby` | Ruby | Bundler, RSpec/minitest, Rubocop, Rails conventions when applicable |
| `php` | PHP | Composer, PHPUnit, PSR standards, Laravel/Symfony awareness |
| `swift` | Swift | Xcode/SPM, XCTest, optionals, protocol-oriented patterns |
| `kotlin` | Kotlin | Gradle, JUnit/kotest, null safety, coroutine conventions |
| `scala` | Scala | sbt, ScalaTest, functional patterns, implicits guidance |
| `shell` | Shell/Bash | shellcheck, POSIX compatibility, quoting rules, set -euo pipefail |
| `sql` | SQL | Dialect awareness (Postgres, MySQL, SQLite), migration patterns, injection prevention |
| `r` | R | tidyverse conventions, testthat, roxygen2, CRAN packaging |
| `lua` | Lua | LuaRocks, busted, metatables, embedding considerations |
| `elixir` | Elixir | mix, ExUnit, OTP patterns, pattern matching, pipe operator |
| `haskell` | Haskell | cabal/stack, HSpec, monadic patterns, type-driven development |
| `dart` | Dart | pub, flutter test, null safety, widget patterns |

### Non-language classes

| Class key | Discipline | Notes |
|-----------|------------|-------|
| `architecture` | System design | ADRs, dependency analysis, failure modes, decomposition |
| `research` | Information gathering | Source citation, accuracy over speed, structured output |
| `writing` | Documentation and prose | Reader-first, concrete examples, scannable structure |
| `design` | UI/UX design | User goals, interaction sequences, edge states |
| `devops` | Infrastructure and CI/CD | Dockerfile, GitHub Actions, Terraform, deployment safety |
| `data` | Data engineering and analysis | Schemas, pipelines, validation, visualization |
| `security` | Security review and hardening | OWASP awareness, threat modeling, dependency auditing |
| `testing` | Test suite creation | Coverage strategy, fixture design, flaky test prevention |
| `audit` | Verification of completed work | Read-only review, gap recording, no fixes |

### Prompt authoring process

The behavioral prompts are the product. They must be authored with the same care as a style guide or engineering handbook. The implementation process should use subagents to research and draft each prompt:

**For each language class:**
1. Research the language's official style guide (PEP 8, Effective Go, Rust API Guidelines, etc.)
2. Research the ecosystem's standard toolchain: formatter, linter, test runner, package manager, build system
3. Research common pitfalls and anti-patterns specific to the language
4. Research the language's error handling idiom (exceptions, Result types, error returns, etc.)
5. Research testing conventions (unit test frameworks, assertion styles, mocking patterns)
6. Draft a prompt that covers: style, error handling, testing, tooling commands, validation steps, and language-specific traps to avoid
7. The prompt should be 40-80 lines: comprehensive enough to shape behavior meaningfully, short enough that it doesn't dominate the context window

**For each non-language class:**
1. Research best practices in the discipline (e.g., for "research": academic citation standards, fact-checking methodology, synthesis techniques)
2. Research common failure modes when LLMs attempt this kind of work (e.g., for "writing": tendency toward vague summaries; for "research": hallucinated citations)
3. Draft a prompt that addresses both the positive guidance and the known failure modes
4. Target 30-60 lines per prompt

**Quality bar:** Each prompt should read like it was written by a senior practitioner of that language or discipline. A Go developer reading the Go class prompt should nod, not wince. A technical writer reading the writing class prompt should recognize their own standards reflected back.

---

## Migration

This is additive. Existing tasks without a `Class` field continue to work exactly as they do today (empty class, no behavioral prompt section, default execute model). No migration required.

New projects get the benefit of classification when the intake model is updated with the class list. Existing projects in flight are unaffected.

---

## What This Does Not Cover

- **Class-specific allowed commands.** Classes don't restrict tools. If a future need arises, `allowed_commands` could be added to the class config, but that's a separate decision.
- **Class inheritance or composition.** A task has exactly one class. No "go-coding + research" hybrids. Split the work instead.
- **Automatic class detection from file types.** The intake model classifies based on the inbox item's description. No heuristic fallback.
- **Class-specific validation rules.** All tasks follow the same deliverable verification and state transition rules regardless of class.
