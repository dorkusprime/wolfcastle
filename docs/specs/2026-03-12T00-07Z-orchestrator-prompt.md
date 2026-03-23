# Spec: Orchestrator Prompt Structure

## Overview

This spec defines how the orchestrator prompt is assembled and delivered to the executing model at each iteration. The orchestrator prompt is the complete text the model receives as its system prompt plus user message when Wolfcastle invokes it via CLI shell-out.

The governing principle: **if it is deterministic, it belongs in Go code; if it requires reasoning, it belongs in the prompt.** The prompt tells the model what to think about. The Go binary handles everything that can be computed.

### Relationship to Ralph

Ralph's `PROMPT.md` was a single monolithic file that served as both orchestrator instructions and system prompt. The model was responsible for navigation, state mutation, status file editing, and deciding what to do. Wolfcastle splits these concerns: Go code handles navigation, state transitions, branch verification, failure counting, and archive generation. The prompt handles understanding tasks, studying code, deciding how to implement, writing breadcrumbs, and recognizing when work is blocked or needs decomposition. The prompt is also no longer a single file but an assembly of fragments, each with a clear purpose.

## 1. Prompt Assembly Order

The Go binary assembles the complete prompt from multiple sources before each model invocation. Assembly is deterministic and happens entirely in Go code. The model never sees the assembly logic -- it receives the final composed result.

### Assembly Layers (in order)

The system prompt is constructed by concatenating the following sections, separated by clear Markdown headings:

```
1. Rule fragments        (composable, from base/custom/local merge)
2. Script reference      (auto-generated from Go command definitions)
3. Stage prompt          (per-pipeline-stage instructions)
4. Iteration context     (current node, task, breadcrumbs, audit scope, failure state)
```

Each layer is a Markdown section within the system prompt. The model receives them as a single document with section boundaries.

### 1.1 Rule Fragments

Resolved from the three-tier merge (ADR-005, ADR-009, ADR-018):

1. **`base/rules/`** -- Wolfcastle-shipped defaults (git conventions, commit format, code style guidelines, ADR policy). Regenerated on `wolfcastle init` and `wolfcastle update`. Never edited by users.
2. **`custom/rules/`** -- Team-shared overrides and additions. Committed to git. A same-named file here completely replaces the base version (ADR-018).
3. **`local/rules/`** -- Personal overrides. Gitignored. A same-named file here completely replaces the custom or base version.

The Go binary resolves the merge, reads the surviving files, and concatenates them under a `# Project Rules` heading. The ordering of fragments within the section is determined by config (a `rules.order` array) or, if unspecified, by lexicographic sort of filenames.

Example resolved output:

```markdown
# Project Rules

## Git Conventions
[content from whichever tier won for git-conventions.md]

## Code Style
[content from whichever tier won for code-style.md]

## ADR Policy
[content from whichever tier won for adr-policy.md]
```

### 1.2 Script Reference

Auto-generated from Wolfcastle's Go command definitions (ADR-017). This section lists every command the model can call, with arguments, tree addressing syntax, expected behavior, and example invocations. It lives in `base/` as a generated file and is regenerated on `wolfcastle init` and `wolfcastle update`.

The section is injected under a `# Wolfcastle Commands` heading. The model treats this as its complete reference for interacting with Wolfcastle state. It does not discover commands at runtime.

Example structure:

```markdown
# Wolfcastle Commands

All state mutations happen through these commands. Do not edit state files directly.

## Task Commands

### wolfcastle task claim --node <path>
Mark a task as In Progress. Must be called before beginning work on any task.
Returns: JSON confirmation with node path and new status.

### wolfcastle task complete --node <path>
Mark a task as Complete. Only call after validation passes.
Returns: JSON confirmation. If this was the last task in the node, includes a
`parent_status` field indicating whether the parent node is now complete.

[... remaining commands ...]
```

The content of this section is the single source of truth. If a command exists in the reference, it works. If it does not exist in the reference, the model must not attempt to call it.

