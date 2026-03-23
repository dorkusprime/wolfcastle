# Task Classes

Tasks are not all the same shape. Writing Go code requires different instincts than researching POS systems or drafting documentation. Today, every task gets the same execute prompt regardless of what it actually involves. Task classes fix that: the task's class selects a behavioral prompt that tells the execute model how to think, not what tools it has.

The resolution pipeline is fully implemented: class resolution, prompt injection, task-level classification, default config entries for 54 classes (21 languages, 22 frameworks, 9 non-language disciplines, plus universal guidance and a coding default), CLI validation, daemon startup validation, audit auto-assignment, and planning-time class assignment via the planning prompt. The behavioral prompt `.md` files have not yet been authored; the `base/prompts/classes/` directory does not exist. Daemon startup validation will warn about every missing prompt file until they are written.

## Governing ADRs

- ADR-063: Three-Tier Configuration (class definitions merge across tiers)
- ADR-066: Scoped Script References (AllowedCommands per stage, unchanged by classes)
- ADR-067: Terminal Markers Only (class prompts don't change the marker protocol)
- ADR-069: Task Deliverables (deliverable verification is class-agnostic)

---

## Core Concepts

### What a class is

A class is a behavioral modifier. It provides a `.md` prompt that is injected into the iteration context under a `## Class Guidance` heading. The behavioral prompt tells the model what kind of work this is and how to approach it.

A class does NOT change:
- Available tools or allowed commands
- The terminal marker protocol (COMPLETE/YIELD/BLOCKED)
- Deliverable verification logic
- State transitions or propagation

A class CAN change:
- The behavioral prompt section (required)
- The model used for execution (optional override, defined in `ClassDef` but not yet wired in daemon dispatch)

### What a class is not

Classes are not capability gates. A "go" task still has access to web search. A "research" task can still write files. The behavioral prompt shapes the model's approach, priorities, and quality standards; it doesn't restrict its toolbox.

---

## Config Structure

Classes are defined as an object in the config under `task_classes`, keyed by class name. The value is a `ClassDef` with a required `description` and an optional `model` override. Object keys merge cleanly across the three-tier config system (base < custom < local). Users can add new classes in `custom/config.json` or `local/config.json` without touching the defaults.

The `Config` struct carries the map:

```go
TaskClasses map[string]ClassDef `json:"task_classes,omitempty"`
```

And `ClassDef` itself:

```go
type ClassDef struct {
    Description string `json:"description"`
    Model       string `json:"model,omitempty"`
}
```

Default `task_classes` entries ship in the hardcoded `Defaults()` function in `internal/config/config.go`. The set covers programming languages (e.g., `coding/go`, `coding/python`), frameworks (e.g., `coding/typescript/react`, `coding/python/django`), and non-language disciplines (e.g., `architecture`, `research`, `writing`, `audit`). Users can add or override classes in `custom/config.json` or `local/config.json`. Example:

```json
{
  "task_classes": {
    "coding/go": { "description": "gofmt, go vet, table-driven tests, error wrapping" },
    "research": { "description": "Source citation, accuracy over speed, structured output", "model": "fast" },
    "audit": { "description": "Read-only review, gap recording, no fixes" }
  }
}
```

### Hierarchical class keys

Class keys use `/` as a separator to express framework specificity. The `ClassRepository` resolves prompt files with a one-level fallback: it tries the exact key first, then strips the last segment (after `/` or `-`) and tries the parent.

- `typescript` resolves to `prompts/classes/typescript.md`
- `typescript/react` tries `prompts/classes/typescript/react.md`, falling back to `prompts/classes/typescript.md`

The fallback also supports `-` as a separator (e.g., `lang-go` falls back to `lang`), though `/`-separated hierarchical keys are the intended convention.

The current implementation resolves a single prompt file per class. It does NOT assemble both the parent and child prompts together for hierarchical keys; it falls through from child to parent and returns whichever resolves first.

### Field definitions

| Field | Required | Description |
|-------|----------|-------------|
| `description` | Yes | One-line description used for classification |
| `model` | No | Model key override. The field exists on `ClassDef` but daemon dispatch does not yet read it. |

### Validation

The `ClassRepository.Validate()` method checks every configured class key for a resolvable prompt file and returns any keys whose prompts are missing from all tiers (including fallback). This is wired into daemon startup validation, which logs warnings for any classes with missing prompt files. CLI-time validation of `--class` values is performed in `task add` against the config's `task_classes` map; unknown values are rejected with an error listing valid classes.

---

## Task Struct

The `Task` struct in `internal/state/types.go` carries both a `TaskType` and a `Class` field:

```go
type Task struct {
    ID                 string     `json:"id"`
    Title              string     `json:"title,omitempty"`
    Description        string     `json:"description"`
    State              NodeStatus `json:"state"`
    TaskType           string     `json:"task_type,omitempty"`
    Class              string     `json:"class,omitempty"`
    // ... other fields
}
```

`TaskType` is an older field with a fixed set of valid values (`discovery`, `spec`, `adr`, `implementation`, `integration`, `cleanup`). `Class` is the newer, config-driven classification. The two coexist; `TaskType` is validated at the CLI, `Class` is not.

### CLI

```
wolfcastle task add "Implement auth middleware" --node my-project --class go
wolfcastle task add "Research POS systems" --node pizza-docs --class research --deliverable "docs/pos-research.md"
```

The `--class` flag is accepted by `task add` and stored directly on the task. Validation is performed at invocation time against the config's `task_classes` map; unknown values are rejected with an error listing valid classes.

### Audit tasks

Auto-assignment of `Class: "audit"` to audit tasks is implemented at claim time when their class is empty. The `IsAudit` field remains the authoritative marker for audit identity; `Class` is used purely for prompt routing.

---

## Prompt Assembly

The `ContextBuilder` injects two layers of class guidance into every task's iteration context:

1. **Universal** (`prompts/classes/universal.md`): principles that apply to all work regardless of type. Always injected.
2. **Class-specific**: resolved from the task's `Class` field. Coding tasks get a prompt from `prompts/classes/coding/`. Non-coding tasks (writing, research, audit) get a prompt from `prompts/classes/` directly.

When a task has no class (empty or unset), the ContextBuilder injects `universal.md` + `coding/default.md`. When a class is set but fails to resolve, it falls back to `coding/default.md`. No task runs without class guidance.

```
# Node: my-project
[node context]

# Task: task-0001
[task context]

## Universal Guidance
[contents of universal.md]

## Class Guidance
[contents of the resolved class prompt file]

# Audit Context
[audit state, breadcrumbs, gaps]
```

### File structure

```
prompts/classes/
  universal.md                # always injected for every task
  coding/
    default.md                # generic coding guidance (unclassified coding tasks)
    go.md
    python.md
    typescript.md
    ...
    typescript/
      react.md
      vue.md
      ...
  writing.md
  writing/
    voice.md                  # separate so users can override voice independently
  research.md
  architecture.md
  audit.md
  design.md
  devops.md
  data.md
  security.md
  testing.md
```

Language class files live under `coding/` at the top level (`coding/go.md`, `coding/python.md`). Framework files live in subdirectories under `coding/` (`coding/typescript/react.md`). Non-language classes live directly under `classes/` (`writing.md`, `audit.md`).

### Resolution

Class prompt files follow the same three-tier resolution as all other prompts:

1. `local/prompts/classes/<path>` (highest priority)
2. `custom/prompts/classes/<path>`
3. `base/prompts/classes/<path>` (ships with Wolfcastle)

For coding classes, the resolver prepends `coding/` to the key: class `go` resolves to `coding/go.md`. Class `typescript/react` resolves to `coding/typescript/react.md`, falling back to `coding/typescript.md`.

Users override a built-in class's behavior by placing a file with the same path in `custom/` or `local/`. Adding a new class means adding an entry in the config and dropping a `.md` file in the appropriate tier.

---

## ClassRepository

The `ClassRepository` in `internal/pipeline/class_repository.go` owns class prompt resolution. It wraps a `PromptRepository` for file access and maintains a goroutine-safe map of configured class definitions.

**Implemented methods:**

- `NewClassRepository(prompts)` creates a repository with an empty class map.
- `Reload(classes)` replaces the internal class map (called by the daemon after loading config).
- `Resolve(key)` returns the behavioral prompt content for a class key, using one-level fallback.
- `List()` returns all configured class keys, sorted lexicographically.
- `Validate()` returns sorted class keys whose prompts are missing from all tiers.

**Daemon wiring:** In `internal/daemon/daemon.go`, the daemon creates a `ClassRepository`, calls `Reload(cfg.TaskClasses)` with the loaded config, and passes it to `NewContextBuilder`. The `ContextBuilder` uses it during iteration context assembly.

---

## Daemon Dispatch

The daemon wires `ClassRepository` into the `ContextBuilder` at startup. Class prompt resolution happens inside `ContextBuilder.Build` when it encounters a task with a non-empty `Class` field. All tasks currently use the execute stage's configured model regardless of class (see Remaining Work for model override dispatch).

---

## Default Classes

The default class set ships in `Defaults()` (`internal/config/config.go`), covering programming languages, frameworks, and non-language disciplines. All class keys use the `coding/` prefix for language and framework classes. The tables below reflect the current defaults.

### Language classes

| Class key | Language | Notes |
|-----------|----------|-------|
| `coding/python` | Python | Type hints, virtual environments, pytest, ruff/black, PEP 8 |
| `coding/javascript` | JavaScript | ESM vs CJS, Node vs browser, eslint, testing frameworks |
| `coding/typescript` | TypeScript | tsconfig strictness, type-only imports, declaration files |
| `coding/java` | Java | Maven/Gradle, JUnit, checked exceptions |
| `coding/csharp` | C# | .NET SDK, NuGet, xUnit/NUnit, nullable reference types |
| `coding/go` | Go | gofmt, go vet, table-driven tests, error wrapping |
| `coding/rust` | Rust | cargo clippy, ownership/borrowing guidance, Result/Option patterns |
| `coding/cpp` | C++ | CMake, clang-tidy, RAII, smart pointers, UB avoidance |
| `coding/c` | C | Makefile conventions, valgrind, buffer safety, POSIX portability |
| `coding/ruby` | Ruby | Bundler, RSpec/minitest, Rubocop |
| `coding/php` | PHP | Composer, PHPUnit, PSR standards |
| `coding/swift` | Swift | Xcode/SPM, XCTest, optionals, protocol-oriented patterns |
| `coding/kotlin` | Kotlin | Gradle, JUnit/kotest, null safety, coroutine conventions |
| `coding/scala` | Scala | sbt, ScalaTest, functional patterns, implicits guidance |
| `coding/shell` | Shell/Bash | shellcheck, POSIX compatibility, quoting rules, set -euo pipefail |
| `coding/sql` | SQL | Dialect awareness (Postgres, MySQL, SQLite), migration patterns, injection prevention |
| `coding/r` | R | tidyverse conventions, testthat, roxygen2, CRAN packaging |
| `coding/lua` | Lua | LuaRocks, busted, metatables, embedding considerations |
| `coding/elixir` | Elixir | mix, ExUnit, OTP patterns, pattern matching, pipe operator |
| `coding/haskell` | Haskell | cabal/stack, HSpec, monadic patterns, type-driven development |
| `coding/dart` | Dart | pub, flutter test, null safety, widget patterns |

### Framework classes

| Class key | Framework | Notes |
|-----------|-----------|-------|
| `coding/typescript/react` | React (TS) | Hooks, JSX, React Testing Library, component patterns, state management |
| `coding/typescript/vue` | Vue 3 (TS) | Composition API, SFCs, Pinia, Vue Test Utils, Vue Router |
| `coding/typescript/angular` | Angular | Modules/standalone components, RxJS, dependency injection, Jasmine/Karma |
| `coding/typescript/nextjs` | Next.js | App Router, Server Components, ISR/SSG/SSR, middleware, API routes |
| `coding/typescript/svelte` | SvelteKit | Runes, load functions, form actions, server routes |
| `coding/javascript/react` | React (JS) | Same as TS/React but with PropTypes, no type annotations |
| `coding/javascript/node` | Node.js | Express/Fastify patterns, middleware, async error handling, clustering |
| `coding/python/django` | Django | MTV pattern, ORM, migrations, DRF, management commands, template conventions |
| `coding/python/fastapi` | FastAPI | Pydantic models, dependency injection, async endpoints, OpenAPI |
| `coding/python/flask` | Flask | Blueprints, extensions, application factory, Jinja2 |
| `coding/ruby/rails` | Rails | Convention over configuration, ActiveRecord, concerns, RSpec Rails, generators |
| `coding/ruby/sinatra` | Sinatra | Lightweight routing, modular style, Rack middleware |
| `coding/java/spring` | Spring Boot | Auto-configuration, annotations, JPA, Spring Security, integration testing |
| `coding/csharp/dotnet` | .NET | Minimal APIs, Entity Framework, middleware pipeline, Razor conventions |
| `coding/php/laravel` | Laravel | Eloquent, Blade, artisan, service providers, feature tests |
| `coding/php/symfony` | Symfony | Bundles, Doctrine, Twig, event system, PHPUnit bridge |
| `coding/kotlin/android` | Android | Jetpack Compose, ViewModel, Room, coroutines, instrumented tests |
| `coding/swift/ios` | iOS/SwiftUI | SwiftUI views, Combine, Core Data, XCUITest, App lifecycle |
| `coding/dart/flutter` | Flutter | Widget tree, state management (Riverpod/Bloc), platform channels, widget tests |
| `coding/elixir/phoenix` | Phoenix | LiveView, Ecto, PubSub, Channels, ExUnit with Sandbox |
| `coding/rust/actix` | Actix Web | Extractors, middleware, app state, integration tests |
| `coding/rust/tokio` | Tokio async | Spawning, channels, select!, graceful shutdown, tracing |

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

### Prompt authoring guidelines

**Class prompts contain ZERO wolfcastle-specific content.** No references to wolfcastle commands, terminal markers, breadcrumbs, audit gaps, deliverables, specs, ADRs, AARs, yielding, or any other system mechanic. All of that belongs in the execute prompt and default.md. A user who overrides `go.md` in their `custom/` tier should only need to describe how they write Go at their organization: style, tooling, testing, conventions. They should not need to know or preserve any wolfcastle internals.

Each behavioral prompt must be grounded in research, not just training data. Before authoring a prompt, research the language's current ecosystem: official style guides, community-accepted linters and formatters, modern testing frameworks, current build tools, and recent paradigm shifts. For example, Python's ecosystem moved from `black` + `flake8` to `ruff`; Go added `slices` and `maps` packages in 1.21; Rust's 2024 edition changed certain defaults. Prompts based on stale conventions will produce stale code.

Each behavioral prompt should read like it was written by a senior practitioner of that language, framework, or discipline. Target 40-80 lines for language prompts, 30-60 for framework and non-language prompts. Cover:

- **Style**: idiomatic patterns, naming, error handling, code organization
- **Build and test**: specific tool names and invocations (not "run the project's linter" but "run `golangci-lint run ./...`")
- **Testing conventions**: framework-specific patterns, test structure, assertion style
- **Common pitfalls**: language-specific footguns that agents frequently trigger

Do NOT include:
- How to commit (the execute prompt handles that)
- How to signal completion (terminal markers are system mechanics)
- References to `.wolfcastle/` directories, specs, ADRs, or audit systems
- Planning guidance (decomposition, file counts, yielding)
- Deliverable instructions

Framework prompts should not repeat language fundamentals (the fallback mechanism handles that).

**Codebase conventions take precedence.** Class prompts describe modern, community-accepted defaults, but the codebase the agent is working in has its own patterns and constraints. When the codebase uses an older language version, a different formatter, or an established convention that differs from the class prompt, the agent should follow the codebase. Prompts should phrase guidance as "prefer X" or "use X when available" rather than "always use X." For example: "prefer `slices.Contains` (Go 1.21+); in older codebases, use a manual loop" rather than "use `slices.Contains`." The agent should never be forced to choose between its prompt and the code in front of it.

---

## Migration

This is additive. Existing tasks without a `Class` field continue to work exactly as they do today (empty class, no behavioral prompt section, default execute model). No migration required.

---

## Remaining Work

Three pieces remain before task classes are fully operational:

1. **Behavioral prompt authoring.** The 55 `.md` files under `base/prompts/classes/` (universal, coding/default, 21 language, 22 framework, and 9 non-language prompts, plus `writing/voice.md`) do not exist yet. The resolution pipeline, config defaults, CLI validation, and daemon wiring are all in place; what's missing is the content. Until the files are written, the ContextBuilder falls back to `coding/default.md` for every task (which also doesn't exist), so class guidance sections will be empty. Follow the prompt authoring guidelines in this spec when writing them.

