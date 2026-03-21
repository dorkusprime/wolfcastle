# Pipeline Stage Contract

This spec defines how pipeline stages are configured, invoked, and how they interact within the Wolfcastle daemon loop. It covers the full lifecycle from stage definition through prompt assembly, execution, output handling, error recovery, and iteration.

## Governing ADRs

- ADR-004: Model-agnostic design
- ADR-005: Composable rule fragments with sensible defaults
- ADR-006: Configurable pipelines
- ADR-009: Three-tier file layering (base/custom/local)
- ADR-012: NDJSON logs with per-iteration files
- ADR-013: Model invocation via CLI shell-out with pipeline configuration
- ADR-016: Archive format with deterministic rollup and model summary
- ADR-017: Script reference via prompt injection
- ADR-018: Merge semantics for config and prompt layering
- ADR-019: Failure handling, decomposition, and retry thresholds
- ADR-020: Daemon lifecycle and process management

## Related Specs

- [Dict-Format Pipeline Stages](2026-03-21T03-11Z-dict-format-stages.md): defines the map-based `pipeline.stages` schema, `stage_order` field, merge semantics, and migration contract.
- [Orchestrator Planning Pipeline](2026-03-17T00-00Z-orchestrator-planning-pipeline.md): defines the recursive planning pipeline for orchestrator nodes, including planning-specific prompt variants and markers.

---

## 1. Stage Definition Schema

Each stage is a named entry in the `pipeline.stages` object in `config.json`. The stage name is the map key; there is no `name` field inside the stage object itself. See the [Dict-Format Pipeline Stages spec](2026-03-21T03-11Z-dict-format-stages.md) for the full rationale behind this structure.

```json
{
  "pipeline": {
    "stages": {
      "<stage-name>": {
        "model": "<string, required>",
        "prompt_file": "<string, required>",
        "enabled": "<boolean, optional, default: true>",
        "skip_prompt_assembly": "<boolean, optional, default: false>",
        "allowed_commands": "<[]string, optional>"
      }
    },
    "stage_order": ["<stage-name>", "..."]
  }
}
```

### Field Reference

The stage name (map key) must match the pattern `[a-z][a-z0-9_-]*`. It serves as the unique identifier for the stage within the pipeline, appearing in log records, error messages, and stage-level configuration references.

| Field | Type | Required | Default | Description |
|-------|------|----------|---------|-------------|
| `model` | string | yes | -- | Key into the top-level `models` dictionary. Resolved at pipeline load time; a missing key is a fatal config error. |
| `prompt_file` | string | yes | -- | Filename (not path) of the stage-specific prompt. Resolved through the three-tier merge (base/custom/local) per ADR-009 and ADR-018. |
| `enabled` | boolean | no | `true` | When `false`, the stage is skipped entirely during pipeline execution. Allows opt-out without removing the stage from config. |
| `skip_prompt_assembly` | boolean | no | `false` | When `true`, the stage receives only its own `prompt_file` content as the prompt, without the full system prompt assembly (no rule fragments, no script reference). Useful for lightweight stages that do not need the full context. |
| `allowed_commands` | []string | no | `nil` | Restricts which wolfcastle CLI commands the stage may invoke. When nil/absent, all commands are allowed. |

### Model Definition (reference)

Models are defined separately in `config.json` under the `models` key (ADR-013). A stage references a model by its key:

```json
{
  "models": {
    "fast": {
      "command": "claude",
      "args": ["-p", "--model", "claude-haiku-4-5-20251001", "--output-format", "stream-json", "--dangerously-skip-permissions"]
    },
    "mid": {
      "command": "claude",
      "args": ["-p", "--model", "claude-sonnet-4-6", "--output-format", "stream-json", "--dangerously-skip-permissions"]
    },
    "heavy": {
      "command": "claude",
      "args": ["-p", "--model", "claude-opus-4-6", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions"]
    }
  }
}
```

---

## 2. Stage Invocation

Wolfcastle invokes each stage by constructing a CLI command from the model definition and the assembled prompt, then shelling out to that command as a child process.

### Command Construction

Given a stage with `"model": "heavy"` and the model definition above, Wolfcastle builds:

```
claude -p --model claude-opus-4-6 --output-format stream-json --verbose --dangerously-skip-permissions
```

