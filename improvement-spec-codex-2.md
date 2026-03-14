# Wolfcastle Improvement Spec for Claude (Round 2, from Codex)

This document translates the findings in `../wolfcastle-eval-report-codex-2.md` into a concrete improvement plan for the Claude implementation.

Claude is already the strongest implementation in package structure, CLI breadth, prompt assembly, and validation architecture. The remaining work is not “add a lot more surface area.” It is to make the daemon path obey the same distributed-state invariants that the rest of the repo already enforces, then tighten runtime liveness handling so the operational story is as strong as the architecture.

Primary references:

- Evaluation report: `../wolfcastle-eval-report-codex-2.md`
- Canonical specs and ADRs in this repo: `docs/specs/`, `docs/decisions/`
- Best reference implementation for daemon/runtime correctness: `../wolfcastle-codex`
- Best reference implementation for process-group invocation and timeout handling: `../wolfcastle-gemini/internal/pipeline/pipeline.go:240-320`

---

## Priority 1: Fix Daemon-Side Propagation So Runtime Mutations Match ADR-024

### Problem

Your CLI helpers already implement correct upward propagation and root-index updates in one deterministic path:

- `cmd/helpers.go:10-86`

But the daemon bypasses that path during normal execution:

- claim path: `internal/daemon/daemon.go:246-259`
- post-model mutation / save path: `internal/daemon/daemon.go:328-390`

Today the daemon:

1. mutates the leaf node,
2. writes the leaf `state.json`,
3. partially updates the root index entry for the leaf,
4. does **not** recompute and persist parent `state.json` files in the same mutation path.

That violates the distributed-state contract in ADR-024 and is the single biggest correctness issue in this repo.

### Reference Implementations

Use your own CLI propagation helper as the first reference:

- `cmd/helpers.go:10-86`

Also study Codex’s runtime mutation flow, which always:

1. writes the leaf,
2. propagates to ancestors,
3. rebuilds or updates the root index:

- `../wolfcastle-codex/runtime_mutation.go:148-193`
- `../wolfcastle-codex/state_tree.go:122-170`

### Required Changes

#### 1.1 Centralize daemon mutations onto the same propagation path the CLI uses

Refactor `internal/daemon/daemon.go` so that every mutation that changes leaf state calls a single helper that:

- saves the leaf,
- recomputes parents,
- updates the root index once.

Do **not** leave the daemon with a “fast path” that only updates the leaf and root entry.

Concretely:

- Extract a daemon-local helper or move the shared logic out of `cmd/helpers.go:10-86` into a reusable internal package.
- Replace the manual claim/update path in `internal/daemon/daemon.go:246-259`.
- Replace the post-marker save path in `internal/daemon/daemon.go:328-390`.

The daemon and CLI should both call the same propagation primitive.

#### 1.2 Propagate after every state-affecting marker, not just on terminal save

`d.applyModelMarkers(result.Stdout, ns, nav)` at `internal/daemon/daemon.go:328-329` mutates `ns` in memory, but the code only writes the leaf afterward. That is too late and too shallow.

If a marker changes:

- task state,
- audit status,
- gaps/escalations that can block/unblock audit,
- decomposition / node shape,

you must run the shared save+propagate+root-index update path before returning from the iteration.

### Tests to Add

Add daemon-path tests that prove runtime execution updates ancestors, not just leaves.

Use Codex’s end-to-end style as a reference:

- `../wolfcastle-codex/main_test.go`

Add cases for:

1. daemon claim transitions a parent from `not_started` to `in_progress`
2. daemon completion of the last child transitions ancestors to `complete`
3. daemon block transitions an ancestor to `blocked` only when all non-complete siblings are blocked
4. root index entries for both leaf and ancestors are updated in the same iteration

---

## Priority 2: Make Stale `in_progress` Detection PID-Aware

### Problem

The validator currently emits `STALE_IN_PROGRESS` whenever exactly one task is `in_progress`:

- `internal/validate/engine.go:253-259`

That is not a stale-task detector. That is a “there is one active task” detector. It will warn during a normal healthy daemon run.

### Reference Implementations

Codex has the right daemon-PID liveness primitives:

- `../wolfcastle-codex/daemon.go:650-671` (`ensureNoLivePID`)
- `../wolfcastle-codex/daemon.go:684-727` (`recoverStaleDaemonState`, `livePIDFromFile`)

Those helpers implement the right model:

- parse PID file,
- check process liveness with signal 0,
- treat malformed/missing PID state separately from live daemon state.

### Required Changes

#### 2.1 Redefine `STALE_IN_PROGRESS`

Only emit `STALE_IN_PROGRESS` when:

1. exactly one task is `in_progress`, **and**
2. there is no live daemon PID for this workspace.

If a daemon is live, that state is expected and must not be reported as stale.

#### 2.2 Introduce daemon metadata/liveness helpers into validation

Add a validation-time helper that checks daemon artifacts before classifying a task as stale.

You can implement this in either:

- `internal/validate/engine.go`, or
- a small daemon-liveness helper package consumed by validation.

### Tests to Add

Add validation tests for:

1. one `in_progress` task + live daemon PID => no `STALE_IN_PROGRESS`
2. one `in_progress` task + dead/missing daemon PID => `STALE_IN_PROGRESS`
3. malformed PID file => warning about daemon state plus stale-task classification if appropriate

