# Task Classes

Task classes are behavioral prompts that shape how the execution agent approaches work. A Go coding task calls for different instincts than a security review or a research pass, and classes encode that difference. When a task carries a class, the agent receives a class-specific prompt that tells it how to think about the problem: what idioms to follow, what tools to reach for, what quality standards to enforce.

Classes do not restrict capabilities. A `coding/go` task can still search the web. A `research` task can still write files. The prompt shapes the agent's priorities and judgment; it does not gate its toolbox. Terminal markers, deliverable verification, and state transitions all work identically regardless of class.

Every task receives two layers of guidance:

1. **Universal** (`universal.md`): principles that apply to all work, injected into every iteration.
2. **Class-specific**: resolved from the task's class field. If the class is empty or fails to resolve, the agent falls back to `coding/default.md`, so no task ever runs without guidance.

For details on how class definitions merge across tiers, see the [Configuration Guide](configuration.md). For the `task_classes` config fields, see the [Config Reference](config-reference.md#task_classes).

---

## Using a Class

Assign a class when creating a task with the `--class` flag:

```
wolfcastle task add "Implement auth middleware" --node my-project --class coding/go
wolfcastle task add "Research POS systems" --node pizza-docs --class research
```

Language classes live under the `coding/` prefix. Non-language discipline classes (`research`, `writing`, `audit`, etc.) use bare keys.

### Framework Fallback

Framework classes use hierarchical keys with `/` separators. When the agent resolves a framework class, it tries the exact key first, then falls back to the parent language:

```
coding/typescript/react  ->  tries coding/typescript/react.md
                              falls back to coding/typescript.md
```

The resolver returns whichever prompt resolves first. It does not combine the parent and child prompts together. If you're using a framework class and the framework prompt file doesn't exist, the agent still gets the parent language's guidance.

```
wolfcastle task add "Build dashboard" --node frontend --class coding/typescript/react
wolfcastle task add "Add SvelteKit routes" --node frontend --class coding/typescript/svelte
```

---

## Built-in Language Classes

All language classes are prefixed with `coding/`. The table below groups them by category.

### Systems

| Class Key | Description |
|-----------|-------------|
| `coding/go` | gofmt, go vet, table-driven tests, error wrapping |
| `coding/rust` | cargo clippy, ownership/borrowing guidance, Result/Option patterns |
| `coding/c` | Makefile conventions, valgrind, buffer safety, POSIX portability |
| `coding/cpp` | CMake, clang-tidy, RAII, smart pointers, UB avoidance |

### JVM / .NET

| Class Key | Description |
|-----------|-------------|
| `coding/java` | Maven/Gradle, JUnit, checked exceptions |
| `coding/kotlin` | Gradle, JUnit/kotest, null safety, coroutine conventions |
| `coding/scala` | sbt, ScalaTest, functional patterns, implicits guidance |
| `coding/csharp` | .NET SDK, NuGet, xUnit/NUnit, nullable reference types |

### Web / Scripting

| Class Key | Description |
|-----------|-------------|
| `coding/javascript` | ESM vs CJS, Node vs browser, eslint, testing frameworks |
| `coding/typescript` | tsconfig strictness, type-only imports, declaration files |
| `coding/python` | Type hints, virtual environments, pytest, ruff/black, PEP 8 |
| `coding/ruby` | Bundler, RSpec/minitest, Rubocop |
| `coding/php` | Composer, PHPUnit, PSR standards |
| `coding/lua` | LuaRocks, busted, metatables, embedding considerations |
| `coding/elixir` | mix, ExUnit, OTP patterns, pattern matching, pipe operator |
| `coding/dart` | pub, flutter test, null safety, widget patterns |

### Data / Specialized

| Class Key | Description |
|-----------|-------------|
| `coding/r` | tidyverse conventions, testthat, roxygen2, CRAN packaging |
| `coding/sql` | Dialect awareness (Postgres, MySQL, SQLite), migration patterns, injection prevention |
| `coding/haskell` | cabal/stack, HSpec, monadic patterns, type-driven development |
| `coding/swift` | Xcode/SPM, XCTest, optionals, protocol-oriented patterns |
| `coding/shell` | shellcheck, POSIX compatibility, quoting rules, set -euo pipefail |

---

## Built-in Framework Classes

Framework classes live under their parent language. Each one falls back to the parent language prompt if the framework-specific file is missing.

### TypeScript Frameworks

| Class Key | Description |
|-----------|-------------|
| `coding/typescript/react` | Hooks, JSX, React Testing Library, component patterns, state management |
| `coding/typescript/vue` | Composition API, SFCs, Pinia, Vue Test Utils, Vue Router |
| `coding/typescript/angular` | Modules/standalone components, RxJS, dependency injection, Jasmine/Karma |
| `coding/typescript/nextjs` | App Router, Server Components, ISR/SSG/SSR, middleware, API routes |
| `coding/typescript/svelte` | Runes, load functions, form actions, server routes |

### JavaScript Frameworks

| Class Key | Description |
|-----------|-------------|
| `coding/javascript/react` | Same as TS/React but with PropTypes, no type annotations |
| `coding/javascript/node` | Express/Fastify patterns, middleware, async error handling, clustering |

### Python Frameworks

| Class Key | Description |
|-----------|-------------|
| `coding/python/django` | MTV pattern, ORM, migrations, DRF, management commands, template conventions |
| `coding/python/fastapi` | Pydantic models, dependency injection, async endpoints, OpenAPI |
| `coding/python/flask` | Blueprints, extensions, application factory, Jinja2 |

### Ruby Frameworks

| Class Key | Description |
|-----------|-------------|
| `coding/ruby/rails` | Convention over configuration, ActiveRecord, concerns, RSpec Rails, generators |
| `coding/ruby/sinatra` | Lightweight routing, modular style, Rack middleware |

### Java / C# / PHP Frameworks

| Class Key | Description |
|-----------|-------------|
| `coding/java/spring` | Auto-configuration, annotations, JPA, Spring Security, integration testing |
| `coding/csharp/dotnet` | Minimal APIs, Entity Framework, middleware pipeline, Razor conventions |
| `coding/php/laravel` | Eloquent, Blade, artisan, service providers, feature tests |
| `coding/php/symfony` | Bundles, Doctrine, Twig, event system, PHPUnit bridge |

### Mobile Frameworks

| Class Key | Description |
|-----------|-------------|
| `coding/kotlin/android` | Jetpack Compose, ViewModel, Room, coroutines, instrumented tests |
| `coding/swift/ios` | SwiftUI views, Combine, Core Data, XCUITest, App lifecycle |
| `coding/dart/flutter` | Widget tree, state management (Riverpod/Bloc), platform channels, widget tests |

### Elixir / Rust Frameworks

| Class Key | Description |
|-----------|-------------|
| `coding/elixir/phoenix` | LiveView, Ecto, PubSub, Channels, ExUnit with Sandbox |
| `coding/rust/actix` | Extractors, middleware, app state, integration tests |
| `coding/rust/tokio` | Spawning, channels, select!, graceful shutdown, tracing |

---

## Non-language Classes

Discipline classes cover work that isn't primarily about writing code in a specific language. They use bare keys (no `coding/` prefix).

| Class Key | Description |
|-----------|-------------|
| `architecture` | ADRs, dependency analysis, failure modes, decomposition |
| `research` | Source citation, accuracy over speed, structured output |
| `writing` | Reader-first, concrete examples, scannable structure |
| `design` | User goals, interaction sequences, edge states |
| `devops` | Dockerfile, GitHub Actions, Terraform, deployment safety |
| `data` | Schemas, pipelines, validation, visualization |
| `security` | OWASP awareness, threat modeling, dependency auditing |
| `testing` | Coverage strategy, fixture design, flaky test prevention |
| `audit` | Read-only review, gap recording, no fixes |

---

## Creating a Custom Class

Custom classes live in your `system/custom/` tier (committed, shared with your team) or `system/local/` tier (gitignored, personal).

**Step 1: Add a ClassDef to config.json.**

In `system/custom/config.json`, add an entry under `task_classes`:

```json
{
  "task_classes": {
    "coding/go-internal": {
      "description": "Go code following internal team conventions"
    }
  }
}
```

The key becomes the value you pass to `--class`. The `description` field is required. The optional `model` field names a model key to use for execution (not yet wired into daemon dispatch).

**Step 2: Create the prompt file.**

Drop a `.md` file at the matching path under `system/custom/prompts/classes/`:

```
.wolfcastle/system/custom/prompts/classes/coding/go-internal.md
```

The prompt should describe your team's Go conventions: style rules, preferred libraries, testing patterns, naming conventions. It should read like guidance from a senior engineer on your team, covering what matters for your codebase.

Class prompts should contain zero Wolfcastle-specific content. No references to terminal markers, breadcrumbs, audit gaps, deliverables, or any other system mechanic. A developer overriding a class prompt should only need to know their language and their team's conventions.

**Step 3: Verify.**

```
wolfcastle config show --section task_classes
```

Your new class should appear in the merged output alongside the built-in classes.

**Step 4: Use it.**

```
wolfcastle task add "Refactor auth package" --node backend --class coding/go-internal
```

---

## Overriding a Built-in Class

To replace the behavior of a built-in class, place a file with the same path in `system/custom/prompts/classes/` (or `system/local/prompts/classes/` for personal overrides). The three-tier prompt resolution will pick up your file instead of the base tier's version.

For example, to override the Go class prompt:

```
.wolfcastle/system/custom/prompts/classes/coding/go.md
```

Your replacement file takes over entirely. Write only your team's coding conventions; the universal prompt and all Wolfcastle system mechanics are injected separately.

---

## Creating a Custom Framework

Framework classes are hierarchical extensions of language classes. Creating one follows the same pattern as any custom class, with the fallback mechanic giving you a safety net.

**Step 1: Add the config entry.**

```json
{
  "task_classes": {
    "coding/python/graphql": {
      "description": "Strawberry/Ariadne GraphQL patterns, schema-first design, resolver conventions"
    }
  }
}
```

**Step 2: Create the prompt file in the language subdirectory.**

```
.wolfcastle/system/custom/prompts/classes/coding/python/graphql.md
```

**Step 3: Verify fallback behavior.**

If you remove the framework prompt file, tasks with `--class coding/python/graphql` will fall back to `coding/python.md` automatically. The framework prompt should focus on framework-specific patterns and not repeat language fundamentals; the fallback mechanism exists so that if the specific prompt is missing, the agent still gets reasonable guidance.

---

## The Universal Prompt

The file `prompts/classes/universal.md` is injected into every task's iteration context, regardless of class. It contains principles that apply to all work: quality standards, communication expectations, and general approach guidelines.

The universal prompt appears in a `## Universal Guidance` section above the class-specific `## Class Guidance` section. Both are always present (the class guidance falls back to `coding/default.md` if no class is set or the class fails to resolve).

To override the universal prompt, place your version at:

- `system/custom/prompts/classes/universal.md` for team-wide changes (committed)
- `system/local/prompts/classes/universal.md` for personal changes (gitignored)

The same three-tier resolution that governs all prompt files applies here. Your override replaces the base version completely.