The assembled prompt is piped to the command's stdin. Wolfcastle does **not** write the prompt to a temporary file; it is streamed directly.

### Streaming Invocation

Model invocations use a streaming pattern (`InvokeStreaming`) that enables real-time output via `wolfcastle follow`:

1. The child process's stdout is connected to a pipe
2. A scanner reads stdout line by line
3. Each line is simultaneously:
   - Written to the NDJSON log file as an `{"type": "assistant", "text": "..."}` record (via `Logger.AssistantWriter()`)
   - Captured in a buffer for the daemon to inspect after completion (terminal marker detection, etc.)
4. Stderr is captured separately in a buffer

This means:
- `wolfcastle follow` can display model output in real time by tailing the log file
- The daemon still has the full output for marker detection (`WOLFCASTLE_YIELD`, etc.)
- If no streaming is needed (e.g., lightweight stages), `Invoke()` falls back to direct buffer capture with no overhead

### Pseudocode

```
func InvokeStreaming(ctx, model, prompt, workDir, logWriter) (Result, error) {
    cmd := exec.Command(model.Command, model.Args...)
    cmd.Dir = workDir
    cmd.Stdin = strings.NewReader(prompt)
    cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

    stdoutPipe := cmd.StdoutPipe()
    cmd.Start()

    var captured bytes.Buffer
    scanner := bufio.NewScanner(stdoutPipe)
    for scanner.Scan() {
        line := scanner.Text()
        captured.WriteString(line + "\n")
        if logWriter != nil {
            fmt.Fprintln(logWriter, line)  // streams to NDJSON log
        }
    }

    cmd.Wait()
    return Result{Stdout: captured.String(), ...}, nil
}
```

### Working Directory

All stages execute with the working directory set to the project root (or the worktree root if `--worktree` was used, per ADR-015). The model navigates within the project via Wolfcastle commands and standard filesystem operations.

### Process Lifecycle

Per ADR-020, the Go daemon owns the child process:
- The child is spawned in its own process group.
- The daemon intercepts SIGTERM/SIGINT and propagates to the child.
- Context cancellation coordinates graceful shutdown.
- If the daemon receives a stop signal mid-stage, it waits for the current stage's child process to exit before shutting down. It does **not** proceed to the next stage.

---

## 3. Prompt Assembly

Each stage's prompt is assembled from multiple sources, layered in a defined order. The final prompt is a single string that becomes the model's system prompt.

### Assembly Order

For stages with `skip_prompt_assembly: false` (the default):

```
1. Rule fragments (merged via three-tier layering)
2. Script reference (auto-generated from Go command definitions)
3. Stage prompt (from prompt_file, merged via three-tier layering)
4. Iteration context (current node state, task details, tree position)
```

This is the same 4-layer model defined in the orchestrator prompt spec. The stage prompt (e.g., `execute.md`) contains the stage-specific instructions including the model's role within Wolfcastle. There is no separate "orchestrator prompt" layer — the stage prompt serves this purpose.

Each section is concatenated with clear delimiters so the model can distinguish them.

For stages with `skip_prompt_assembly: true`:

```
1. Stage-specific prompt (from prompt_file only)
2. Iteration context (current node state)
```

### Three-Tier Prompt Resolution

Each `prompt_file` is resolved through the three-tier merge (ADR-009, ADR-018):

1. **`base/prompts/{prompt_file}`** -- Wolfcastle-managed defaults, regenerated on `wolfcastle init` and `wolfcastle update`.
2. **`custom/prompts/{prompt_file}`** -- Team-shared overrides, committed to git.
3. **`local/prompts/{prompt_file}`** -- Personal overrides, gitignored.

A same-named file in a higher tier **completely replaces** the lower tier's version (file-level replacement, not partial merge, per ADR-018). If no file exists in any tier for a given `prompt_file`, that is a fatal config error.

The same resolution applies to rule fragments referenced in config.

### Script Reference Injection

Per ADR-017, the script reference is a prompt fragment generated from Wolfcastle's Go command definitions. It is:
- Regenerated on `wolfcastle init` and `wolfcastle update`.
- Stored in `base/prompts/script-reference.md`.
- Injected into every stage that has `skip_prompt_assembly: false`.
- Never separately maintained -- single source of truth in Go code.

### Iteration Context