---

## Priority 3: Fix Daemon Shutdown to Cancel In-Flight Model Work, Not Just Close a Channel

### Problem

Signal handling in `Run` currently closes an internal shutdown channel:

- `internal/daemon/daemon.go:121-127`

But the in-flight stage invocation is running under the parent context and the signal path does not itself cancel that invocation. The daemon loop is structurally sound, but the shutdown behavior is weaker than the spec asks for.

### Reference Implementations

Codex uses `signal.NotifyContext` to root the supervisor in a cancelable context:

- `../wolfcastle-codex/daemon.go:129-134`

Codex also isolates the detached daemon into its own process group and signals the full group on stop:

- `../wolfcastle-codex/daemon.go:137-170`
- `../wolfcastle-codex/daemon.go:422-462`

Gemini’s model invocation shows the expected process-group handling at the invocation boundary:

- `../wolfcastle-gemini/internal/pipeline/pipeline.go:245-301`

### Required Changes

#### 3.1 Root the daemon in a cancelable signal context

Move from:

- “close shutdown channel and hope the loop notices”

to:

- a parent `context.Context` canceled by SIGINT/SIGTERM.

Then pass that context through to every model invocation path.

#### 3.2 Ensure stop semantics terminate the full invocation subtree

If your detached daemon or model subprocess is a process-group leader, stopping the daemon should signal the process group, not just the immediate process.

At minimum:

- detached daemon startup should create its own process group
- stop/force-stop should signal that process group

### Tests to Add

Add daemon tests for:

1. SIGTERM causes the active invocation context to cancel
2. detached stop kills the full process group
3. stop waits for clean exit when the child honors cancellation

---

## Priority 4: Fix Failure-Threshold Handling (`==` → `>=`) and Surface Decomposition Advice Consistently

### Problem

The daemon only reacts to the decomposition threshold on exact equality:

- `internal/daemon/daemon.go:367-378`

That is too brittle. If the threshold condition is missed once, subsequent iterations stop surfacing the transition.

### Reference Implementations

Codex uses `>=` in failure escalation logic:

- `../wolfcastle-codex/runtime_mutation.go:153-178`

Gemini also consistently uses `>=` when deciding which decomposition prompt to show:

- `../wolfcastle-gemini/internal/pipeline/pipeline.go:185-206`

### Required Changes

#### 4.1 Change the threshold comparison to `>=`

Update:

- `internal/daemon/daemon.go:367`

from exact equality to `>=`.

#### 4.2 Surface decomposition pressure in prompt context

You already have good prompt assembly architecture. Use it.

Add explicit decomposition guidance to the iteration context when:

- `failure_count >= decomposition_threshold`
- `decomposition_depth < max_decomposition_depth`

This is the same product decision Gemini makes in:

- `../wolfcastle-gemini/internal/pipeline/pipeline.go:185-206`

but Claude should implement it through its cleaner prompt/context split:

- `internal/pipeline/context.go`
- `internal/pipeline/prompt.go:10-57`

### Tests to Add

1. failure count above threshold continues surfacing decomposition guidance
2. depth at max transitions to block pressure instead of decomposition guidance
3. hard cap still wins over decomposition advice

---

## Priority 5: Keep the Validation Architecture, but Extend It to Cover Runtime-Specific Integrity

### Problem

Your validation engine structure is the best of the three. Keep it.

The gap is that it validates static structure better than runtime correctness. The biggest missing runtime-aware checks are:

- daemon-liveness-aware stale-task classification
- daemon/root-index propagation drift caused by the runtime path

### Reference Implementations

Validation architecture strength:

- `internal/validate/engine.go:41-264`

Deterministic fix staging strength:

- `internal/validate/fix.go:19-220`

Codex’s runtime-state repair coverage is less elegant architecturally, but it is more willing to check the actual artifacts on disk:

- `../wolfcastle-codex/doctor.go:1-490`

### Required Changes

Add validation categories/checks for:

1. daemon/runtime propagation drift
2. stale daemon artifacts vs live PID
3. root index state mismatches caused by daemon mutation paths

Prefer adding checks in the existing engine rather than growing a second validation mechanism.

---

## Priority 6: Do Not Sacrifice Package Discipline While Fixing Runtime Correctness

Claude’s package boundaries are already better than both Gemini and Codex. Do not flatten the repo while fixing the daemon.

Keep these strengths:

- config split: `internal/config/types.go:1-142`, `internal/config/validate.go:5-95`
- state split: `internal/state/navigation.go:12-111`, `internal/state/propagation.go:5-110`
- pipeline split: `internal/pipeline/prompt.go:10-57`, `internal/pipeline/context.go`
- embedded templates: `internal/project/embedded.go:1-6`, `internal/project/scaffold.go:16-139`

The right move is:

- share propagation and daemon-liveness helpers across packages,
- not to collapse everything into `cmd/daemon.go`.

---

## Recommended Order

1. Fix daemon propagation in `internal/daemon/daemon.go`
2. Make `STALE_IN_PROGRESS` PID-aware
3. Upgrade signal handling and process-group shutdown
4. Change failure threshold handling to `>=` and surface decomposition guidance in prompts
5. Add runtime-focused validation checks and tests

If you do only one thing, do Priority 1. That is the difference between “architecturally impressive” and “operationally correct.”