### 1.3 Stage Prompt

Each pipeline stage (ADR-006, ADR-013) has its own prompt file that defines the stage's purpose and behavioral expectations. These live in the three-tier hierarchy (`base/prompts/`, `custom/prompts/`, `local/prompts/`) and merge by file-level replacement (ADR-018).

The stage prompt file is referenced by name in the pipeline config:

```json
{
  "pipeline": {
    "stages": [
      { "name": "expand", "model": "fast", "prompt_file": "expand.md" },
      { "name": "file", "model": "mid", "prompt_file": "file.md" },
      { "name": "execute", "model": "heavy", "prompt_file": "execute.md" }
    ]
  }
}
```

Only the active stage's prompt file is included in the assembly. Section 6 of this spec covers how the prompt content differs per stage.

### 1.4 Iteration Context

Generated dynamically by Go code at the start of each iteration. This is not a file -- it is computed from the current tree state and injected as the final section of the system prompt (or as the user message, depending on provider conventions). See Section 2 for the full specification of iteration context contents.

### Assembly as User Message vs System Prompt

The exact partitioning between system prompt and user message depends on the model provider's conventions. Some providers (Claude CLI) accept a single prompt via `-p`. Others distinguish system and user messages. The Go binary handles this partitioning based on the model config. From the model's perspective, it receives all four layers as coherent, ordered input regardless of how they are technically delivered.

## 2. Iteration Context

The iteration context is the dynamic portion of the prompt, computed fresh by Go code before each model invocation. It gives the model everything it needs to understand where it is in the tree, what it should work on, and what has happened so far.

### 2.1 Contents

The iteration context includes the following, under a `# Current Iteration` heading:

**Current node information:**
- Node path (tree address from root, e.g. `my-project/auth-module/task-3`)
- Node type (leaf or orchestrator)
- Task description (the Markdown text describing what this task is)
- Task status (Not Started or In Progress -- `wolfcastle navigate` only finds the task, it does not claim it. The daemon calls `wolfcastle task claim` separately before model invocation, so the task should be In Progress by the time the model sees it)

**Breadcrumbs so far:**
- All breadcrumbs recorded against this node, in chronological order
- If this is a retry (failure count > 0), breadcrumbs from prior attempts are included so the model can learn from previous failures

