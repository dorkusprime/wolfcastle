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
      "prompt_file": "class-go-coding.md",
      "description": "Writing or modifying Go source code"
    },
    "typescript-coding": {
      "prompt_file": "class-typescript-coding.md",
      "description": "Writing or modifying TypeScript/JavaScript source code"
    },
    "architecture": {
      "prompt_file": "class-architecture.md",
      "description": "System design, ADRs, decomposition, dependency analysis",
      "model": "heavy"
    },
    "research": {
      "prompt_file": "class-research.md",
      "description": "Information gathering, comparison, analysis",
      "model": "light"
    },
    "writing": {
      "prompt_file": "class-writing.md",
      "description": "Documentation, specs, guides, prose",
      "model": "light"
    },
    "design": {
      "prompt_file": "class-design.md",
      "description": "UI/UX design, wireframes, interaction patterns"
    },
    "audit": {
      "prompt_file": "class-audit.md",
      "description": "Verification and review of completed work"
    }
  }
}
```

### Field definitions

| Field | Required | Description |
|-------|----------|-------------|
| `prompt_file` | Yes | Behavioral prompt file, resolved via the three-tier system (`base/prompts/`, `custom/prompts/`, `local/prompts/`) |
| `description` | Yes | One-line description shown to the intake model so it can classify accurately |
| `model` | No | Model key override. If set, this class uses a different model than the execute stage default. Must reference a key in the top-level `models` map. |

### Validation

At daemon startup, the config loader validates:
1. Every class has a non-empty `prompt_file` and `description`.
2. If `model` is set, it references a valid key in `config.models`.
3. The `prompt_file` resolves to an existing file in at least one tier.

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
[contents of class-go-coding.md]

---

# Execute Stage
[execute.md]

---

# Current Task Context
[iteration context with node, task, deliverables, breadcrumbs]
```

The class section is inserted only when the task has a class and a matching config entry exists. Tasks with no class (or an empty class) get the prompt assembled exactly as today.

### Prompt file resolution

Class prompt files follow the same three-tier resolution as all other prompts:

1. `local/prompts/class-go-coding.md` (highest priority)
2. `custom/prompts/class-go-coding.md`
3. `base/prompts/class-go-coding.md` (ships with Wolfcastle)

Users override a built-in class's behavior by placing a file with the same name in `custom/` or `local/`.

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
   a. Resolve the behavioral prompt file
   b. If model override is set, use that model for invocation
   c. Pass the behavioral prompt to AssemblePrompt as the class section
5. If not found (empty class or missing config entry):
   a. Assemble prompt without a class section (today's behavior)
```

No changes to the execute stage's `AllowedCommands`, script reference filtering, or terminal marker handling. The class only affects which behavioral prompt is injected and optionally which model runs.

---

## Default Behavioral Prompts

Each default class ships a `.md` file in `base/prompts/`. These are starting points; users customize them in `custom/` or `local/`.

### class-go-coding.md

```markdown
You are working on a Go codebase. Follow Go conventions:
- Run `go fmt` before committing. Run `go vet` to catch issues.
- Test what you build. Table-driven tests for multiple cases.
- Errors are lowercase, wrapped with %w where callers inspect the chain.
- Use `_ =` for intentionally discarded errors.
- Package names are short, lowercase, no underscores.
- Check that your code compiles before signaling completion.
```

### class-typescript-coding.md

```markdown
You are working on a TypeScript codebase. Follow project conventions:
- Check for a tsconfig.json and respect its strictness settings.
- Run the project's lint and type-check commands before committing.
- Prefer typed interfaces over `any`. Use generics where they reduce duplication.
- Test what you build. Check for an existing test framework (jest, vitest, etc.) and match its patterns.
- Check that the project compiles cleanly before signaling completion.
```

### class-architecture.md

```markdown
You are making architectural decisions. Think in systems, not files:
- Identify the components affected and their interfaces.
- Consider failure modes, not just the happy path.
- Document decisions as ADRs when they change how the system works.
- Prefer reversible decisions. When a choice is hard to undo, say so explicitly.
- Decompose into smaller pieces if the scope exceeds what one task should carry.
```

### class-research.md

```markdown
You are gathering and synthesizing information. Accuracy matters more than speed:
- Cite sources. Name the document, URL, or dataset you drew from.
- Distinguish established fact from estimate from opinion.
- When sources disagree, present the range rather than picking a winner.
- Structure findings for a reader who wasn't in the room. Headings, tables, comparisons.
- Deliverables should stand alone as reference material.
```

### class-writing.md

```markdown
You are producing written documentation. Clarity is the priority:
- Write for the reader who will encounter this document cold, six months from now.
- Lead with what the reader needs to know, not the process you followed.
- Use concrete examples. Abstract descriptions without examples are incomplete.
- Keep paragraphs short. Use headings to create scannable structure.
- Match the project's existing voice and formatting conventions.
```

### class-design.md

```markdown
You are designing user-facing systems. Think from the user's perspective:
- Start with the user's goal, then work backward to the interface.
- Describe interactions as sequences: what the user does, what the system responds.
- Consider edge cases: empty states, error states, overloaded states.
- When proposing visual structure, describe layout and hierarchy, not pixel values.
- Validate designs against the project's existing patterns for consistency.
```

### class-audit.md

```markdown
You are verifying completed work, not creating new work:
- Read every file the preceding tasks produced. Check for completeness.
- Verify deliverables exist and contain what the task description promised.
- Run tests, linters, and build commands. Report what passes and what fails.
- If you find gaps, record them with `wolfcastle audit gap`. Do not fix them yourself.
- Your job is the report, not the repair. Summarize findings honestly.
```

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