Wolfcastle injects dynamic context at the end of the assembled prompt. This includes:
- The current node's tree address.
- The current node's state (task list, statuses, breadcrumbs, audit state).
- Failure history for the current node (failure count, decomposition depth), so the model is aware of prior attempts.
- After Action Reviews (AARs) from prior tasks in the node, providing structured retrospective data (objective, what happened, went well, improvements, action items) that subsequent tasks can learn from.
- The inbox contents (for stages that process the inbox, e.g. `intake`).

The iteration context is always injected regardless of `skip_prompt_assembly`.

### Example: Assembled Prompt for the `execute` Stage

```markdown
<!-- Rule Fragments -->
[git-conventions.md content]
[commit-format.md content]
[adr-policy.md content]
[...any additional rule fragments...]

<!-- Script Reference -->
The following commands are available to you:
[script-reference.md content]

<!-- Stage Prompt: execute.md -->
You are Wolfcastle, an autonomous software engineering agent...
You are in the execute stage. Your job is to complete the current task...
[execute.md content]

<!-- Iteration Context -->
Current node: attunement-tree/fire-implementation/task-3
Current state:
{...JSON state of the node...}
```

---

## 4. Input/Output Contract

### Stage Input

Every stage receives:

1. **The assembled prompt** (via stdin to the CLI command) -- as described in section 3.
2. **The working directory** -- project root or worktree root.
3. **The filesystem** -- the model can read any file in the project. What it can write depends on the CLI's permission flags (ADR-022).

Stages do **not** receive the output of the previous stage as direct input. Instead, stages communicate through **side effects**: changes to the filesystem (committed code, state file mutations via Wolfcastle commands). This is deliberate. Each stage reads the current state of the world, not a message from the previous stage.

### Stage Output

The model's stdout is captured by Wolfcastle for:
- **Logging** -- written to the current iteration's NDJSON log file (ADR-012).
- **`wolfcastle follow`** -- streamed to the terminal in real time.
- **Error detection** -- Wolfcastle inspects the exit code and output for invocation failures.

The model's **meaningful output** is not its stdout text but its **side effects**:
- Calls to `wolfcastle task claim`, `wolfcastle task complete`, `wolfcastle audit breadcrumb`, etc.
- Files created, modified, or deleted.
- Git commits made during execution.

### Per-Stage Expectations

| Stage | Expected Input State | Expected Side Effects |
|-------|---------------------|----------------------|
| `intake` | Inbox has items with status "new" | Reads inbox items, calls `wolfcastle project create` and `wolfcastle task add` directly to create projects and tasks in the tree. Runs in a parallel goroutine (ADR-064). |
| `execute` | A navigable task exists in `not_started` or `in_progress` state | Claims a task, does the work (writes code, runs tests, etc.), writes breadcrumbs, marks tasks complete or blocked. May create subtasks. When a task with `task_type: "spec"` completes, the daemon auto-creates a sibling `spec-review` task (see Spec Review Auto-Trigger below). |
| (summary) | N/A — inline | Per ADR-036, summaries are emitted inline by the executing model via `WOLFCASTLE_SUMMARY:` marker, not as a separate stage invocation. |

---

## 5. Stage Ordering and Dependencies

### Ordering

Stages execute in the order specified by the `pipeline.stage_order` array. The daemon iterates `stage_order` and looks up each name in the `pipeline.stages` map. Stage N must complete before stage N+1 begins. There is no parallel stage execution.

When `stage_order` is omitted from the resolved config, the daemon falls back to sorting the `stages` map keys alphabetically. This fallback is deterministic but rarely produces the desired order for a real pipeline, so the default `base/config.json` always includes an explicit `stage_order`.

Every name in `stage_order` must exist as a key in `stages`; a reference to a missing stage is a fatal config error. Every key in `stages` should appear in `stage_order`. A stage present in the map but absent from `stage_order` will never execute, and the config loader emits a warning for this case.

### Conditional Stages

Stages can be conditional in two ways:

