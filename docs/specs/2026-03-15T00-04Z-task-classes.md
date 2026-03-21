# Task Classes

Tasks are not all the same shape. Writing Go code requires different instincts than researching POS systems or drafting documentation. Today, every task gets the same execute prompt regardless of what it actually involves. Task classes fix that: the task's class selects a behavioral prompt that tells the execute model how to think, not what tools it has.

The infrastructure for class resolution, prompt injection, and task-level classification is implemented. The behavioral prompt files themselves and the default class config entries have not yet been authored.

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

No default `task_classes` entries ship in any config tier yet. The example config shown below is the planned default set; see "Future work" at the end of each section for what remains.

```json
{
  "task_classes": {
    "go": { "description": "Writing or modifying Go source code" },
    "research": { "description": "Information gathering, comparison, analysis", "model": "light" },
    "audit": { "description": "Verification and review of completed work" }
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

The `ClassRepository.Validate()` method checks every configured class key for a resolvable prompt file and returns any keys whose prompts are missing from all tiers (including fallback). This is available as a library call but is not wired into daemon startup validation.

**Future work:** Startup validation that enforces non-empty descriptions, valid model references, and resolvable prompt files. CLI-time validation of `--class` values against the config's `task_classes` map (the `task add` command currently accepts any string).

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

The `--class` flag is accepted by `task add` and stored directly on the task. No validation is performed against the config's `task_classes` map at invocation time; any string is accepted.

**Future work:** Validate `--class` against the config at CLI time, rejecting unknown values with an error listing valid classes.

### Audit tasks

**Future work:** Auto-assign `Class: "audit"` to audit tasks at claim time when their class is empty. The `IsAudit` field would remain the authoritative marker for audit identity; `Class` would be purely for prompt routing.

---

## Prompt Assembly

The `ContextBuilder` in `internal/pipeline/context_builder.go` injects class guidance into the iteration context. When a task has a non-empty `Class` field, the builder calls `ClassRepository.Resolve(task.Class)` and, if resolution succeeds, appends a `## Class Guidance` section containing the prompt file's contents.

The class guidance appears after the task context and before the audit context in the assembled output. Tasks with no class (or an empty class, or a class that fails to resolve) get the context assembled without a class section.

```
# Node: my-project
[node context]

# Task: task-0001
[task context]

## Class Guidance
[contents of the resolved class prompt file]

# Audit Context
[audit state, breadcrumbs, gaps]
```

### Prompt file resolution

Class prompt files live under `prompts/classes/` and follow the same three-tier resolution as all other prompts, delegated through `PromptRepository.ResolveRaw`:

1. `local/prompts/classes/<key>.md` (highest priority)
2. `custom/prompts/classes/<key>.md`
3. `base/prompts/classes/<key>.md` (ships with Wolfcastle)

Users override a built-in class's behavior by placing a file with the same name in `custom/` or `local/`. Adding a new class means adding an entry in the config and dropping a `.md` file in the appropriate tier.

**Future work:** No class prompt `.md` files have been authored yet. The resolution infrastructure works, but there is nothing to resolve.

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

## Intake Classification

**Future work.** The intake prompt has not been updated to include class information. Automatic classification by the intake model, dynamic class list generation for the intake prompt, and split-by-class task decomposition rules are all planned but unimplemented.

---

## Daemon Dispatch

The daemon wires `ClassRepository` into the `ContextBuilder` at startup. Class prompt resolution happens inside `ContextBuilder.Build` when it encounters a task with a non-empty `Class` field.

The `ClassDef.Model` field exists in the config struct but the daemon does not read it during dispatch. All tasks use the execute stage's configured model regardless of class.

**Future work:** Model override dispatch, where the daemon reads `ClassDef.Model` and uses a different model for classes that specify one. Auto-assignment of the `"audit"` class to audit tasks at claim time.

---

## Default Classes

**Future work.** The planned default class set is extensive, covering programming languages, frameworks, and non-language disciplines. The tables below preserve the intended scope for when prompt authoring begins.

### Language classes

