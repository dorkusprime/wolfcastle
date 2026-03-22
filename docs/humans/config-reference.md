# Config Reference

This is the field-level reference for every Wolfcastle configuration option. Fields are organized by section, matching the JSON structure of your `config.json` files.

For context on how the three-tier system works, how merges resolve, and how to use the CLI to modify config, see the [Configuration Guide](configuration.md).

---

## version

Schema version for the configuration format. Used by Wolfcastle to detect incompatible config files.

- **Type:** `int`
- **Default:** `1`
- **Description:** Configuration schema version. Currently only version `1` exists.

```json
{
  "version": 1
}
```

---

## identity

Identifies the engineer and machine running Wolfcastle. Typically auto-populated in `local/config.json` by `wolfcastle init`.

### identity.user

- **Type:** `string`
- **Default:** *(none, set by `wolfcastle init`)*
- **Description:** Username for the current engineer. Determines the [namespace](collaboration.md#engineer-namespacing) under which your project tree lives (`.wolfcastle/system/projects/<user>-<machine>/`).

```json
{
  "identity": {
    "user": "jane"
  }
}
```

### identity.machine

- **Type:** `string`
- **Default:** *(none, set by `wolfcastle init`)*
- **Description:** Machine identifier. Combined with `user` to form the project namespace, allowing the same engineer to run separate trees on different machines.

```json
{
  "identity": {
    "machine": "workstation"
  }
}
```

---

## models

Defines the CLI commands Wolfcastle invokes as "models." Each key is a model name referenced elsewhere in config (pipeline stages, summary, doctor, etc.). Wolfcastle does not embed any model SDK; it shells out to whatever command you configure here.

- **Type:** `map[string]ModelDef`
- **Default:** Three models: `fast`, `mid`, `heavy` (see below)

Each `ModelDef` has two fields:

### ModelDef.command

- **Type:** `string`
- **Description:** The executable to invoke. Must be on `$PATH` or an absolute path.

### ModelDef.args

- **Type:** `[]string`
- **Description:** Arguments passed to the command. The assembled prompt is piped to stdin.

**Default models:**

| Name | Command | Model |
|------|---------|-------|
| `fast` | `claude` | `claude-haiku-4-5-20251001` |
| `mid` | `claude` | `claude-sonnet-4-6` |
| `heavy` | `claude` | `claude-opus-4-6` |

```json
{
  "models": {
    "local-llama": {
      "command": "ollama",
      "args": ["run", "llama3", "--format", "json"]
    }
  }
}
```

---

## pipeline

Controls the stage pipeline that the daemon runs for each iteration.

### pipeline.stages

- **Type:** `map[string]PipelineStage`
- **Default:** Two stages: `intake` and `execute`
- **Description:** Dictionary of named pipeline stages. The dict format allows higher tiers to override a single stage's fields without rewriting the entire map. Each stage's `model` field must reference a key defined in [models](#models).

Each `PipelineStage` contains:

#### PipelineStage.model

- **Type:** `string`
- **Description:** Name of the model to invoke for this stage. Must match a key in [models](#models).

#### PipelineStage.prompt_file

- **Type:** `string`
- **Description:** Path to the prompt template file, resolved through the three-tier prompt system.

#### PipelineStage.enabled

- **Type:** `*bool`
- **Default:** `true` (when omitted)
- **Description:** Whether this stage runs. Set to `false` to disable a stage without removing it.

```json
{
  "pipeline": {
    "stages": {
      "intake": {
        "enabled": false
      }
    }
  }
}
```

#### PipelineStage.skip_prompt_assembly

- **Type:** `*bool`
- **Default:** `false` (when omitted)
- **Description:** When `true`, the raw prompt file is sent directly without fragment assembly or template expansion.

#### PipelineStage.allowed_commands

- **Type:** `[]string`
- **Default:** Stage-dependent (see below)
- **Description:** Wolfcastle CLI commands the agent is permitted to invoke during this stage. Commands not in this list are rejected.

**Default stage configuration:**

| Stage | Model | Prompt File | Allowed Commands |
|-------|-------|-------------|-----------------|
| `intake` | `mid` | `stages/intake.md` | `project create`, `task add`, `status` |
| `execute` | `heavy` | `stages/execute.md` | `project create`, `task add`, `task block`, `task deliverable`, `audit breadcrumb`, `audit escalate`, `audit gap`, `audit fix-gap`, `audit scope`, `audit summary`, `audit resolve-escalation`, `status`, `adr create`, `spec create`, `spec link`, `spec list` |

### pipeline.stage_order

- **Type:** `[]string`
- **Default:** `["intake", "execute"]`
- **Description:** Controls which stages run and in what sequence. Omit to run all stages in map-iteration order. Every entry must match a key in `pipeline.stages`.

```json
{
  "pipeline": {
    "stage_order": ["lint-check", "intake", "execute"]
  }
}
```

### pipeline.planning

Controls orchestrator planning passes, where the daemon decomposes work into child nodes and tasks.

#### pipeline.planning.enabled

- **Type:** `bool`
- **Default:** `true`
- **Description:** Whether the daemon runs planning passes to decompose orchestrator nodes into children.

#### pipeline.planning.model

- **Type:** `string`
- **Default:** `"heavy"`
- **Description:** Model used for planning passes. Must match a key in [models](#models).

#### pipeline.planning.max_children

- **Type:** `int`
- **Default:** `10`
- **Description:** Maximum number of child nodes a planning pass may create under a single orchestrator.

#### pipeline.planning.max_tasks_per_leaf

- **Type:** `int`
- **Default:** `8`
- **Description:** Maximum tasks a planning pass may assign to a single leaf node.

#### pipeline.planning.max_replans

- **Type:** `int`
- **Default:** `3`
- **Description:** Maximum number of times the daemon may re-plan an orchestrator node before giving up.

---

## daemon

Controls the daemon's polling intervals, timeouts, and restart behavior.

### daemon.poll_interval_seconds

- **Type:** `int`
- **Default:** `5`
- **Description:** Seconds between main-loop polling cycles. The daemon checks for claimable tasks at this interval.

### daemon.blocked_poll_interval_seconds

- **Type:** `int`
- **Default:** `5`
- **Description:** Seconds between polls when all tasks are blocked. A separate interval lets you reduce polling frequency during stalled periods.

### daemon.inbox_poll_interval_seconds

- **Type:** `int`
- **Default:** `5`
- **Description:** Seconds between inbox checks for new work items filed by the intake stage.

### daemon.max_iterations

- **Type:** `int`
- **Default:** `-1` (unlimited)
- **Description:** Maximum task iterations the daemon runs before stopping. Set to `-1` for unlimited. Useful for CI or testing to cap total work.

```json
{
  "daemon": {
    "max_iterations": 20
  }
}
```

### daemon.max_turns_per_invocation

- **Type:** `int`
- **Default:** `200`
- **Description:** Maximum conversation turns (tool calls) per model invocation before the daemon terminates the call.

### daemon.invocation_timeout_seconds

- **Type:** `int`
- **Default:** `3600`
- **Description:** Hard timeout in seconds for a single model invocation. The daemon kills the process after this limit.

### daemon.stall_timeout_seconds

- **Type:** `int`
- **Default:** `120`
- **Description:** If a model invocation produces no output for this many seconds, the daemon considers it stalled and kills the process.

### daemon.max_restarts

- **Type:** `int`
- **Default:** `3`
- **Description:** Maximum times the daemon restarts a failed model invocation for the same task before marking it as a failure.

### daemon.restart_delay_seconds

- **Type:** `int`
- **Default:** `2`
- **Description:** Seconds to wait before restarting a failed model invocation.

### daemon.log_level

- **Type:** `string`
- **Default:** `"info"`
- **Description:** Logging verbosity for the daemon process. Accepts standard levels: `debug`, `info`, `warn`, `error`.

```json
{
  "daemon": {
    "log_level": "debug"
  }
}
```

---

## logs

Controls NDJSON log retention (see ADR-012).

### logs.max_files

- **Type:** `int`
- **Default:** `100`
- **Description:** Maximum number of log files to retain. Oldest files are deleted when this limit is exceeded.

### logs.max_age_days

- **Type:** `int`
- **Default:** `30`
- **Description:** Maximum age in days for log files. Files older than this are deleted during cleanup.

### logs.compress

- **Type:** `bool`
- **Default:** `true`
- **Description:** Whether to gzip-compress rotated log files to save disk space.

```json
{
  "logs": {
    "max_files": 50,
    "max_age_days": 14,
    "compress": false
  }
}
```

---

## retries

Controls retry behavior for failed model invocations. Uses exponential backoff.

### retries.initial_delay_seconds

- **Type:** `int`
- **Default:** `30`
- **Description:** Initial delay before the first retry attempt. Subsequent retries use exponential backoff from this value.

### retries.max_delay_seconds

- **Type:** `int`
- **Default:** `600`
- **Description:** Maximum delay between retry attempts. Backoff is capped at this ceiling.

### retries.max_retries

- **Type:** `int`
- **Default:** `-1` (unlimited)
- **Description:** Maximum number of retries for a single invocation. Set to `-1` for unlimited retries, which is useful when transient provider errors are common.

```json
{
  "retries": {
    "initial_delay_seconds": 10,
    "max_delay_seconds": 120,
    "max_retries": 5
  }
}
```

---

## failure

Controls decomposition thresholds and hard failure caps. When a task fails repeatedly, the daemon can decompose it into smaller subtasks. These settings govern when and how deeply that happens.

### failure.decomposition_threshold

- **Type:** `int`
- **Default:** `10`
- **Description:** Number of consecutive failures on a task before the daemon attempts to decompose it into subtasks.

### failure.max_decomposition_depth

- **Type:** `int`
- **Default:** `5`
- **Description:** Maximum depth of recursive decomposition. Prevents infinite subdivision of failing work.

### failure.hard_cap

- **Type:** `int`
- **Default:** `50`
- **Description:** Absolute maximum failures across all decomposition attempts for a single logical task. Once hit, the task is marked as a hard failure and no further retries or decompositions are attempted.

```json
{
  "failure": {
    "decomposition_threshold": 5,
    "hard_cap": 20
  }
}
```

---

## git

Controls automatic commit behavior and branch verification.

### git.auto_commit

- **Type:** `bool`
- **Default:** `true`
- **Description:** Whether the daemon automatically commits changes after task completion. When `false`, the agent must commit manually.

### git.commit_message_format

- **Type:** `string`
- **Default:** `"wolfcastle: {action} [{node}]"`
- **Description:** Template for auto-commit messages. Supports `{action}` (what happened) and `{node}` (the tree node address) as placeholders.

```json
{
  "git": {
    "commit_message_format": "[wc] {action} on {node}"
  }
}
```

### git.verify_branch

- **Type:** `bool`
- **Default:** `true`
- **Description:** Whether the daemon verifies it is on the expected git branch before committing. Prevents accidental commits to the wrong branch.

### git.skip_hooks_on_auto_commit

- **Type:** `bool`
- **Default:** `true`
- **Description:** Whether auto-commits skip git hooks (pre-commit, commit-msg, etc.). Enabled by default because hooks that prompt for input will stall the daemon.

### git.commit_on_success

- **Type:** `bool`
- **Default:** `true`
- **Description:** Whether the daemon commits changes after successful task completion. Only applies when `auto_commit` is `true`.

### git.commit_on_failure

- **Type:** `bool`
- **Default:** `true`
- **Description:** Whether the daemon commits partial work after task failure. Only applies when `auto_commit` is `true`.

### git.commit_state

- **Type:** `bool`
- **Default:** `true`
- **Description:** Whether the `.wolfcastle/` state directory is included in daemon commits. When `false`, only code changes are committed. Only applies when `auto_commit` is `true`.

---

## overlap_advisory

Configures the overlap advisory system, which detects when multiple engineers are working on related files or nodes (see ADR-027, ADR-041).

### overlap_advisory.enabled

- **Type:** `bool`
- **Default:** `true`
- **Description:** Whether overlap detection is active. When enabled, the daemon checks for file and node overlap between active engineers.

### overlap_advisory.model

- **Type:** `string`
- **Default:** `"fast"`
- **Description:** Model used for overlap analysis. A lightweight model suffices since the analysis is primarily structural. Must match a key in [models](#models).

### overlap_advisory.threshold

- **Type:** `float64`
- **Default:** `0.3`
- **Description:** Overlap score threshold (0.0 to 1.0) above which an advisory is raised. Lower values are more sensitive.

```json
{
  "overlap_advisory": {
    "threshold": 0.5
  }
}
```

---

## summary

Controls the optional post-completion summary stage (see ADR-016). When a task completes, the daemon can generate a natural-language summary of what changed.

### summary.enabled

- **Type:** `bool`
- **Default:** `true`
- **Description:** Whether post-completion summaries are generated.

### summary.model

- **Type:** `string`
- **Default:** `"fast"`
- **Description:** Model used for summary generation. A lightweight model typically suffices. Must match a key in [models](#models).

### summary.prompt_file

- **Type:** `string`
- **Default:** `"summary.md"`
- **Description:** Prompt template for summary generation, resolved through the three-tier prompt system.

```json
{
  "summary": {
    "enabled": false
  }
}
```

---

## docs

Controls the documentation output directory.

### docs.directory

- **Type:** `string`
- **Default:** `"docs"`
- **Description:** Root directory for generated documentation, relative to the repository root.

```json
{
  "docs": {
    "directory": "documentation"
  }
}
```

---

## validation

Defines user-specified validation commands run after task completion. Use these to enforce build checks, tests, or linting as gates before a task is considered done.

### validation.commands

- **Type:** `[]ValidationCommand`
- **Default:** `[]` (empty)
- **Description:** List of shell commands to run as post-task validation. All commands must pass for validation to succeed.

Each `ValidationCommand` contains:

#### ValidationCommand.name

- **Type:** `string`
- **Description:** Human-readable name for this validation step. Shown in daemon output and logs.

#### ValidationCommand.run

- **Type:** `string`
- **Description:** Shell command to execute. Runs via the system shell. A non-zero exit code means validation failed.

#### ValidationCommand.timeout_seconds

- **Type:** `int`
- **Default:** `0` (no timeout)
- **Description:** Maximum seconds the command may run before being killed.

```json
{
  "validation": {
    "commands": [
      {
        "name": "build",
        "run": "make build",
        "timeout_seconds": 120
      },
      {
        "name": "test",
        "run": "go test ./...",
        "timeout_seconds": 300
      }
    ]
  }
}
```

---

## prompts

Controls prompt fragment inclusion and exclusion. Fragments are composable pieces of prompt text assembled by the context builder before each model invocation.

### prompts.fragments

- **Type:** `[]string`
- **Default:** `[]` (empty, meaning auto-discovery in alphabetical order)
- **Description:** Explicit ordered list of prompt fragment paths to include. When empty, fragments are discovered automatically. When set, only listed fragments are included, in the specified order.

### prompts.exclude_fragments

- **Type:** `[]string`
- **Default:** `[]` (empty)
- **Description:** Fragment paths to exclude from assembly. Applied after inclusion, so you can auto-discover and then remove specific fragments you don't want.

```json
{
  "prompts": {
    "exclude_fragments": ["fragments/verbose-logging.md"]
  }
}
```

---

## doctor

Configures the structural validation and repair command (see ADR-025). The `wolfcastle doctor` command diagnoses and fixes common project tree issues.

### doctor.model

- **Type:** `string`
- **Default:** `"mid"`
- **Description:** Model used for diagnostic analysis. Must match a key in [models](#models).

### doctor.prompt_file

- **Type:** `string`
- **Default:** `"doctor.md"`
- **Description:** Prompt template for the doctor command, resolved through the three-tier prompt system.

---

## unblock

Configures the unblock workflow (see ADR-028). The `wolfcastle unblock` command helps resolve blocked tasks by analyzing the blocking condition and suggesting remediation.

### unblock.model

- **Type:** `string`
- **Default:** `"heavy"`
- **Description:** Model used for unblock analysis. Uses a heavier model because unblocking often requires reasoning about complex dependencies. Must match a key in [models](#models).

### unblock.prompt_file

- **Type:** `string`
- **Default:** `"unblock.md"`
- **Description:** Prompt template for the unblock command, resolved through the three-tier prompt system.

---

## audit

Configures the codebase audit command (see ADR-029). The `wolfcastle audit` command runs a model-driven review of code changes and project state.

### audit.model

- **Type:** `string`
- **Default:** `"heavy"`
- **Description:** Model used for audit analysis. Uses a heavier model for thorough review. Must match a key in [models](#models).

### audit.prompt_file

- **Type:** `string`
- **Default:** `"audits/audit.md"`
- **Description:** Prompt template for the audit command, resolved through the three-tier prompt system.

---

## archive

Controls automatic archival of completed project trees. When all tasks in a tree finish, the daemon can archive the tree after a configurable delay.

### archive.auto_archive_enabled

- **Type:** `bool`
- **Default:** `true`
- **Description:** Whether completed project trees are automatically archived.

### archive.auto_archive_delay_hours

- **Type:** `int`
- **Default:** `24`
- **Description:** Hours to wait after a tree completes before archiving it. Gives engineers time to review results before the tree is moved to the archive.

### archive.archive_poll_interval_seconds

- **Type:** `int`
- **Default:** `300`
- **Description:** Seconds between archive eligibility checks. The daemon scans for completed trees at this interval.

```json
{
  "archive": {
    "auto_archive_delay_hours": 48,
    "archive_poll_interval_seconds": 600
  }
}
```

---

## knowledge

Controls the codebase knowledge file system, where agents record non-obvious discoveries about the codebase for future tasks. See [How It Works: Codebase Knowledge Files](how-it-works.md#codebase-knowledge-files) for the full explanation.

### knowledge.max_tokens

- **Type:** `int`
- **Default:** `2000`
- **Description:** Maximum token budget for the knowledge file. The entire file is injected into every task's context, so this directly controls context usage. When `wolfcastle knowledge add` would push the file over this limit, the command fails and asks for pruning.

```json
{
  "knowledge": {
    "max_tokens": 3000
  }
}
```

---

## task_classes

Defines behavioral classes that shape how the agent approaches work. Each key is a class identifier (e.g., `coding/go`, `architecture`), and each value is a `ClassDef`. For a full guide on using and creating classes, see [Task Classes](task-classes.md).

- **Type:** `map[string]ClassDef`
- **Default:** See tables below

Each `ClassDef` contains:

### ClassDef.description

- **Type:** `string`
- **Description:** Short description shown to the intake model for classification. Describes the class's focus areas and conventions.

### ClassDef.model

- **Type:** `string`
- **Default:** `""` (empty, meaning the stage's default model is used)
- **Description:** Optional model override for tasks assigned to this class. When set, the execution stage uses this model instead of its configured default. Must match a key in [models](#models).

```json
{
  "task_classes": {
    "coding/go-internal": {
      "description": "Internal Go services with team conventions, custom linter rules",
      "model": "heavy"
    }
  }
}
```

**Default language classes:**

| Class Key | Description |
|-----------|-------------|
| `coding/python` | Type hints, virtual environments, pytest, ruff/black, PEP 8 |
| `coding/javascript` | ESM vs CJS, Node vs browser, eslint, testing frameworks |
| `coding/typescript` | tsconfig strictness, type-only imports, declaration files |
| `coding/java` | Maven/Gradle, JUnit, checked exceptions |
| `coding/csharp` | .NET SDK, NuGet, xUnit/NUnit, nullable reference types |
| `coding/go` | gofmt, go vet, table-driven tests, error wrapping |
| `coding/rust` | cargo clippy, ownership/borrowing guidance, Result/Option patterns |
| `coding/cpp` | CMake, clang-tidy, RAII, smart pointers, UB avoidance |
| `coding/c` | Makefile conventions, valgrind, buffer safety, POSIX portability |
| `coding/ruby` | Bundler, RSpec/minitest, Rubocop |
| `coding/php` | Composer, PHPUnit, PSR standards |
| `coding/swift` | Xcode/SPM, XCTest, optionals, protocol-oriented patterns |
| `coding/kotlin` | Gradle, JUnit/kotest, null safety, coroutine conventions |
| `coding/scala` | sbt, ScalaTest, functional patterns, implicits guidance |
| `coding/shell` | shellcheck, POSIX compatibility, quoting rules, set -euo pipefail |
| `coding/sql` | Dialect awareness (Postgres, MySQL, SQLite), migration patterns, injection prevention |
| `coding/r` | tidyverse conventions, testthat, roxygen2, CRAN packaging |
| `coding/lua` | LuaRocks, busted, metatables, embedding considerations |
| `coding/elixir` | mix, ExUnit, OTP patterns, pattern matching, pipe operator |
| `coding/haskell` | cabal/stack, HSpec, monadic patterns, type-driven development |
| `coding/dart` | pub, flutter test, null safety, widget patterns |

**Default framework classes:**

| Class Key | Description |
|-----------|-------------|
| `coding/typescript/react` | Hooks, JSX, React Testing Library, component patterns, state management |
| `coding/typescript/vue` | Composition API, SFCs, Pinia, Vue Test Utils, Vue Router |
| `coding/typescript/angular` | Modules/standalone components, RxJS, dependency injection, Jasmine/Karma |
| `coding/typescript/nextjs` | App Router, Server Components, ISR/SSG/SSR, middleware, API routes |
| `coding/typescript/svelte` | Runes, load functions, form actions, server routes |
| `coding/javascript/react` | Same as TS/React but with PropTypes, no type annotations |
| `coding/javascript/node` | Express/Fastify patterns, middleware, async error handling, clustering |
| `coding/python/django` | MTV pattern, ORM, migrations, DRF, management commands, template conventions |
| `coding/python/fastapi` | Pydantic models, dependency injection, async endpoints, OpenAPI |
| `coding/python/flask` | Blueprints, extensions, application factory, Jinja2 |
| `coding/ruby/rails` | Convention over configuration, ActiveRecord, concerns, RSpec Rails, generators |
| `coding/ruby/sinatra` | Lightweight routing, modular style, Rack middleware |
| `coding/java/spring` | Auto-configuration, annotations, JPA, Spring Security, integration testing |
| `coding/csharp/dotnet` | Minimal APIs, Entity Framework, middleware pipeline, Razor conventions |
| `coding/php/laravel` | Eloquent, Blade, artisan, service providers, feature tests |
| `coding/php/symfony` | Bundles, Doctrine, Twig, event system, PHPUnit bridge |
| `coding/kotlin/android` | Jetpack Compose, ViewModel, Room, coroutines, instrumented tests |
| `coding/swift/ios` | SwiftUI views, Combine, Core Data, XCUITest, App lifecycle |
| `coding/dart/flutter` | Widget tree, state management (Riverpod/Bloc), platform channels, widget tests |
| `coding/elixir/phoenix` | LiveView, Ecto, PubSub, Channels, ExUnit with Sandbox |
| `coding/rust/actix` | Extractors, middleware, app state, integration tests |
| `coding/rust/tokio` | Spawning, channels, select!, graceful shutdown, tracing |

**Default non-language classes:**

| Class Key | Description |
|-----------|-------------|
| `architecture` | ADRs, dependency analysis, failure modes, decomposition |
| `research` | Source citation, accuracy over speed, structured output (uses `light` model) |
| `writing` | Reader-first, concrete examples, scannable structure |
| `design` | User goals, interaction sequences, edge states |
| `devops` | Dockerfile, GitHub Actions, Terraform, deployment safety |
| `data` | Schemas, pipelines, validation, visualization |
| `security` | OWASP awareness, threat modeling, dependency auditing |
| `testing` | Coverage strategy, fixture design, flaky test prevention |
| `audit` | Read-only review, gap recording, no fixes |