1. **Static opt-out via `enabled: false`** -- the stage is always skipped. Configured at init time and does not change during execution. Useful for permanently removing a stage (e.g., disabling `expand` for projects that don't use an inbox).

2. **Dynamic skipping by the daemon** -- The intake stage runs in a parallel goroutine (ADR-064) and is automatically skipped in the main iteration pipeline. The intake goroutine checks for new inbox items independently.

| Stage | Skip Condition | Log Reason |
|-------|---------------|------------|
| `intake` | Always skipped in main loop (runs in parallel goroutine). In the goroutine, skipped when no inbox items have status `"new"`. | N/A |
| `execute` (and other custom stages) | No actionable task found by navigation. | N/A |
| (summary) | N/A — summary is generated inline by the executing model per ADR-036, not as a separate stage. | N/A |

When a stage is skipped, Wolfcastle emits a `stage_skip` log record with the stage name and reason, then proceeds to the next stage.

### Dependency Model

Stages do not declare explicit dependencies on each other. Instead, the shared filesystem and state creates an implicit dependency chain:
- `intake` creates projects and tasks that `execute` picks up.
- `execute` completes work that `summary` summarizes.

Because stages communicate through side effects (not direct output passing), a user can remove or reorder stages as long as the resulting state flow makes sense. For example, removing `intake` is valid if the user manually populates the task tree -- `execute` will still find tasks to work on.

---

## 6. The Default Pipeline

The default pipeline ships with Wolfcastle and is written into `config.json` by `wolfcastle init`. It implements the Ralph-style expand-file-execute flow with the addition of the summary stage.

### Full Default Configuration

The default `pipeline.stages` map contains two stages (ADR-064). The summary stage is controlled separately via the `summary` config key (see Section 7) and is not a pipeline stage — it runs conditionally based on `summary.enabled` after a node completes its audit.

```json
{
  "models": {
    "fast": {
      "command": "claude",
      "args": ["-p", "--model", "claude-haiku-4-5-20251001", "--output-format", "stream-json", "--dangerously-skip-permissions"]
    },
    "mid": {
      "command": "claude",
      "args": ["-p", "--model", "claude-sonnet-4-6", "--output-format", "stream-json", "--dangerously-skip-permissions"]
    },
    "heavy": {
      "command": "claude",
      "args": ["-p", "--model", "claude-opus-4-6", "--output-format", "stream-json", "--verbose", "--dangerously-skip-permissions"]
    }
  },
  "pipeline": {
    "stages": {
      "intake": {
        "model": "mid",
        "prompt_file": "intake.md"
      },
      "execute": {
        "model": "heavy",
        "prompt_file": "execute.md"
      }
    },
    "stage_order": ["intake", "execute"]
  },
  "summary": {
    "enabled": true,
    "model": "fast",
    "prompt_file": "summary.md"
  }
}
```

### Stage Descriptions

**intake** (model: mid)
- Reads raw inbox items and the existing project tree.
- Creates projects and tasks directly via `wolfcastle project create` and `wolfcastle task add`.
- Runs in a parallel goroutine, independent of the main execution loop (ADR-064).
- Uses a mid-tier model because structuring work requires judgment about task scope and organization.

**execute** (model: heavy)
- Navigates to the next actionable task via `wolfcastle navigate`.
- Claims the task, performs the work (writes code, runs tests, creates documentation).
- Writes breadcrumbs along the way via `wolfcastle audit breadcrumb`.
- Marks tasks complete or blocked.
- Handles audit tasks (the last task in every node, enforced by scripts per ADR-007).
- Uses the most capable model because execution requires deep reasoning and code generation.

**summary** (inline, per ADR-036)
- Not a separate pipeline stage. The executing model generates the summary inline when it completes the last task in a node.
- The daemon prompts the model to emit `WOLFCASTLE_SUMMARY:` alongside `WOLFCASTLE_COMPLETE`.
- The summary is stored in node state and later included in the archive entry (ADR-016).

---

## 7. Summary Generation (Inline via ADR-036)

> **Note:** The original design described the summary as a separate pipeline stage with its own model invocation. ADR-036 superseded this approach — summaries are now generated inline by the executing model, eliminating an extra model call.

### How It Works

When `BuildIterationContext` detects that the current task is the last incomplete task in the node, it appends a "Summary Required" section to the prompt. This instructs the executing model to emit a `WOLFCASTLE_SUMMARY:` marker in its output alongside `WOLFCASTLE_COMPLETE`. The daemon parses this marker and stores the text in the node's `audit.result_summary` field.

No separate model invocation occurs for summarization. The executing model generates the summary as part of its final task output.

### Configuration

```json
{
  "summary": {
    "enabled": true
  }
}
```

Setting `"enabled": false` disables summary generation entirely. When disabled:
- The "Summary Required" section is not appended to the prompt.
- Archive entries are still generated but without a model-written summary section — they contain breadcrumbs, audit results, and metadata only.
- This saves token cost for users who do not need narrative summaries.

### Output

The daemon's `applyModelMarkers` function detects the `WOLFCASTLE_SUMMARY:` prefix in model output and writes the text to `audit.result_summary` in the node's state. The archive rollup (ADR-016) reads this field when generating the archive entry.

---

## 7.5 Spec Review Auto-Trigger

When the daemon completes a task with `task_type: "spec"`, it automatically creates a sibling review task that audits the spec before downstream implementation begins. This is implemented in `daemon/spec_review.go`.

The review task:
- Has a deterministic ID derived from the spec task: `{specTaskID}-review`.
- Carries `task_type: "spec-review"` and is inserted before the audit task.
- References the same spec documents as the original spec task.
- Uses the `spec-review.md` prompt, which instructs the model to perform an adversarial review for logical gaps, missing signatures, contradictions, under-specified behavior, incomplete error handling, and missing edge cases.
- Emits `WOLFCASTLE_COMPLETE` if the spec passes review, or `WOLFCASTLE_BLOCKED` with specific issues if revision is needed.

If the review task blocks, `handleSpecReviewBlocked` feeds the review feedback back to the original spec task, resets it to `not_started`, and appends the issues to its body so the spec author can revise.

---

## 8. Error Handling Per Stage

Errors during stage execution fall into two categories with distinct handling.

### Model Invocation Failures

These are infrastructure-level failures: the CLI command crashes, returns a non-zero exit code with no output, the API is down, or the process is killed.

Per ADR-019:
- Wolfcastle applies exponential backoff starting at `retries.initial_delay_seconds` (default: 30).
- Backoff doubles each attempt up to `retries.max_delay_seconds` (default: 600).
- No retry cap by default (`retries.max_retries: -1` means unlimited).
- The daemon waits patiently and retries the **same stage** with the **same prompt**.
- Each retry is logged as a distinct record in the iteration's NDJSON log.

```json
{
  "retries": {
    "initial_delay_seconds": 30,
    "max_delay_seconds": 600,
    "max_retries": -1
  }
}
```

If `max_retries` is set to a positive integer and retries are exhausted, the daemon logs the failure and stops (it does not proceed to the next stage with a failed stage behind it).

### Task-Level Failures

These are failures detected by the model during execution: tests don't pass, validation fails, the model gets stuck. These are handled **within** the `execute` stage, not between stages. The model uses its judgment per ADR-019's escalation thresholds:

| Failure Count | Behavior |
|---------------|----------|
| 0 to N (default N=10) | Keep fixing. Model iterates on the problem. |
| N (default 10) | Model is prompted to consider decomposition. |
| Hard cap (default 50) | Node is auto-blocked regardless of model judgment. |

Task-level failures are tracked per node in state and persist across iterations. The failure counter resets when `wolfcastle task unblock` is called.

### Stage-Level Error Propagation

If a stage fails in a way that is not a retryable invocation error and not a task-level failure (e.g., the model produces no script calls and exits cleanly but accomplishes nothing), the daemon:
1. Logs the stage result.
2. Proceeds to the next stage. An unproductive stage is not treated as a blocking error -- the next iteration will try again with updated state.

The critical invariant: **the daemon never silently drops errors**. Every stage invocation, whether successful, retried, or failed, produces a log record.

### Per-Stage Error Log Record

```json
{
  "timestamp": "2026-03-12T18:45:32Z",
  "iteration": 7,
  "stage": "execute",
  "model": "heavy",
  "node": "attunement-tree/fire-implementation/task-3",
  "exit_code": 1,
  "error": "API rate limit exceeded",
  "retry_attempt": 2,
  "next_retry_delay_seconds": 120
}
```

---

## 9. Daemon Loop and Stage Iteration

The daemon loop is the top-level control flow that drives pipeline execution. Each pass through the loop is one **iteration**.

### Iteration Lifecycle

```
1. Branch verification (ADR-015)
2. Create iteration log file (ADR-012)
3. For each stage name in pipeline.stage_order (look up in pipeline.stages):
   a. Check if stage is enabled
   b. Check stage-specific preconditions (section 5)
   c. If skipped, log skip reason and continue
   d. Assemble prompt (section 3)
   e. Invoke stage (section 2)
   f. Handle errors (section 8)
   g. Log stage result
4. Commit state changes (state committed alongside code)
5. Check for stop signal (graceful shutdown per ADR-020)
6. If work remains, begin next iteration (go to 1)
7. If no work remains (all tasks complete/blocked, inbox empty), idle or exit
```

### One Iteration, All Stages

A single iteration runs through **all enabled stages** in `stage_order` sequence, skipping the intake stage which runs in its own parallel goroutine (ADR-064). The main loop focuses on finding and executing tasks.

Inbox processing happens concurrently. The intake goroutine creates projects and tasks in the background, and the next iteration of the main loop picks them up through navigation.

### Navigation Within Execute

The `execute` stage works on **one task per iteration**. At the start of the execute stage, the daemon calls `wolfcastle navigate` to find the next actionable task (depth-first traversal per ADR-014). The model then works on that single task.

If the task completes within the stage invocation, the daemon does **not** re-invoke execute for the next task in the same iteration. The next task is picked up in the next iteration. This keeps iterations bounded and predictable.

### Idle Behavior

When the daemon completes an iteration and finds no actionable work:
- No tasks in `not_started` or `in_progress` state.
- No nodes awaiting summary.

The daemon enters an idle poll loop. It periodically checks for new inbox items or unblocked tasks. The poll interval is configurable:

```json
{
  "daemon": {
    "blocked_poll_interval_seconds": 60
  }
}
```

### Iteration Logging

Each iteration produces its own NDJSON log file (ADR-012):

```
.wolfcastle/system/logs/0001-20260312T18-45Z.jsonl
.wolfcastle/system/logs/0002-20260312T18-47Z.jsonl
```

Within each log file, every stage invocation produces structured records including:
- Stage name and model used.
- Prompt hash (for debugging prompt assembly issues without logging the full prompt).
- Duration.
- Exit code.
- Node operated on (for `execute` and `summary`).
- Whether the stage was skipped and why.

### Stop Signal Handling

Per ADR-020, `wolfcastle stop` sends a graceful shutdown signal. The daemon:
1. Finishes the **current stage** invocation (waits for the child process to exit).
2. Does **not** proceed to subsequent stages in the current iteration.
3. Logs the partial iteration.
4. Exits cleanly.

`wolfcastle stop --force` kills the child process immediately via the process group.

---

## Appendix: Minimal Custom Pipeline Example

A simplified pipeline that skips inbox processing, going straight to execution:

```json
{
  "models": {
    "worker": {
      "command": "claude",
      "args": ["-p", "--model", "claude-sonnet-4-6", "--output-format", "stream-json", "--dangerously-skip-permissions"]
    }
  },
  "pipeline": {
    "stages": {
      "execute": {
        "model": "worker",
        "prompt_file": "execute.md"
      }
    },
    "stage_order": ["execute"]
  },
  "summary": {
    "enabled": false
  }
}
```

This is valid. The user manually populates the task tree and Wolfcastle only executes. No intake, no summary.

---

## Appendix: Pipeline Stage Lifecycle Diagram

```
wolfcastle start
       |
       v
  [Start inbox goroutine]  ─────────────────────────────────────┐
       |                                                         |
       v                                              [Inbox Goroutine]
  [Iteration N]                                          |
       |                                        poll inbox.json
       +---> Branch check (ADR-015)               |-- no new items? --> sleep, poll again
       |                                          |-- new items --> invoke mid model
       +---> execute stage                                         with intake.md
       |        |-- no actionable task? --> skip
       |        |-- else --> navigate, invoke heavy model with execute.md prompt
       |        |        |
       |        |        +-- invocation failure? --> retry with backoff
       |        |        +-- task failure? --> model iterates (ADR-019 thresholds)
       |        |        +-- task complete? --> proceed
       |
       +---> (summary generated inline by execute stage per ADR-036)
       |
       +---> Commit state
       +---> Check stop signal
       +---> Work remains? --> [Iteration N+1]
       +---> No work? --> idle poll
```