| Class key | Language | Notes |
|-----------|----------|-------|
| `python` | Python | Type hints, virtual environments, pytest, ruff/black, PEP 8 |
| `javascript` | JavaScript | ESM vs CJS, Node vs browser, eslint, testing frameworks |
| `typescript` | TypeScript | tsconfig strictness, type-only imports, declaration files |
| `java` | Java | Maven/Gradle, JUnit, checked exceptions |
| `csharp` | C# | .NET SDK, NuGet, xUnit/NUnit, nullable reference types |
| `go` | Go | gofmt, go vet, table-driven tests, error wrapping |
| `rust` | Rust | cargo clippy, ownership/borrowing guidance, Result/Option patterns |
| `cpp` | C++ | CMake, clang-tidy, RAII, smart pointers, UB avoidance |
| `c` | C | Makefile conventions, valgrind, buffer safety, POSIX portability |
| `ruby` | Ruby | Bundler, RSpec/minitest, Rubocop |
| `php` | PHP | Composer, PHPUnit, PSR standards |
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

### Framework classes

| Class key | Framework | Notes |
|-----------|-----------|-------|
| `typescript/react` | React (TS) | Hooks, JSX, React Testing Library, component patterns, state management |
| `typescript/vue` | Vue 3 (TS) | Composition API, SFCs, Pinia, Vue Test Utils, Vue Router |
| `typescript/angular` | Angular | Modules/standalone components, RxJS, dependency injection, Jasmine/Karma |
| `typescript/nextjs` | Next.js | App Router, Server Components, ISR/SSG/SSR, middleware, API routes |
| `typescript/svelte` | SvelteKit | Runes, load functions, form actions, server routes |
| `javascript/react` | React (JS) | Same as TS/React but with PropTypes, no type annotations |
| `javascript/node` | Node.js | Express/Fastify patterns, middleware, async error handling, clustering |
| `python/django` | Django | MTV pattern, ORM, migrations, DRF, management commands, template conventions |
| `python/fastapi` | FastAPI | Pydantic models, dependency injection, async endpoints, OpenAPI |
| `python/flask` | Flask | Blueprints, extensions, application factory, Jinja2 |
| `ruby/rails` | Rails | Convention over configuration, ActiveRecord, concerns, RSpec Rails, generators |
| `ruby/sinatra` | Sinatra | Lightweight routing, modular style, Rack middleware |
| `java/spring` | Spring Boot | Auto-configuration, annotations, JPA, Spring Security, integration testing |
| `csharp/dotnet` | .NET | Minimal APIs, Entity Framework, middleware pipeline, Razor conventions |
| `php/laravel` | Laravel | Eloquent, Blade, artisan, service providers, feature tests |
| `php/symfony` | Symfony | Bundles, Doctrine, Twig, event system, PHPUnit bridge |
| `kotlin/android` | Android | Jetpack Compose, ViewModel, Room, coroutines, instrumented tests |
| `swift/ios` | iOS/SwiftUI | SwiftUI views, Combine, Core Data, XCUITest, App lifecycle |
| `dart/flutter` | Flutter | Widget tree, state management (Riverpod/Bloc), platform channels, widget tests |
| `elixir/phoenix` | Phoenix | LiveView, Ecto, PubSub, Channels, ExUnit with Sandbox |
| `rust/actix` | Actix Web | Extractors, middleware, app state, integration tests |
| `rust/tokio` | Tokio async | Spawning, channels, select!, graceful shutdown, tracing |

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

Each behavioral prompt should read like it was written by a senior practitioner of that language, framework, or discipline. Target 40-80 lines for language prompts, 30-60 for framework and non-language prompts. Cover idiomatic style, error handling, testing conventions, tooling commands, and common pitfalls. Framework prompts should not repeat language fundamentals.

---

## Migration

This is additive. Existing tasks without a `Class` field continue to work exactly as they do today (empty class, no behavioral prompt section, default execute model). No migration required.

---

## What This Does Not Cover

- **Class-specific allowed commands.** Classes don't restrict tools. If a future need arises, `allowed_commands` could be added to the class config, but that's a separate decision.
- **Class inheritance or composition.** A task has exactly one class. No "go + research" hybrids. Split the work instead.
- **Automatic class detection from file types.** Classification is manual (via `--class`) until intake integration is built.
- **Class-specific validation rules.** All tasks follow the same deliverable verification and state transition rules regardless of class.
- **Dual prompt assembly for hierarchical keys.** The spec originally proposed assembling both the parent language prompt and the child framework prompt together. The implementation uses single-file resolution with fallback instead.