**Audit scope:**
- The audit scope for this node (what systems and files the node's audit will verify)
- Breadcrumbs from sibling tasks that have already completed (so the model understands what has been done nearby)

**Failure state:**
- Current failure count for this node
- Decomposition threshold (e.g. "10")
- Current decomposition depth (e.g. "0" for an original task, "2" for a twice-decomposed subtask)
- Whether the decomposition prompt is active (see Section 5)

**Parent chain summary:**
- A compact representation of ancestor nodes from root to current, showing each node's name, status, and completion percentage. This gives the model a sense of where the current task fits in the larger project without requiring it to navigate the tree.

**Branch:**
- The branch name Wolfcastle is working on, verified by Go code before invocation

### 2.2 What Is NOT in the Iteration Context

The following are handled by Go code and are not communicated to the model:

- Navigation logic (which node to visit next -- `wolfcastle navigate` already resolved this)
- State file paths or JSON structure (the model interacts via commands, not files)
- Retry backoff timing (the daemon handles this)
- Archive generation details (deterministic, post-completion)
- Log rotation configuration
- Other engineers' project namespaces

### 2.3 Context Retention Across Phases

Within a single iteration, the model retains its full context window. The phase structure (Section 3) describes a sequence of activities the model performs in one invocation. There is no context boundary between phases within an iteration. The model reads code in the study phase and uses that knowledge in the implement phase without re-reading.

Context is lost **between iterations**. Each iteration is a fresh model invocation. The only continuity is what Go code injects (iteration context) and what the model wrote previously (breadcrumbs, committed code, ADRs, specs).

## 3. Phase Structure Within an Iteration

Each iteration follows a phase sequence that mirrors Ralph's Phase 2 (execute a task) but adapted for Wolfcastle's script-based architecture. The phases are described in the stage prompt (Section 1.3) -- they are instructions to the model, not enforced by Go code.

Navigation (Ralph's Phase 0 and Phase 1) is handled by Go code via `wolfcastle navigate` before the model is invoked. The model never navigates the tree. It receives its assignment and executes it.

### Phase A: Claim

The model calls `wolfcastle task claim --node <path>` to mark the task as In Progress.

In practice, the daemon calls `wolfcastle task claim` after navigation and before model invocation, so the task should already be In Progress when the model starts. If the iteration context shows the task is already In Progress, the model skips this call. The prompt instructs the model: "If the task is already In Progress, proceed to Study. Otherwise, claim it first."

### Phase B: Study

Before implementing anything:

1. **Check ADRs.** Read existing architecture decisions in `.wolfcastle/docs/decisions/`. If an ADR governs the system being touched, follow it. If the model is making a decision that would conflict with an existing ADR, it must either follow the ADR or create a new one superseding it via `wolfcastle adr create`.

2. **Search the codebase.** Do not assume something is missing -- confirm with code search. Read relevant files. Understand existing patterns and connections. This is the single most common failure mode in autonomous execution (inherited from Ralph): building what already exists.

3. **Read breadcrumbs from prior attempts.** If the failure count is > 0, the iteration context includes breadcrumbs from previous attempts. The model must study these to avoid repeating the same approach.

4. **On discovery tasks:** Search broadly. The task description is orientation, not exhaustive scope. Use grep and glob liberally. Check recent git history. The discovery task's output is the task list, not a verification of a predetermined list.

### Phase C: Implement

Do the work described by the task. Keep changes focused -- one task, one concern.

The model writes code, creates files, modifies configuration, writes documentation -- whatever the task requires. It interacts with the project's codebase directly (reading files, writing files, running commands) using whatever tools the model provider makes available (e.g. Claude Code's file editing tools, shell access).

The model interacts with Wolfcastle state exclusively through `wolfcastle` commands. It does not edit JSON state files.

During implementation, the model writes breadcrumbs via `wolfcastle audit breadcrumb --node <path> "text"` for anything the audit should verify later: files changed, patterns introduced, integration points touched, non-obvious decisions made. Breadcrumbs are lightweight -- one line per notable event.

### Phase D: Validate

Run validation before marking anything complete:

1. If the project config specifies validation commands (in `config.json` under a `validation` key), run those.
2. If no project-level validation is configured, and the task produced code changes, the model should run whatever build/test commands are standard for the project (detected from the codebase -- package.json scripts, Makefile targets, go test, etc.).
3. If the task produced only documentation or configuration changes, skip code validation.

If validation fails, fix the issues before proceeding. Do not commit code that fails validation. If the model cannot fix a failure after reasonable effort, it calls `wolfcastle task block --node <path> "reason"` and emits `WOLFCASTLE_BLOCKED`.

### Phase E: Record

1. Call `wolfcastle task complete --node <path>` to mark the task as Complete.
2. Write a breadcrumb summarizing what was done: `wolfcastle audit breadcrumb --node <path> "Completed: [brief description of what changed and why]"`.
3. If the model discovered something surprising -- a gotcha, a failed approach, a hidden dependency -- record it as a breadcrumb. These feed the archive (ADR-016) and inform future iterations.

### Phase F: Commit

**Every task that produces or modifies a file must be committed before yielding.** Work that is not committed does not survive context boundaries.

The model commits using the project's git conventions (specified in rule fragments). Each task should produce a coherent, reviewable commit. The model stages the specific files it touched. Wolfcastle state files (JSON in `projects/`) are committed automatically by the `wolfcastle task complete` command -- the model does not need to stage them separately.

### Phase G: Yield

After completing, validating, recording, and committing one task:

1. Output a brief summary: what task was completed, what was done, what the next expected task is.
2. Output `WOLFCASTLE_YIELD` as bare text on its own line.
3. **Stop. Make no further tool calls.** Do not read files. Do not run commands. Do not begin the next task. Do not plan the next task.

The daemon detects `WOLFCASTLE_YIELD` and terminates the model process. Any output or tool calls after the yield marker are lost.

**One iteration = one task = one yield.**

## 4. How the Model Calls Wolfcastle Commands

The model calls Wolfcastle commands by executing them as shell commands. Because Wolfcastle invokes models via CLI shell-out (ADR-013), and those CLIs typically provide shell access to the model, the model runs `wolfcastle` commands the same way it runs `git`, `grep`, or any other CLI tool.

Example flow within a single iteration:

```
# Phase A: Claim (if not already claimed)
$ wolfcastle task claim --node my-project/auth/task-3

# Phase B: Study
[model reads files, searches codebase using its native tools]

# Phase C: Implement
[model edits files, writes code]
$ wolfcastle audit breadcrumb --node my-project/auth/task-3 "Added JWT validation middleware in auth/middleware.go"

# Phase D: Validate
$ go test ./auth/...

# Phase E: Record
$ wolfcastle task complete --node my-project/auth/task-3
$ wolfcastle audit breadcrumb --node my-project/auth/task-3 "Completed: JWT validation middleware with expiry checking and refresh token support"

# Phase F: Commit
$ git add auth/middleware.go auth/middleware_test.go
$ git commit -m "Add JWT validation middleware with expiry and refresh support"

# Phase G: Yield
WOLFCASTLE_YIELD
```

### Command Output

All Wolfcastle commands that the model calls return JSON output (ADR-021). The model can parse this to confirm operations succeeded or to read state information. Errors return non-zero exit codes with a descriptive JSON error message.

### Node Path Convention

The model receives the current node path in the iteration context (Section 2.1). It uses this exact path in all `--node` arguments. The model does not need to discover or construct node paths -- they are provided.

For commands that target other nodes (e.g. `wolfcastle audit escalate` targeting a parent, or `wolfcastle task add` adding to a sibling), the iteration context includes the parent chain summary which provides the necessary paths.

## 5. The Decomposition Prompt

When the Go binary detects that a node's failure count has reached the decomposition threshold (ADR-019, default 10), it injects a decomposition section into the iteration context. This section replaces the normal phase instructions for that iteration.

### When Decomposition Is Offered

| Condition | What happens |
|-----------|-------------|
| Failures < threshold | Normal phase instructions. No mention of decomposition. |
| Failures = threshold, depth < max | Decomposition prompt injected. Model chooses to decompose or block. |
| Failures = threshold, depth = max | Auto-blocked by Go code. Model is not invoked. |
| Failures = hard cap (any depth) | Auto-blocked by Go code. Model is not invoked. |

### Decomposition Prompt Content

When active, the iteration context includes an additional section:

```markdown
## Decomposition Required

This task has failed [N] times. The failure threshold has been reached.

Review the breadcrumbs from prior attempts:
[breadcrumbs listed here]

You have two options:

### Option 1: Decompose
Break this task into smaller, more tractable subtasks. Use `wolfcastle task add`
to create new tasks under the current node, then call `wolfcastle task complete`
on the current task (the decomposition itself is the completion). Each new subtask
should be specific enough that a fresh model invocation can execute it without
the context that led to repeated failures here.

When decomposing:
- Each subtask inherits this node's audit scope
- Each subtask starts with zero failures
- The targeted audit task must remain last
- Do not create subtasks that merely retry the same approach -- restructure the work

### Option 2: Block
If the failures are due to an environmental issue (broken dependency, missing
infrastructure, incorrect requirements) that decomposition cannot solve, call
`wolfcastle task block --node <path> "reason"` with a detailed explanation.

Choose based on your judgment. Decomposition is appropriate when the task is too
large or too tangled for a single pass. Blocking is appropriate when the problem
is external to the task itself.
```

### Decomposition Mechanics

When the model chooses to decompose:

1. The model calls `wolfcastle task add --node <path> "description"` for each new subtask. These are children of the current node. The Go binary tracks their `decomposition_depth` as parent depth + 1.
2. The model calls `wolfcastle task complete --node <path>` on the current (parent) task. The Go binary marks it as decomposed, not simply complete.
3. The model emits `WOLFCASTLE_YIELD`.
4. On the next iteration, `wolfcastle navigate` finds the first uncompleted child of the decomposed task and proceeds with normal execution.

## 6. How the Prompt Differs Per Pipeline Stage

The default pipeline has three stages (ADR-006, ADR-013): expand, file, execute. Each stage has its own prompt file that replaces the stage prompt layer (Section 1.3). The rule fragments, script reference, and iteration context layers are shared across all stages.

### 6.1 Expand Stage (`expand.md`)

**Purpose:** Process the inbox. Convert raw ideas into structured project nodes or task entries.

**Model tier:** Typically fast/cheap (e.g. Haiku).

**What the iteration context contains:** The contents of the inbox file (raw bullets added by the user or by `wolfcastle inbox add`).

**What the prompt instructs:**
- Read each inbox item.
- Determine whether it is a task item (specific, bounded change) or a project seed (architectural goal, system to build).
- For task items: identify the appropriate existing node or create a new leaf via `wolfcastle task add`.
- For project seeds: create a new project via `wolfcastle project create`, with a discovery sub-project as the first child (the discovery-first pattern).
- Do not pre-define full task lists for project seeds. The discovery task defines the task list.
- After processing all items, yield.

**What the prompt does NOT instruct:** The expand stage never reads code, never implements anything, never runs validation. It is purely organizational.

### 6.2 File Stage (`file.md`)

**Purpose:** Organize and structure work that the expand stage created. Ensure nodes are well-formed, properly addressed, and ready for execution.

**Model tier:** Typically mid-tier (e.g. Sonnet).

**What the iteration context contains:** Newly created nodes from the expand stage that need structuring -- task descriptions to refine, audit scopes to define, validation commands to specify.

**What the prompt instructs:**
- Review newly created project and task nodes.
- Write detailed task descriptions in Markdown (the model writes these directly as task content, not via state files).
- Define audit scope for new nodes via `wolfcastle audit breadcrumb`.
- Ensure the discovery-first pattern is correctly applied where needed.
- Ensure targeted audit tasks are in the final position for every node.

### 6.3 Execute Stage (`execute.md`)

**Purpose:** Do the actual work. This is the primary stage and the one most directly descended from Ralph's Phase 2.

**Model tier:** Typically heavy/capable (e.g. Opus).

**What the iteration context contains:** Full iteration context as described in Section 2 -- current node, task, breadcrumbs, audit scope, failure state, parent chain.

**What the prompt instructs:** The full phase sequence described in Section 3 (Claim, Study, Implement, Validate, Record, Commit, Yield).

### 6.4 Summary (inline, per ADR-036)

> **Note:** The original design described the summary as a separate pipeline stage. ADR-036 superseded this: summaries are now generated inline by the executing model.

**Purpose:** Write a plain-language summary of what a completed node accomplished, used as the top section of the archive entry.

**How it works:** When `BuildIterationContext` detects that the current task is the last incomplete task in the node, it appends a "Summary Required" section to the prompt. The executing model emits a `WOLFCASTLE_SUMMARY:` marker alongside `WOLFCASTLE_COMPLETE`. The daemon parses this and stores it in `audit.result_summary`. No separate model invocation occurs.

## 7. Guardrails Expressed in the Prompt

These rules appear in the stage prompt (primarily `execute.md`) and are phrased as direct instructions to the model. They are behavioral constraints, not enforced by Go code (except where noted).

### 7.1 One Task Per Iteration

**Prompt text (paraphrased):** "After completing, validating, recording, and committing one task, yield immediately. Do not begin the next task. Do not plan the next task. Do not make further tool calls after yielding. One iteration = one task = one yield."

This is the single most important guardrail. Ralph had the same rule and it was the most frequently violated. The prompt must be emphatic and specific about what "stop" means: no reading files, no running commands, no output after the yield marker.

### 7.2 Commit Before Yield

**Prompt text (paraphrased):** "Every task that produces or modifies a file must be committed before yielding. Work that is not committed does not survive context boundaries. The next iteration starts fresh and can only see what is in git."

### 7.3 Claim Before Working

**Prompt text (paraphrased):** "If the task is not already In Progress, call `wolfcastle task claim` before doing any work. Status must reflect the active state before work begins, not after it ends."

### 7.4 Study Before Building

**Prompt text (paraphrased):** "Search the codebase before implementing. Confirm assumptions with code search. Do not assume something is missing because you do not see it in the current context. Check existing ADRs before making architectural decisions."

### 7.5 Validate Before Completing

**Prompt text (paraphrased):** "Run validation before marking anything complete. Do not commit code that fails validation. If you cannot fix a validation failure, block the task -- do not commit broken code."

### 7.6 Breadcrumbs Are Mandatory

**Prompt text (paraphrased):** "Write breadcrumbs during implementation for anything the audit should verify: files changed, patterns introduced, integration points touched, non-obvious decisions. Write a completion breadcrumb after marking the task complete. Breadcrumbs are the raw material for the archive and the context for future iterations. Write them as if the next reader has never seen this codebase."

### 7.7 Respect Existing ADRs

**Prompt text (paraphrased):** "Check `.wolfcastle/docs/decisions/` before making architectural decisions. If an ADR governs the system you are touching, follow it. If you need to deviate, create a new ADR superseding the existing one -- do not silently contradict a recorded decision."

### 7.8 Audit Task Is Always Last

**Prompt text (paraphrased):** "When adding tasks to a node, always place new tasks before the targeted audit task. The audit task must remain the final task in every node. No exceptions."

### 7.9 Do Not Skip Navigation Errors

**Prompt text (paraphrased):** "If the iteration context indicates an error (missing node, malformed state), do not attempt to work around it. Emit `WOLFCASTLE_BLOCKED` with a description of the problem."

## 8. Terminal Markers

The model emits terminal markers as bare text on their own line. The Go daemon detects these via substring match on the model's output. Markers must not be wrapped in code fences, backticks, or Markdown formatting.

| Marker | Meaning | When emitted | Pass type |
|--------|---------|-------------|-----------|
| `WOLFCASTLE_YIELD` | Task completed, iteration finished | After completing, recording, and committing one task | Execution only |
| `WOLFCASTLE_COMPLETE` | All work in the scoped tree is done | When the last task in the entire tree (or scoped subtree) completes, or when a planning pass verifies all success criteria are met | Both |
| `WOLFCASTLE_BLOCKED` | Cannot proceed | When a task is blocked and no unblocked tasks remain in the current scope, or when a planning pass cannot resolve an issue | Both |
| `WOLFCASTLE_SKIP` | Task already done or not applicable | When the model determines the assigned task does not need execution | Execution only |
| `WOLFCASTLE_CONTINUE` | Planning created new work | When a planning review pass finds gaps and creates new children | Planning only |

### Rules for Terminal Markers

1. **Exactly one marker per iteration.** The model emits one of the valid markers for its pass type and then stops.
2. **Nothing follows the marker.** No summary, no planning, no tool calls. The marker is the final output.
3. **Never use marker text in prose.** Do not write "I will now emit WOLFCASTLE_YIELD" or "this would normally trigger WOLFCASTLE_BLOCKED." The daemon cannot distinguish a marker from a sentence containing the marker text. If the model needs to discuss the concept, it must paraphrase without using the exact marker string.
4. **Bare text only.** No `\`WOLFCASTLE_YIELD\``, no `**WOLFCASTLE_YIELD**`, no `> WOLFCASTLE_YIELD`. Just the plain string on its own line.

### Marker Namespace Isolation

The daemon's `scanTerminalMarker` function accepts a `validMarkers` parameter that restricts which markers are recognized for a given pass type. When called with no arguments, it defaults to scanning for all five markers. The daemon calls it with restricted marker sets depending on context:

- **Execution passes:** COMPLETE, SKIP, BLOCKED, YIELD (not CONTINUE)
- **Planning passes:** COMPLETE, BLOCKED, CONTINUE (not YIELD or SKIP)

This prevents a planning model from accidentally emitting YIELD (which has task-completion semantics the planning pipeline doesn't handle) or an execution model from emitting CONTINUE (which has re-planning semantics the execution pipeline doesn't handle). The `scanTerminalMarker` function scans the full output and returns the highest-priority marker found among the valid set. Priority order: COMPLETE > SKIP > CONTINUE > BLOCKED > YIELD.

### Daemon Behavior on Marker Detection

**Execution pass markers:**
- **WOLFCASTLE_YIELD:** Daemon terminates the model process, increments the iteration counter, and begins the next iteration (navigation, prompt assembly, model invocation).
- **WOLFCASTLE_COMPLETE:** Daemon terminates the model process, runs archive generation if configured, and exits the daemon loop cleanly. For orchestrator audit tasks, propagates completion state to ancestors.
- **WOLFCASTLE_BLOCKED:** Daemon terminates the model process, logs the blocked state, and pauses. The daemon remains running but does not start new iterations until `wolfcastle task unblock` is called (or the user restarts with a different `--node` scope).
- **WOLFCASTLE_SKIP:** Daemon terminates the model process, treats the task as already done (the model determined no work was needed), and continues to the next iteration.

**Planning pass markers:**
- **WOLFCASTLE_COMPLETE:** Planning finished successfully. The daemon clears `needs_planning`, completes the orchestrator's audit task, propagates state, and continues to the next iteration.
- **WOLFCASTLE_BLOCKED:** The orchestrator cannot plan (external dependency, insufficient information). The daemon blocks the orchestrator and continues.
- **WOLFCASTLE_CONTINUE:** The review found gaps and created new children. The daemon clears `needs_planning` (it will be re-set when the new children complete) and continues.

### No Marker Detected

If the model process exits without emitting any terminal marker (crash, timeout, context window exhaustion), the daemon treats it as an invocation failure (ADR-019) and applies retry backoff. The task's failure counter is not incremented for invocation failures -- only for task-level failures (validation failures, explicit blocks).

## 9. What the Model Must NOT Do

These prohibitions are stated explicitly in the prompt. They represent the boundary between model responsibility and Go code responsibility.

### 9.1 Do Not Edit State Files Directly

State lives in JSON files under `.wolfcastle/system/projects/`. The model must never read, write, or modify these files. All state interaction happens through `wolfcastle` commands. This is the core invariant from ADR-002 and ADR-003.

**Why:** Ralph's most common corruption bugs came from the model editing STATUS.md incorrectly -- wrong status strings, malformed tables, forgotten updates. Wolfcastle eliminates this entire failure class by making state mutations deterministic.

### 9.2 Do Not Navigate the Tree

Navigation is handled by `wolfcastle navigate` before the model is invoked. The model receives its assignment in the iteration context. It does not read tree state to decide what to work on, does not traverse parent/sibling nodes to find work, and does not call `wolfcastle navigate` itself (unless explicitly documented in the script reference for a specific use case).

**Why:** Navigation is deterministic depth-first traversal. Having the model do it wastes tokens and introduces the possibility of incorrect traversal.

### 9.3 Do Not Skip Validation

If validation commands are configured or discoverable, the model must run them. Committing code that fails validation is a prompt violation. If validation cannot be fixed, the correct action is to block the task, not to skip validation and mark it complete.

### 9.4 Do Not Work on Multiple Tasks

One task per iteration. After yielding, stop. Do not "just quickly" handle the next task, run an extra check, or do preparatory work for a future task. If something needs doing, it is a task -- record it and let the next iteration handle it.

### 9.5 Do Not Emit Output After the Terminal Marker

The terminal marker is the last thing the model outputs. The daemon stops reading after detecting it. Anything after the marker is lost. More importantly, additional tool calls after the marker may execute but their results will not be captured or committed.

### 9.6 Do Not Manage Branches

Branch creation, checkout, and verification are handled by Go code (ADR-015). The model commits to whatever branch it finds itself on. It does not create branches, switch branches, or verify it is on the correct branch. If the branch is wrong, Go code would have blocked the iteration before invoking the model.

### 9.7 Do Not Generate Archive Entries

Archive generation is deterministic and handled by `wolfcastle archive add` (ADR-016). The model writes breadcrumbs during execution. Go code assembles the archive entry from those breadcrumbs. The optional summary stage (Section 6.4) is the model's only contribution to the archive beyond breadcrumbs.

### 9.8 Do Not Modify the Prompt or Rule Fragments

The model must not edit files in `base/`, `custom/`, or `local/` prompt and rule directories. These are configuration owned by the user and the Wolfcastle binary. The model works within the rules it is given, not on the rules themselves.

### 9.9 Do Not Call Undocumented Commands

The script reference (Section 1.2) is the complete list of available commands. If a command is not in the reference, it does not exist. The model must not guess at command names, flags, or syntax beyond what is documented.

## Appendix A: Assembly Pseudocode

```
func assemblePrompt(stage PipelineStage, node TreeNode, config Config) string {
    var sections []string

    // 1. Rule fragments (three-tier merge, sorted by config or filename)
    rules := mergeRuleFragments(config.BasePath, config.CustomPath, config.LocalPath)
    sections = append(sections, renderRulesSection(rules))

    // 2. Script reference (generated from Go command definitions)
    sections = append(sections, loadScriptReference(config.BasePath))

    // 3. Stage prompt (per-pipeline-stage file, three-tier merge)
    stagePrompt := resolveStagePrompt(stage.PromptFile, config)
    sections = append(sections, stagePrompt)

    // 4. Iteration context (computed from live state)
    ctx := buildIterationContext(node, config)
    sections = append(sections, renderIterationContext(ctx))

    return strings.Join(sections, "\n\n---\n\n")
}
```

## Appendix B: Comparison with Ralph

| Concern | Ralph | Wolfcastle |
|---------|-------|------------|
| Navigation | Model reads STATUS.md files, walks tree | Go code runs `wolfcastle navigate`, injects result |
| State mutation | Model edits STATUS.md directly | Model calls `wolfcastle task claim/complete/block` |
| Prompt structure | Single PROMPT.md file | Assembled from fragments + reference + stage prompt + context |
| Rule injection | Single CLAUDE-ralph.md concatenated | Composable fragments from base/custom/local |
| Command reference | Implicit in PROMPT.md | Auto-generated from Go source, injected into prompt |
| Branch management | Model creates and verifies branches | Go code handles branch creation and verification |
| Failure handling | "3 failures = blocked" | Configurable thresholds with decomposition option |
| Audit mechanics | Model edits audit.md | Model calls `wolfcastle audit breadcrumb/escalate` |
| Archive | None (STATUS.md was the record) | Deterministic rollup from breadcrumbs + optional model summary |
| Context between iterations | STATUS.md + .ralph-active fast path | Iteration context computed by Go, breadcrumbs in state |
| Terminal markers | RALPH_YIELD, RALPH_COMPLETE, RALPH_BLOCKED | WOLFCASTLE_YIELD, WOLFCASTLE_COMPLETE, WOLFCASTLE_BLOCKED |
| Multi-engineer | Not supported | Namespaced project directories per engineer |