2. **Model override dispatch.** The `ClassDef.Model` field exists in the config struct and defaults ship for the `research` class (`Model: "fast"`), but the daemon does not read it during dispatch. All tasks use the execute stage's configured model regardless of class. Wiring this means reading `ClassDef.Model` in the daemon's stage invocation path and selecting the corresponding model from the config's `Models` map.

3. **Intake classification.** The intake prompt has not been updated to include class information. Automatic classification by the intake model, dynamic class list generation for the intake prompt, and split-by-class task decomposition rules are all planned but unimplemented.

---

## What This Does Not Cover

- **Class-specific allowed commands.** Classes don't restrict tools. If a future need arises, `allowed_commands` could be added to the class config, but that's a separate decision.
- **Class inheritance or composition.** A task has exactly one class. No "go + research" hybrids. Split the work instead.
- **Automatic class detection from file types.** Classification is manual (via `--class` at the CLI or assigned by the planning agent). Intake-level automatic classification is not implemented.
- **Class-specific validation rules.** All tasks follow the same deliverable verification and state transition rules regardless of class.
- **Dual prompt assembly for hierarchical keys.** The spec originally proposed assembling both the parent language prompt and the child framework prompt together. The implementation uses single-file resolution with fallback instead.
- **Config-level validation of ClassDef.Model.** When a class specifies a `model` override, nothing currently validates that the model key exists in the `Models` map. Invalid references will fail at dispatch time (once model override dispatch is implemented) rather than at config load time.
