# CLI Command Specification

This is the primary implementation reference for every Wolfcastle CLI command. All commands listed in ADR-021 are specified here with full synopsis, behavior, output format, exit codes, and error handling.

## Conventions

These conventions apply to every command unless explicitly stated otherwise.

### Tree Addressing

All `--node` flags accept a slash-delimited path from the tree root to the target node (ADR-008). Example: `attunement-tree/fire-impl/task-3`. The path is resolved relative to the engineer's project directory (`projects/{identity}/`), where identity is `{user}-{machine}` from `config.local.json` (ADR-009).

### Output Modes

Commands have a primary audience (user, model, or daemon). Model-facing and daemon-internal commands return structured JSON to stdout. User-facing commands return human-readable text to stdout by default. All commands accept a `--json` flag that forces JSON output regardless of audience.

### Error Output

Errors are written to stderr. When `--json` is active (or for model-facing commands), errors are also returned as JSON to stdout with a non-zero exit code:

```json
{
  "ok": false,
  "error": "descriptive message",
  "code": "ERROR_CODE"
}
```

### State Directory

All commands except `wolfcastle init` require a `.wolfcastle/` directory to exist in the current working directory or an ancestor. If not found, the command exits with code 1 and the message: `fatal: not a wolfcastle project (no .wolfcastle/ found)`.

### Identity Resolution

Commands that need the engineer's identity resolve it from `config.local.json` as `{user}-{machine}`. If `config.local.json` is missing or identity fields are absent, the command exits with code 1 and the message: `fatal: identity not configured. Run 'wolfcastle init' first.`

---

## wolfcastle init

### Synopsis

```
wolfcastle init [--force]
```

### Description

Scaffolds the `.wolfcastle/` directory structure in the current working directory and auto-populates engineer identity in `config.local.json`. This must be run before any other wolfcastle command. If `.wolfcastle/` already exists, the command is a no-op unless `--force` is passed.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--force` | boolean | No | `false` | Re-scaffold `.wolfcastle/`, regenerating `base/` and refreshing `config.local.json` identity without overwriting existing config files |

### Behavior

1. Check whether `.wolfcastle/` already exists in the current directory.
   - If it exists and `--force` is not set, print a message and exit 0.
   - If it exists and `--force` is set, proceed (skip directory creation, regenerate `base/`, refresh identity).
2. Create the `.wolfcastle/` directory structure (ADR-009):
   ```
   .wolfcastle/
     .gitignore
     config.json
     config.local.json
     base/
     custom/
     local/
     projects/
     archive/
     docs/
       decisions/
       specs/
     logs/
   ```
3. Write `.wolfcastle/.gitignore` with the content specified in ADR-009 (commit `config.json`, `custom/`, `projects/`, `archive/`, `docs/`; gitignore everything else).
4. Write a default `config.json` with sensible defaults for models, pipeline, failure thresholds, and log retention (ADRs 013, 006, 019, 012).
5. Auto-detect engineer identity:
   - `user`: result of `whoami`
   - `machine`: result of `hostname`, with `.local` suffix stripped if present
6. Write `config.local.json` with the detected identity. If the file already exists (force mode), update identity fields only; preserve any other keys the user has added.
7. Generate `base/` contents from the installed Wolfcastle binary (prompt fragments, rule defaults, script reference per ADR-017).
8. Create the engineer's project directory: `projects/{user}-{machine}/`.
9. Write an initial root state file at `projects/{user}-{machine}/state.json` with an empty node registry (ADR-024). This root index tracks the full tree structure for fast navigation.

### Output

```
Initialized Wolfcastle project in .wolfcastle/
Identity: wild-macbook
```

With `--force` on an existing project:

```
Reinitialized Wolfcastle project in .wolfcastle/
Identity: wild-macbook
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Success |
| 1 | Current directory is not writable |
| 1 | `config.local.json` exists but is malformed JSON (force mode) |

### Examples

```bash
# Initialize a new project
cd ~/projects/my-app
wolfcastle init

# Re-initialize after updating wolfcastle (regenerates base/)
wolfcastle init --force
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Directory not writable | `fatal: cannot write to current directory` | 1 |
| Already initialized (no --force) | `Wolfcastle already initialized in .wolfcastle/. Use --force to reinitialize.` | 0 |
| Malformed config.local.json (force mode) | `fatal: config.local.json exists but is not valid JSON` | 1 |

---

## wolfcastle start

### Synopsis

```
wolfcastle start [--node <path>] [--worktree <branch>] [-d]
```

### Description

Starts the Wolfcastle daemon, which begins the execution loop: navigate to the next active task, invoke the configured pipeline, commit results, and repeat. In foreground mode (default), the process runs in the current terminal. In background mode (`-d`), the process forks, writes a PID file, and returns control to the terminal.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--node <path>` | string | No | (none -- full tree) | Scope execution to a specific subtree. The path is a tree address (ADR-008) |
| `--worktree <branch>` | string | No | (none -- current branch) | Run in an isolated git worktree on the specified branch. Creates the branch from HEAD if it does not exist (ADR-015) |
| `-d` | boolean | No | `false` | Run as a background daemon. Forks, writes PID file, returns immediately |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity from `config.local.json`. Fail if not configured.
3. Load and merge configuration: `base/` defaults, `config.json`, `config.local.json` (deep merge per ADR-018).
4. **Stale PID check**: If `.wolfcastle/wolfcastle.pid` exists, check whether the PID is still a running wolfcastle process.
   - If running: print error and exit (another instance is already active).
   - If stale: remove the PID file and continue.
5. **Branch verification**: Record the current branch name via `git rev-parse --abbrev-ref HEAD`.
6. **Worktree setup** (if `--worktree` is specified):
   a. Check if the branch exists. If not, create it from HEAD.
   b. Create a git worktree at `.wolfcastle/worktrees/{branch-name}/`.
   c. Change the daemon's working directory to the worktree.
7. **Node scoping** (if `--node` is specified):
   a. Validate that the node path exists in the root `state.json` index.
   b. Record the scope root. Navigation will only traverse within this subtree.
8. **Startup validation** (ADR-025): Run the structural validation subset (the same checks used by `wolfcastle doctor`) to catch obvious issues early -- orphaned state files, missing index entries, stale `In Progress` states. If critical issues are found, print a warning and suggest running `wolfcastle doctor`.
9. **Self-healing check** (ADR-020): If any task in the tree is in `In Progress` state (from a previous crash), navigate to it first and let the model assess the state of uncommitted changes.
10. **Background mode** (if `-d` is specified):
    a. Fork the process.
    b. Write the child PID to `.wolfcastle/wolfcastle.pid`.
    c. Print the PID and return control to the terminal.
    d. The child continues with step 11.
11. **Execution loop** (repeats until stopped or no work remains):
    a. **Branch verification**: Confirm current branch matches the branch recorded at startup. If changed, emit `WOLFCASTLE_BLOCKED` and exit.
    b. **Navigate**: Call the navigation logic (equivalent to `wolfcastle navigate [--node <path>]`) to find the next active leaf task via depth-first traversal.
    c. If no active leaf found: all work is complete. Exit gracefully.
    d. **Pipeline execution**: For each stage in the configured pipeline (ADR-013):
       - Assemble the system prompt: rule fragments (ADR-005) + script reference (ADR-017) + stage prompt + current node context.
       - Invoke the model via CLI shell-out with the configured command and args.
       - Stream output to the log file (NDJSON per ADR-012).
       - Parse model output for script calls and execute them.
    e. **Log rotation**: Check log retention thresholds and clean up old files if needed (ADR-012).
    f. Loop back to step 11a.

### Output

Foreground mode:

```
Wolfcastle started (foreground)
Identity: wild-macbook
Scope: attunement-tree/fire-impl
Branch: main
```

Background mode:

```
Wolfcastle started (background, PID 48291)
Identity: wild-macbook
Scope: (full tree)
Branch: main
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | All work complete, or graceful stop via signal |
| 1 | `.wolfcastle/` not found |
| 1 | Identity not configured |
| 2 | Another wolfcastle instance is already running |
| 3 | Specified `--node` path does not exist in the tree |
| 4 | Branch changed during execution (`WOLFCASTLE_BLOCKED`) |
| 5 | Git worktree creation failed |

### Examples

```bash
# Start in foreground, full tree
wolfcastle start

# Start in background, scoped to a subtree, in an isolated worktree
wolfcastle start --node attunement-tree/fire-impl --worktree feature/fire -d

# Start scoped to a project
wolfcastle start --node attunement-tree
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| No identity | `fatal: identity not configured. Run 'wolfcastle init' first.` | 1 |
| Already running | `fatal: wolfcastle is already running (PID 48291). Use 'wolfcastle stop' first.` | 2 |
| Invalid node | `fatal: node 'foo/bar' not found in tree` | 3 |
| Branch changed | `WOLFCASTLE_BLOCKED: branch changed from 'main' to 'feature/x' during execution` | 4 |
| Worktree failure | `fatal: could not create worktree for branch 'feature/fire': {git error}` | 5 |

---

## wolfcastle stop

### Synopsis

```
wolfcastle stop [--force]
```

### Description

Stops a running Wolfcastle daemon. By default, sends a graceful stop signal that allows the current iteration to complete before exiting. With `--force`, performs a hard kill via SIGKILL.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--force` | boolean | No | `false` | Hard kill via SIGKILL instead of graceful SIGTERM |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. **PID file check**: Look for `.wolfcastle/wolfcastle.pid`.
   - If PID file exists: read the PID, verify the process is running.
     - If process is running: proceed to step 3.
     - If process is not running (stale PID): remove PID file, print message, exit 0.
   - If PID file does not exist: assume foreground mode. Print instructions to use Ctrl+C, exit 0.
3. **Signal the process**:
   - Without `--force`: Send SIGTERM to the daemon process. The daemon finishes its current iteration, cleans up worktrees if any, removes the PID file, and exits.
   - With `--force`: Send SIGKILL to the daemon process. Then send SIGKILL to the child process group (the model CLI process). Remove the PID file.
4. Wait for the process to exit (up to 30 seconds for graceful mode).
   - If the process does not exit within 30 seconds: print a message suggesting `--force`.
5. Clean up the PID file if it still exists.

### Output

Graceful stop:

```
Stopping Wolfcastle (PID 48291)... waiting for current iteration to finish.
Wolfcastle stopped.
```

Force stop:

```
Force-stopping Wolfcastle (PID 48291)...
Wolfcastle killed.
```

No daemon running (PID file missing):

```
No background Wolfcastle process found. If running in foreground, use Ctrl+C.
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Daemon stopped successfully, or no daemon was running |
| 1 | `.wolfcastle/` not found |
| 1 | Process could not be signaled (permission denied, etc.) |

### Examples

```bash
# Graceful stop (finishes current iteration)
wolfcastle stop

# Hard kill (immediate)
wolfcastle stop --force
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| Signal failed | `fatal: could not stop process 48291: {os error}` | 1 |
| Graceful timeout | `Wolfcastle did not stop within 30 seconds. Use 'wolfcastle stop --force' to kill it.` | 1 |

---

## wolfcastle status

### Synopsis

```
wolfcastle status [--node <path>] [--all] [--json]
```

### Description

Displays the current state of the work tree, including active task, progress summary, blocked tasks, and daemon status. Works regardless of whether the daemon is running.

When `--node` is provided, shows status for only the specified subtree, consistent with the `--node` flag on `start` and `navigate`.

By default, shows only the current engineer's tree. With `--all`, aggregates state across all engineer directories at runtime (ADR-024). The `--all` mode is read-only -- it scans other engineers' `projects/` directories for their root `state.json` and per-node `state.json` files but never writes to them.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--node <path>` | string | No | (none -- full tree) | Show status for only the specified subtree (ADR-008). Consistent with `--node` on `start` and `navigate` |
| `--all` | boolean | No | `false` | Aggregate status across all engineer directories, not just the current engineer's |
| `--json` | boolean | No | `false` | Output as structured JSON instead of human-readable text |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity from `config.local.json`.
3. Load the engineer's root state index at `projects/{identity}/state.json` (ADR-024).
4. Walk the tree structure from the root index, loading each node's co-located `state.json` to compute summary statistics:
   - Total tasks, completed tasks, in-progress tasks, blocked tasks, pending tasks.
   - Currently active task (if any is `In Progress`).
   - List of blocked tasks with reasons.
5. Check daemon status:
   - If `.wolfcastle/wolfcastle.pid` exists and the process is running: daemon is active.
   - If PID file exists but process is dead: stale (report as not running).
   - If no PID file: not running (or foreground -- cannot distinguish without more info).
6. **If `--all` is specified**: scan `projects/` for all engineer directories. For each, repeat steps 3-4 using that engineer's root `state.json`. Aggregate results grouped by engineer identity. Daemon status (step 5) still reflects only the local engineer's daemon.
7. Output the status.

### Output

Human-readable (default, single engineer):

```
Wolfcastle Status
=================
Daemon:    running (PID 48291, background)
Identity:  wild-macbook
Branch:    main
Scope:     attunement-tree

Progress:  12/37 tasks complete (32%)
Active:    attunement-tree/fire-impl/task-3 (In Progress)
Blocked:   2 tasks
  - attunement-tree/water-impl/task-1: "Missing upstream API dependency"
  - attunement-tree/earth-impl/task-5: "Flaky test infrastructure"
Pending:   23 tasks
```

Human-readable (`--all`):

```
Wolfcastle Status (all engineers)
==================================
Daemon:    running (PID 48291, background)  [local]

wild-macbook:
  Progress:  12/37 tasks complete (32%)
  Active:    attunement-tree/fire-impl/task-3 (In Progress)
  Blocked:   2 tasks
  Pending:   23 tasks

alice-workstation:
  Progress:  8/20 tasks complete (40%)
  Active:    auth-refactor/oauth2-impl/task-2 (In Progress)
  Blocked:   0 tasks
  Pending:   12 tasks
```

JSON (`--json`, single engineer):

```json
{
  "ok": true,
  "daemon": {
    "running": true,
    "pid": 48291,
    "mode": "background",
    "scope": "attunement-tree",
    "branch": "main"
  },
  "identity": "wild-macbook",
  "progress": {
    "total": 37,
    "complete": 12,
    "in_progress": 1,
    "blocked": 2,
    "not_started": 23,
    "percent_complete": 32
  },
  "active_task": {
    "node": "attunement-tree/fire-impl/task-3",
    "state": "in_progress"
  },
  "blocked_tasks": [
    {
      "node": "attunement-tree/water-impl/task-1",
      "reason": "Missing upstream API dependency"
    },
    {
      "node": "attunement-tree/earth-impl/task-5",
      "reason": "Flaky test infrastructure"
    }
  ]
}
```

JSON (`--json --all`):

```json
{
  "ok": true,
  "daemon": {
    "running": true,
    "pid": 48291,
    "mode": "background",
    "scope": "attunement-tree",
    "branch": "main"
  },
  "engineers": [
    {
      "identity": "wild-macbook",
      "progress": {
        "total": 37,
        "complete": 12,
        "in_progress": 1,
        "blocked": 2,
        "not_started": 23,
        "percent_complete": 32
      },
      "active_task": {
        "node": "attunement-tree/fire-impl/task-3",
        "state": "in_progress"
      },
      "blocked_tasks": [
        {
          "node": "attunement-tree/water-impl/task-1",
          "reason": "Missing upstream API dependency"
        }
      ]
    },
    {
      "identity": "alice-workstation",
      "progress": {
        "total": 20,
        "complete": 8,
        "in_progress": 1,
        "blocked": 0,
        "not_started": 12,
        "percent_complete": 40
      },
      "active_task": {
        "node": "auth-refactor/oauth2-impl/task-2",
        "state": "in_progress"
      },
      "blocked_tasks": []
    }
  ]
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Success |
| 1 | `.wolfcastle/` not found |
| 1 | Identity not configured |

### Examples

```bash
# Human-readable status (current engineer only)
wolfcastle status

# Status for a specific subtree
wolfcastle status --node attunement-tree/fire-impl

# Status across all engineers
wolfcastle status --all

# JSON status (for scripting or model consumption)
wolfcastle status --json

# JSON status for all engineers
wolfcastle status --all --json
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| No identity | `fatal: identity not configured. Run 'wolfcastle init' first.` | 1 |
| No root state file | `fatal: no tree state found at projects/{identity}/state.json. Start wolfcastle to initialize.` | 1 |

---

## wolfcastle follow

### Synopsis

```
wolfcastle follow [--lines <n>]
```

### Description

Tails the active iteration's model output in real time. Works in both foreground and background modes by streaming from NDJSON log files (ADR-012). Finds the highest-numbered log file and tails it, automatically switching to the next file when a new iteration starts.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--lines <n>` | integer | No | `20` | Number of historical lines to show before streaming |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Find the highest-numbered log file in `.wolfcastle/logs/`.
   - If no log files exist, wait for one to appear (with a timeout message after 5 seconds).
3. Print the last `n` lines of the current log file (parsed from NDJSON, formatted for human readability).
4. Tail the file, printing new lines as they are appended.
5. Watch for new iteration files. When a new, higher-numbered file appears, switch to tailing it and print a separator:
   ```
   --- iteration 0042 started ---
   ```
6. Continue until the user presses Ctrl+C, or the daemon exits (detected by checking the PID file / process status periodically).
7. When the daemon exits, print a final message and exit.

### Output

Streaming human-readable output parsed from NDJSON log entries:

```
[18:45:03] Navigating to attunement-tree/fire-impl/task-3
[18:45:04] Executing pipeline stage: execute (heavy)
[18:45:05] Model: Analyzing the fire implementation stamina cost...
[18:45:12] Script call: wolfcastle task claim --node attunement-tree/fire-impl/task-3
[18:45:12] Task claimed: attunement-tree/fire-impl/task-3
...
--- iteration 0042 started ---
[18:47:01] Navigating to attunement-tree/fire-impl/task-4
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | User interrupted (Ctrl+C) or daemon exited |
| 1 | `.wolfcastle/` not found |

### Examples

```bash
# Follow with default 20 lines of history
wolfcastle follow

# Follow with 100 lines of history
wolfcastle follow --lines 100
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| No logs, no daemon | `No log files found and no daemon running. Start wolfcastle first.` | 1 |

---

## wolfcastle update

### Synopsis

```
wolfcastle update
```

### Description

Updates the Wolfcastle binary to the latest version and regenerates the `base/` directory contents (prompt fragments, rule defaults, script reference). Equivalent to updating the binary and running `wolfcastle init --force` for the `base/` regeneration step only.

### Arguments and Flags

None.

### Behavior

1. Check for the latest version of the Wolfcastle binary via the release channel (GitHub releases or Homebrew, depending on installation method).
2. If already on the latest version, print a message and exit 0.
3. Download and install the new binary.
4. Locate `.wolfcastle/` directory. If not found, skip `base/` regeneration (the user may be updating outside a project).
5. Regenerate `base/` contents from the new binary version (prompt fragments, rule defaults, script reference per ADR-017).
6. Print the old and new version numbers and what was regenerated.

### Output

```
Updating Wolfcastle: v0.3.1 -> v0.4.0
Binary updated.
Regenerated base/ from v0.4.0.
```

Already up to date:

```
Wolfcastle is already at the latest version (v0.4.0).
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Success (updated or already current) |
| 1 | Network error (cannot reach release channel) |
| 1 | Permission denied (cannot write binary location) |

### Examples

```bash
# Update wolfcastle
wolfcastle update

# Update and verify
wolfcastle update && wolfcastle status
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Network failure | `fatal: could not reach update server: {error}` | 1 |
| Permission denied | `fatal: cannot write to {binary path}: permission denied` | 1 |
| Checksum mismatch | `fatal: download integrity check failed. Try again.` | 1 |

---

## wolfcastle task add

### Synopsis

```
wolfcastle task add --node <path> "<description>"
```

### Description

Adds a new task to a leaf node's task list, inserting it before the audit task (which is always last per ADR-007). The target node must be a leaf node. The new task is created in `not_started` state. This command is called by both models (during discovery/decomposition) and users.

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Tree address of the leaf node to add the task to (ADR-008) |
| `"<description>"` | string (positional) | Yes | -- | Human-readable description of the task |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the root state index at `projects/{identity}/state.json` (ADR-024).
4. Validate the `--node` path exists in the tree index.
5. Validate the target node is a leaf node (tasks live in leaves, not orchestrators).
6. Load the leaf's `state.json`.
7. Generate the next task ID (`task-N` where N is one greater than the current highest).
8. Insert a new task entry into the leaf's `tasks` array before the audit task:
   - `id`: the generated task ID
   - `description`: the provided description
   - `state`: `"not_started"`
   - `failure_count`: `0`
9. Write the updated leaf `state.json`. Adding a `not_started` task does not change the node's state, so no propagation is needed.
10. Output the result as JSON (model-facing command).

### Output

```json
{
  "ok": true,
  "action": "task_added",
  "node": "attunement-tree/fire-impl",
  "task_id": "task-4",
  "task_address": "attunement-tree/fire-impl/task-4",
  "description": "Wire stamina cost into fire ability",
  "state": "not_started"
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Task added successfully |
| 1 | `.wolfcastle/` not found or identity not configured |
| 2 | Node path not found |
| 3 | Target node is not a leaf (tasks can only be added to leaves) |
| 4 | Description is empty |

### Examples

```bash
# Add a task to a leaf node
wolfcastle task add --node attunement-tree/fire-impl "Wire stamina cost into fire ability"

# Add a task to a deeply nested leaf
wolfcastle task add --node attunement-tree/balance-pass/pvp "Adjust fire spell damage for PvP"
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Node not found | `{"ok": false, "error": "node 'foo/bar' not found in tree", "code": "NODE_NOT_FOUND"}` | 2 |
| Node is not a leaf | `{"ok": false, "error": "cannot add tasks to orchestrator node 'foo/bar' — use wolfcastle project create for child nodes", "code": "INVALID_NODE_TYPE"}` | 3 |
| Empty description | `{"ok": false, "error": "description must not be empty", "code": "EMPTY_DESCRIPTION"}` | 4 |

---

## wolfcastle task claim

### Synopsis

```
wolfcastle task claim --node <path>
```

### Description

Marks a task as `in_progress`. The task must currently be in `not_started` state. Only one task should be `in_progress` at a time (serial execution per ADR-014), but this invariant is enforced by the daemon, not by this command -- the command validates only the target task's own state.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Tree address of the task to claim (ADR-008) |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the root state index at `projects/{identity}/state.json` (ADR-024).
4. Validate the `--node` path exists and points to a leaf task.
5. Load the node's co-located `state.json` at `projects/{identity}/{node-path}/state.json`.
6. Validate the task's current state is `not_started`.
7. Update the task's state to `in_progress`.
8. Record a timestamp for when the task was claimed.
9. Write the updated node `state.json`.
10. Output the result as JSON.

### Output

```json
{
  "ok": true,
  "action": "task_claimed",
  "node": "attunement-tree/fire-impl/task-3",
  "state": "in_progress",
  "previous_state": "not_started"
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Task claimed successfully |
| 1 | `.wolfcastle/` not found or identity not configured |
| 2 | Node path not found |
| 3 | Node is not a leaf task |
| 4 | Task is not in `not_started` state |

### Examples

```bash
# Claim the next task
wolfcastle task claim --node attunement-tree/fire-impl/task-3

# Claim after navigating
NODE=$(wolfcastle navigate --json | jq -r '.node')
wolfcastle task claim --node "$NODE"
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Node not found | `{"ok": false, "error": "node 'foo/bar' not found in tree", "code": "NODE_NOT_FOUND"}` | 2 |
| Not a leaf | `{"ok": false, "error": "node 'foo/bar' is not a leaf task", "code": "INVALID_NODE_TYPE"}` | 3 |
| Wrong state | `{"ok": false, "error": "task 'foo/bar' is 'complete', expected 'not_started'", "code": "INVALID_STATE"}` | 4 |

---

## wolfcastle task complete

### Synopsis

```
wolfcastle task complete --node <path>
```

### Description

Marks a task as `complete`. The task must currently be in `in_progress` state. When a task completes, the parent orchestrator is checked -- if all its children are complete, the parent itself may become eligible for its audit task.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Tree address of the task to complete (ADR-008) |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the root state index at `projects/{identity}/state.json` (ADR-024).
4. Validate the `--node` path exists and points to a leaf task.
5. Load the node's co-located `state.json` at `projects/{identity}/{node-path}/state.json`.
6. Validate the task's current state is `in_progress`.
7. Update the task's state to `complete`.
8. Record a completion timestamp.
9. Write the updated node `state.json`.
10. Check the parent node's `state.json`: if all children are now `complete`, update the parent's state metadata to indicate readiness for audit (but do not change the parent's own state -- the audit task handles that).
11. Output the result as JSON.

### Output

```json
{
  "ok": true,
  "action": "task_completed",
  "node": "attunement-tree/fire-impl/task-3",
  "state": "complete",
  "previous_state": "in_progress",
  "parent_ready_for_audit": false
}
```

When the parent becomes ready for audit:

```json
{
  "ok": true,
  "action": "task_completed",
  "node": "attunement-tree/fire-impl/task-5",
  "state": "complete",
  "previous_state": "in_progress",
  "parent_ready_for_audit": true,
  "parent": "attunement-tree/fire-impl"
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Task completed successfully |
| 1 | `.wolfcastle/` not found or identity not configured |
| 2 | Node path not found |
| 3 | Node is not a leaf task |
| 4 | Task is not in `in_progress` state |

### Examples

```bash
# Complete the current task
wolfcastle task complete --node attunement-tree/fire-impl/task-3

# Complete and check if parent is ready for audit
wolfcastle task complete --node attunement-tree/fire-impl/task-5
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Node not found | `{"ok": false, "error": "node 'foo/bar' not found in tree", "code": "NODE_NOT_FOUND"}` | 2 |
| Not a leaf | `{"ok": false, "error": "node 'foo/bar' is not a leaf task", "code": "INVALID_NODE_TYPE"}` | 3 |
| Wrong state | `{"ok": false, "error": "task 'foo/bar' is 'not_started', expected 'in_progress'", "code": "INVALID_STATE"}` | 4 |

---

## wolfcastle task block

### Synopsis

```
wolfcastle task block --node <path> "<reason>"
```

### Description

Marks a task as `blocked` with an explanation of why it cannot proceed. The task must be in `in_progress` state (only claimed tasks can be blocked). Blocked tasks are skipped by navigation until explicitly unblocked.

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Tree address of the task to block (ADR-008) |
| `"<reason>"` | string (positional) | Yes | -- | Human-readable explanation of why the task is blocked |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the root state index at `projects/{identity}/state.json` (ADR-024).
4. Validate the `--node` path exists and points to a leaf task.
5. Load the node's co-located `state.json` at `projects/{identity}/{node-path}/state.json`.
6. Validate the task's current state is `in_progress`.
7. Update the task's state to `blocked`.
8. Record the block reason and timestamp.
9. Write the updated node `state.json`.
10. Output the result as JSON.

### Output

```json
{
  "ok": true,
  "action": "task_blocked",
  "node": "attunement-tree/water-impl/task-1",
  "state": "blocked",
  "previous_state": "in_progress",
  "reason": "Missing upstream API dependency"
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Task blocked successfully |
| 1 | `.wolfcastle/` not found or identity not configured |
| 2 | Node path not found |
| 3 | Node is not a leaf task |
| 4 | Task is not in `in_progress` state |
| 5 | Reason is empty |

### Examples

```bash
# Block a task with a reason
wolfcastle task block --node attunement-tree/water-impl/task-1 "Missing upstream API dependency"

# Block a task that the model can't fix
wolfcastle task block --node attunement-tree/earth-impl/task-5 "Flaky test infrastructure — needs human intervention"
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Node not found | `{"ok": false, "error": "node 'foo/bar' not found in tree", "code": "NODE_NOT_FOUND"}` | 2 |
| Not a leaf | `{"ok": false, "error": "node 'foo/bar' is not a leaf task", "code": "INVALID_NODE_TYPE"}` | 3 |
| Wrong state | `{"ok": false, "error": "task 'foo/bar' is 'complete', cannot block (only in_progress tasks can be blocked)", "code": "INVALID_STATE"}` | 4 |
| Empty reason | `{"ok": false, "error": "block reason must not be empty", "code": "EMPTY_REASON"}` | 5 |

---

## wolfcastle task unblock

### Synopsis

```
wolfcastle task unblock --node <path>
```

### Description

Clears the `blocked` state on a task, resetting it to `not_started` and resetting the failure counter to zero (ADR-019, ADR-028). The task must be re-claimed before work can resume. This ensures the model re-examines the task fresh rather than blindly resuming.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Tree address of the task to unblock (ADR-008) |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the root state index at `projects/{identity}/state.json` (ADR-024).
4. Validate the `--node` path exists and points to a leaf task.
5. Load the node's co-located `state.json` at `projects/{identity}/{node-path}/state.json`.
6. Validate the task's current state is `blocked`.
7. Update the task's state to `not_started` (ADR-028: unblock resets to Not Started, requiring re-claim).
8. Reset the task's `failure_count` to `0` (ADR-019).
9. Clear the block reason.
10. Record an unblock timestamp.
11. Write the updated node `state.json`.
12. Output the result as JSON.

### Output

```json
{
  "ok": true,
  "action": "task_unblocked",
  "node": "attunement-tree/water-impl/task-1",
  "state": "not_started",
  "previous_state": "blocked",
  "failure_count_reset": true
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Task unblocked successfully |
| 1 | `.wolfcastle/` not found or identity not configured |
| 2 | Node path not found |
| 3 | Node is not a leaf task |
| 4 | Task is not in `blocked` state |

### Examples

```bash
# Unblock after fixing the external dependency
wolfcastle task unblock --node attunement-tree/water-impl/task-1

# Unblock and immediately start the daemon to pick it up
wolfcastle task unblock --node attunement-tree/water-impl/task-1 && wolfcastle start
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Node not found | `{"ok": false, "error": "node 'foo/bar' not found in tree", "code": "NODE_NOT_FOUND"}` | 2 |
| Not a leaf | `{"ok": false, "error": "node 'foo/bar' is not a leaf task", "code": "INVALID_NODE_TYPE"}` | 3 |
| Wrong state | `{"ok": false, "error": "task 'foo/bar' is 'not_started', not 'blocked'", "code": "INVALID_STATE"}` | 4 |

---

## wolfcastle unblock

### Synopsis

```
wolfcastle unblock --node <path>
wolfcastle unblock --agent --node <path>
```

### Description

Provides assisted unblocking for blocked tasks. Two modes are available (ADR-028):

**Interactive mode** (`wolfcastle unblock --node <path>`): Starts a multi-turn interactive chat session with a configurable model, pre-loaded with the block context (block reason, failure history, breadcrumbs, audit state, relevant code areas, and previous fix attempts). The human works through the fix conversationally with the model. This is explicitly NOT autonomous -- anything blocked has already failed in the autonomous loop, so human judgment is the missing ingredient. When the fix is applied, the human runs `wolfcastle task unblock --node <path>` to flip the status.

**Agent mode** (`wolfcastle unblock --agent --node <path>`): Outputs rich structured diagnostic context to stdout for consumption by an already-running interactive agent (e.g., Claude Code). No model invocation occurs -- this is a context dump. The output includes block diagnostic, failure history, breadcrumbs, audit state, relevant file paths, suggested approaches based on failure patterns, and instructions to run `wolfcastle task unblock --node <path>` when done. The agent and human take it from there.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Tree address of the blocked task (ADR-008) |
| `--agent` | boolean | No | `false` | Output structured diagnostic context for an already-running agent instead of starting an interactive session |

### Behavior (Interactive Mode)

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the root state index at `projects/{identity}/state.json` (ADR-024).
4. Validate the `--node` path exists and points to a blocked task.
5. Load the node's co-located `state.json`.
6. Gather block context: block reason, failure count, decomposition depth, failure history, breadcrumbs, audit state, and relevant code areas referenced in breadcrumbs.
7. Resolve the model from `config.json` under `unblock.model` (default: `"heavy"`).
8. Resolve the prompt from `unblock.prompt_file` (default: `"unblock.md"`) via three-tier merge.
9. Assemble the prompt with the gathered block context and invoke the model as an interactive multi-turn chat session.
10. The human works through the fix conversationally. When done, the human exits the session and runs `wolfcastle task unblock --node <path>` to reset the task.

### Behavior (Agent Mode)

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the root state index at `projects/{identity}/state.json` (ADR-024).
4. Validate the `--node` path exists and points to a blocked task.
5. Load the node's co-located `state.json`.
6. Gather block context: block reason, failure count, decomposition depth, failure history, breadcrumbs, audit state, relevant file paths referenced in breadcrumbs, and suggested approaches based on failure patterns.
7. Assemble and output structured Markdown to stdout containing all gathered context and instructions to run `wolfcastle task unblock --node <path>` when the fix is applied.
8. No model is invoked.

### Output

Interactive mode produces an interactive chat session (no structured output).

Agent mode outputs structured Markdown to stdout:

```markdown
# Unblock Diagnostic: attunement-tree/water-impl/task-1

## Block Reason
Missing upstream API dependency

## Failure History
- Failure count: 12
- Decomposition depth: 0
- Last 3 attempts failed with: API endpoint /v2/attunements returns 404

## Breadcrumbs
- [2026-03-12T18:30:00Z] Attempted to call /v2/attunements endpoint
- [2026-03-12T18:35:00Z] Confirmed endpoint not yet deployed
...

## Audit State
Scope: Verify water attunement integration with upstream API
Status: pending

## Relevant Files
- api/client.go
- combat/water_attunement.go

## Suggested Approaches
- Verify upstream API deployment status
- Consider mocking the endpoint for local development
- Check if the API contract has changed

## When Fixed
Run: wolfcastle task unblock --node attunement-tree/water-impl/task-1
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Success (interactive session completed, or agent context output) |
| 1 | `.wolfcastle/` not found or identity not configured |
| 2 | Node path not found |
| 3 | Task is not in `blocked` state |

### Examples

```bash
# Start an interactive unblock session with a model
wolfcastle unblock --node attunement-tree/water-impl/task-1

# Output diagnostic context for an already-running agent (e.g., Claude Code)
wolfcastle unblock --agent --node attunement-tree/water-impl/task-1

# Agent mode piped to clipboard for pasting into an agent session
wolfcastle unblock --agent --node attunement-tree/water-impl/task-1 | pbcopy
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| No identity | `fatal: identity not configured. Run 'wolfcastle init' first.` | 1 |
| Node not found | `fatal: node 'foo/bar' not found in tree` | 2 |
| Not blocked | `fatal: task 'foo/bar' is 'in_progress', not 'blocked'` | 3 |

### Configuration

The model used for interactive mode is configured in `config.json`:

```json
{
  "unblock": {
    "model": "heavy",
    "prompt_file": "unblock.md"
  }
}
```

Agent mode does not invoke a model and ignores these settings.

---

## wolfcastle project create

### Synopsis

```
wolfcastle project create [--node <parent>] "<name>"
```

### Description

Creates a new project or sub-project node. When `--node` is omitted, creates a root-level project. When `--node` is provided, creates a child node under the specified parent. The new node is an orchestrator (can have children) rather than a leaf task. Used during discovery to structure work into a tree hierarchy.

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `--node <parent>` | string | No | (none -- root level) | Tree address of the parent node (ADR-008). When omitted, the project is created at the root level |
| `"<name>"` | string (positional) | Yes | -- | Name for the new project node (used as its slug in the tree path) |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the root state index at `projects/{identity}/state.json` (ADR-024).
4. If `--node` is provided, validate the parent path exists in the tree index and is an orchestrator node. If `--node` is omitted, create the project at the root level.
5. (When `--node` is provided) Validate the parent is an orchestrator node.
6. Generate a slug from the name (lowercase, hyphens for spaces, strip special characters).
7. Check that no sibling node already has the same slug. If collision, append a numeric suffix.
8. Create the new project's directory at `projects/{identity}/{parent-path}/{slug}/` and write its co-located `state.json` (ADR-024):
   - `name`: the provided name
   - `type`: `"orchestrator"`
   - `children`: `[]`
   - `audit`: `null`
9. Create the project description Markdown file at `projects/{identity}/{parent-path}/{slug}/{slug}.md` (ADR-024).
10. Append the new node to the parent's `children` list in the parent's `state.json`.
11. Update the root `state.json` index to register the new node.
12. Output the result as JSON.

### Output

```json
{
  "ok": true,
  "action": "project_created",
  "node": "attunement-tree/fire-implementation",
  "parent": "attunement-tree",
  "name": "Fire Implementation",
  "type": "orchestrator"
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Project created successfully |
| 1 | `.wolfcastle/` not found or identity not configured |
| 2 | Parent node path not found |
| 3 | Parent is a leaf task (cannot nest projects under leaf tasks) |
| 4 | Name is empty |

### Examples

```bash
# Create a root-level project (no --node flag)
wolfcastle project create "Attunement Tree"

# Create a sub-project under an existing parent
wolfcastle project create --node attunement-tree "Fire Implementation"
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Parent not found | `{"ok": false, "error": "node 'foo/bar' not found in tree", "code": "NODE_NOT_FOUND"}` | 2 |
| Parent is a leaf | `{"ok": false, "error": "cannot create project under leaf node 'foo/bar/task-1'", "code": "INVALID_NODE_TYPE"}` | 3 |
| Empty name | `{"ok": false, "error": "project name must not be empty", "code": "EMPTY_NAME"}` | 4 |

---

## wolfcastle adr create

### Synopsis

```
wolfcastle adr create "<title>" [--stdin | --file <path>]
```

### Description

Creates a new Architecture Decision Record in `.wolfcastle/docs/decisions/`. The filename follows the ISO 8601 timestamp format specified in ADR-011. The ADR follows the format from ADR-001 (Status, Date, Context, Decision, Consequences). Content can be provided via stdin or a file for lengthy decisions, or the command creates a template for the model to fill in.

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `"<title>"` | string (positional) | Yes | -- | Title of the ADR |
| `--stdin` | boolean | No | `false` | Read ADR body from stdin |
| `--file <path>` | string | No | -- | Read ADR body from the specified file |

`--stdin` and `--file` are mutually exclusive. If neither is provided, the command creates a template with placeholder sections.

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Generate the filename:
   - Get the current UTC timestamp with minute precision.
   - Format as `{YYYY}-{MM}-{DD}T{HH}-{mm}Z-{slug}.md` (ADR-011).
   - Slug is derived from the title (lowercase, hyphens for spaces, strip special characters).
3. Determine the body content:
   - If `--stdin`: read all of stdin as the body.
   - If `--file <path>`: read the specified file as the body.
   - If neither: generate a template:
     ```markdown
     # ADR: {title}

     ## Status
     Accepted

     ## Date
     {YYYY-MM-DD}

     ## Context
     <!-- Why is this decision needed? -->

     ## Decision
     <!-- What was decided and why? -->

     ## Consequences
     <!-- What follows from this decision? -->
     ```
4. Write the file to `.wolfcastle/docs/decisions/{filename}`.
5. Output the result as JSON (model-facing command).

### Output

```json
{
  "ok": true,
  "action": "adr_created",
  "path": ".wolfcastle/docs/decisions/2026-03-12T18-45Z-use-websockets-for-live-updates.md",
  "title": "Use WebSockets for Live Updates",
  "filename": "2026-03-12T18-45Z-use-websockets-for-live-updates.md"
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | ADR created successfully |
| 1 | `.wolfcastle/` not found |
| 2 | Title is empty |
| 3 | `--file` path does not exist or is not readable |
| 4 | Both `--stdin` and `--file` specified |

### Examples

```bash
# Create an ADR with a template (model fills it in later)
wolfcastle adr create "Use WebSockets for Live Updates"

# Create an ADR with body from a file
wolfcastle adr create "Switch to PostgreSQL" --file /tmp/adr-body.md

# Create an ADR with body piped from stdin
echo "## Status\nAccepted\n..." | wolfcastle adr create "Adopt Conventional Commits" --stdin
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Empty title | `{"ok": false, "error": "ADR title must not be empty", "code": "EMPTY_TITLE"}` | 2 |
| File not found | `{"ok": false, "error": "file '/tmp/body.md' not found", "code": "FILE_NOT_FOUND"}` | 3 |
| Conflicting flags | `{"ok": false, "error": "--stdin and --file are mutually exclusive", "code": "CONFLICTING_FLAGS"}` | 4 |

---

## wolfcastle audit breadcrumb

### Synopsis

```
wolfcastle audit breadcrumb --node <path> "<text>"
```

### Description

Appends a breadcrumb entry to a node's audit trail. Breadcrumbs are chronological notes recorded by the model during task execution. They serve as the raw material for archive entries (ADR-016) and provide an audit trail of what was done and why.

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Tree address of the node (ADR-008) |
| `"<text>"` | string (positional) | Yes | -- | Breadcrumb text describing what was done or observed |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the root state index at `projects/{identity}/state.json` (ADR-024).
4. Validate the `--node` path exists in the tree index.
5. Load the node's co-located `state.json` at `projects/{identity}/{node-path}/state.json`.
6. Append a breadcrumb entry to the node's `audit.breadcrumbs` array:
   ```json
   {
     "timestamp": "2026-03-12T18:45:03Z",
     "task": "<resolved from daemon execution context — full tree address of the active task>",
     "text": "the breadcrumb text"
   }
   ```
7. Write the updated node `state.json`.
8. Output the result as JSON.

### Output

```json
{
  "ok": true,
  "action": "breadcrumb_added",
  "node": "attunement-tree/fire-impl/task-3",
  "breadcrumb_count": 5,
  "text": "Implemented stamina cost deduction in fire_ability.go. Added unit test for edge case where stamina is exactly zero."
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Breadcrumb added successfully |
| 1 | `.wolfcastle/` not found or identity not configured |
| 2 | Node path not found |
| 3 | Text is empty |

### Examples

```bash
# Record work done on a task
wolfcastle audit breadcrumb --node attunement-tree/fire-impl/task-3 \
  "Implemented stamina cost deduction in fire_ability.go. Added unit test for edge case where stamina is exactly zero."

# Record an observation
wolfcastle audit breadcrumb --node attunement-tree/fire-impl/task-3 \
  "Discovered that the stamina system uses float64. Converting to int for consistency with the rest of the codebase."
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Node not found | `{"ok": false, "error": "node 'foo/bar' not found in tree", "code": "NODE_NOT_FOUND"}` | 2 |
| Empty text | `{"ok": false, "error": "breadcrumb text must not be empty", "code": "EMPTY_TEXT"}` | 3 |

---

## wolfcastle audit escalate

### Synopsis

```
wolfcastle audit escalate --node <path> "<gap>"
```

### Description

Escalates an audit gap to the parent node's audit trail. When the model audits a node and finds a gap (something missing, incorrect, or incomplete) that cannot be fixed at the current level, it escalates to the parent so the parent's audit will also check for that gap at the integration level (ADR-007).

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Tree address of the node where the gap was found (ADR-008) |
| `"<gap>"` | string (positional) | Yes | -- | Description of the audit gap being escalated |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the root state index at `projects/{identity}/state.json` (ADR-024).
4. Validate the `--node` path exists in the tree index.
5. Validate the node has a parent (cannot escalate from the root).
6. Resolve the parent node.
7. Load the parent node's co-located `state.json` at `projects/{identity}/{parent-path}/state.json`.
8. Append an escalation entry to the parent's `escalations` array:
   ```json
   {
     "timestamp": "2026-03-12T18:45:03Z",
     "source_node": "attunement-tree/fire-impl/task-3",
     "gap": "the gap description"
   }
   ```
9. Also record the escalation on the source node's `state.json` for traceability.
10. Write both updated `state.json` files.
11. Output the result as JSON.

### Output

```json
{
  "ok": true,
  "action": "gap_escalated",
  "source_node": "attunement-tree/fire-impl/task-3",
  "parent_node": "attunement-tree/fire-impl",
  "gap": "Integration between fire and stamina modules not verified — needs cross-module test"
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Gap escalated successfully |
| 1 | `.wolfcastle/` not found or identity not configured |
| 2 | Node path not found |
| 3 | Node is the root (cannot escalate further) |
| 4 | Gap description is empty |

### Examples

```bash
# Escalate a gap found during audit
wolfcastle audit escalate --node attunement-tree/fire-impl/task-3 \
  "Integration between fire and stamina modules not verified — needs cross-module test"

# Escalate a gap at the sub-project level to the project level
wolfcastle audit escalate --node attunement-tree/fire-impl \
  "No end-to-end test covering the full fire attunement flow"
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Node not found | `{"ok": false, "error": "node 'foo/bar' not found in tree", "code": "NODE_NOT_FOUND"}` | 2 |
| Root node | `{"ok": false, "error": "cannot escalate from root node — no parent to escalate to", "code": "NO_PARENT"}` | 3 |
| Empty gap | `{"ok": false, "error": "gap description must not be empty", "code": "EMPTY_GAP"}` | 4 |

---

## wolfcastle audit pending

### Synopsis

```
wolfcastle audit pending
```

### Description

Displays the current batch of audit findings that have not yet been approved or rejected. Shows finding IDs, titles, and description previews. If no pending batch exists, reports that (ADR-038).

### Behavior

1. Load `audit-review.json` from `.wolfcastle/`.
2. If no file exists, report "No pending audit review batch."
3. Filter findings with status `"pending"`.
4. Display each finding's ID, title, and first line of description.
5. With `--json`, return the full batch metadata and pending findings array.

### Output

Human output lists findings with IDs for use with `approve`/`reject`:

```
Pending audit findings (batch audit-20260314T120000Z, 2 scope(s)):

  [finding-1] Missing input validation on API endpoints
  [finding-2] Stale database migration files
```

---

## wolfcastle audit approve

### Synopsis

```
wolfcastle audit approve <finding-id>
wolfcastle audit approve --all
```

### Description

Approves a pending audit finding, creating a leaf project in the work tree. Use `--all` to approve every remaining pending finding. When all findings have been decided, the batch is archived to `audit-review-history.json` and the pending file is removed (ADR-038).

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `<finding-id>` | string (positional) | Yes (unless `--all`) | -- | ID of the finding to approve |
| `--all` | boolean | No | false | Approve all pending findings |

### Behavior

1. Load `audit-review.json`. Fail if not found.
2. Load root index.
3. For each targeted finding: create a leaf project with the finding title, write state and description files.
4. Mark findings as `"approved"` with timestamp and created node address.
5. Save updated batch and root index.
6. If all findings are decided, archive the batch to history with retention (100 entries, 90 days) and remove the pending file.

---

## wolfcastle audit reject

### Synopsis

```
wolfcastle audit reject <finding-id>
wolfcastle audit reject --all
```

### Description

Rejects a pending audit finding without creating any project. The decision is recorded for audit trail purposes. When all findings have been decided, the batch is archived to history and the pending file is removed (ADR-038).

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `<finding-id>` | string (positional) | Yes (unless `--all`) | -- | ID of the finding to reject |
| `--all` | boolean | No | false | Reject all pending findings |

### Behavior

1. Load `audit-review.json`. Fail if not found.
2. Mark targeted findings as `"rejected"` with timestamp.
3. Save updated batch.
4. If all findings are decided, archive the batch to history and remove the pending file.

---

## wolfcastle audit history

### Synopsis

```
wolfcastle audit history
```

### Description

Displays the history of completed audit review batches with their decisions. Most recent batches are shown first. Each entry shows the batch ID, completion timestamp, scopes, and a summary of approved/rejected findings (ADR-038).

### Behavior

1. Load `audit-review-history.json` from `.wolfcastle/`.
2. If no file exists, report "No audit review history."
3. Display entries in reverse chronological order with decision counts and individual finding outcomes.

---

## wolfcastle archive add

### Synopsis

```
wolfcastle archive add --node <path>
```

### Description

Generates an archive entry from a completed node's state. The archive entry is a Markdown file written to `.wolfcastle/archive/` with a timestamp filename (ADR-011). The entry is assembled deterministically from the node's breadcrumbs, audit results, escalations, and optional model-written summary (ADR-016). This command is typically called by the daemon after a node's audit task completes.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Tree address of the completed node to archive (ADR-008) |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the root state index at `projects/{identity}/state.json` (ADR-024).
4. Validate the `--node` path exists in the tree index.
5. Load the node's co-located `state.json` at `projects/{identity}/{node-path}/state.json`.
6. Validate the node's state is `complete` (all children complete and audit done).
7. Gather archive data from the node and its children's `state.json` files:
   - Summary (if summary stage was enabled and ran)
   - Breadcrumbs (chronological)
   - Audit results (scope, gaps found, gaps fixed, escalations)
   - Metadata (node path, completion timestamp, engineer identity, branch)
8. Generate the archive filename: `{timestamp}-{slug}.md` (ADR-011).
9. Assemble the Markdown content deterministically (no model call):
   ```markdown
   # {Node Name}

   ## Summary
   {model-written summary, if available}

   ## Breadcrumbs
   - [{timestamp}] {text}
   - [{timestamp}] {text}
   ...

   ## Audit
   ### Scope
   {audit scope description}

   ### Gaps Found and Resolved
   - {gap}: {resolution}

   ### Escalations
   - {gap} (escalated to {parent})

   ## Metadata
   - **Node**: {full tree path}
   - **Completed**: {timestamp}
   - **Identity**: {user-machine}
   - **Branch**: {branch name}
   ```
10. Write the file to `.wolfcastle/archive/{filename}`.
11. Output the result as JSON.

### Output

```json
{
  "ok": true,
  "action": "archive_created",
  "node": "attunement-tree/fire-impl",
  "archive_path": ".wolfcastle/archive/2026-03-12T18-45Z-fire-implementation-complete.md",
  "breadcrumb_count": 23,
  "has_summary": true
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Archive entry created successfully |
| 1 | `.wolfcastle/` not found or identity not configured |
| 2 | Node path not found |
| 3 | Node is not complete |

### Examples

```bash
# Archive a completed sub-project
wolfcastle archive add --node attunement-tree/fire-impl

# Archive the entire project
wolfcastle archive add --node attunement-tree
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Node not found | `{"ok": false, "error": "node 'foo/bar' not found in tree", "code": "NODE_NOT_FOUND"}` | 2 |
| Not complete | `{"ok": false, "error": "node 'foo/bar' is not complete (state: 'in_progress')", "code": "NODE_NOT_COMPLETE"}` | 3 |

---

## wolfcastle inbox add

### Synopsis

```
wolfcastle inbox add "<idea>"
```

### Description

Adds an item to the inbox. The inbox is a list of unstructured ideas, tasks, or notes that the pipeline's expansion stage (if configured) will later process into structured tasks. This is a convenience alternative to editing the inbox file directly.

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `"<idea>"` | string (positional) | Yes | -- | The idea or task description to add to the inbox |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the inbox file at `projects/{identity}/inbox.json`. If it does not exist, create it with an empty items array.
4. Append a new entry:
   ```json
   {
     "timestamp": "2026-03-12T18:45:03Z",
     "text": "the idea text",
     "status": "new"
   }
   ```
5. Write the updated `inbox.json`.
6. Output a confirmation.

### Output

Human-readable (user-facing command):

```
Added to inbox: "Add caching layer for database queries"
Inbox now has 7 items.
```

With `--json`:

```json
{
  "ok": true,
  "action": "inbox_item_added",
  "text": "Add caching layer for database queries",
  "inbox_count": 7
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Item added successfully |
| 1 | `.wolfcastle/` not found or identity not configured |
| 2 | Idea text is empty |

### Examples

```bash
# Add an idea to the inbox
wolfcastle inbox add "Add caching layer for database queries"

# Add a more detailed idea
wolfcastle inbox add "Refactor the auth middleware to support OAuth2 — currently only supports basic auth and API keys"
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| No identity | `fatal: identity not configured. Run 'wolfcastle init' first.` | 1 |
| Empty idea | `fatal: idea text must not be empty` | 2 |

---

## wolfcastle navigate

### Synopsis

```
wolfcastle navigate [--node <path>]
```

### Description

Performs a depth-first traversal of the work tree to find the next actionable task. Returns the path to the first `not_started` or `in_progress` task found, or indicates that no work is available. This command only finds tasks -- it does NOT claim them. The daemon calls `wolfcastle task claim` separately after navigation to transition the task to `in_progress`. Optionally scoped to a subtree. This command is used internally by the daemon at the start of each iteration and can also be called by the model or user to inspect what would run next.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--node <path>` | string | No | (none -- full tree) | Scope traversal to a specific subtree (ADR-008) |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the root state index at `projects/{identity}/state.json` (ADR-024).
4. If `--node` is specified, validate the path exists in the tree index and use it as the traversal root. Otherwise, use the tree root.
5. Perform depth-first traversal (loading each node's co-located `state.json` as needed):
   a. If the current node is a leaf task:
      - If state is `in_progress`: return it (self-healing case -- crashed mid-task).
      - If state is `not_started`: return it (next task to execute).
      - If state is `complete` or `blocked`: skip, continue traversal.
   b. If the current node is an orchestrator:
      - If all children are `complete` and audit has not run: return the audit task for this node.
      - Otherwise: recurse into children in order.
6. If traversal completes with no actionable task found: return a "no work available" result.

### Output

Task found:

```json
{
  "ok": true,
  "action": "navigated",
  "node": "attunement-tree/fire-impl/task-3",
  "state": "not_started",
  "description": "Wire stamina cost into fire ability",
  "depth": 2
}
```

Self-healing (in-progress task from previous crash):

```json
{
  "ok": true,
  "action": "navigated",
  "node": "attunement-tree/fire-impl/task-3",
  "state": "in_progress",
  "description": "Wire stamina cost into fire ability",
  "depth": 2,
  "resumed": true
}
```

No work available:

```json
{
  "ok": true,
  "action": "navigated",
  "node": null,
  "reason": "all_complete"
}
```

No work available (all remaining tasks blocked):

```json
{
  "ok": true,
  "action": "navigated",
  "node": null,
  "reason": "all_blocked",
  "blocked_count": 3
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Navigation completed (whether or not a task was found) |
| 1 | `.wolfcastle/` not found or identity not configured |
| 2 | Specified `--node` path not found |

### Examples

```bash
# Find the next task in the full tree
wolfcastle navigate

# Find the next task within a specific project
wolfcastle navigate --node attunement-tree/fire-impl

# Use navigation output in a script
NEXT=$(wolfcastle navigate)
NODE=$(echo "$NEXT" | jq -r '.node')
if [ "$NODE" != "null" ]; then
  wolfcastle task claim --node "$NODE"
fi
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Not initialized | `{"ok": false, "error": "not a wolfcastle project (no .wolfcastle/ found)", "code": "NOT_INITIALIZED"}` | 1 |
| No identity | `{"ok": false, "error": "identity not configured", "code": "NO_IDENTITY"}` | 1 |
| Node not found | `{"ok": false, "error": "node 'foo/bar' not found in tree", "code": "NODE_NOT_FOUND"}` | 2 |

---

## wolfcastle doctor

### Synopsis

```
wolfcastle doctor [--fix] [--json]
```

### Description

Validates the structural integrity of the Wolfcastle project and offers to fix issues. The doctor runs a comprehensive suite of checks against the distributed state files (ADR-024), identifying orphaned files, missing index entries, stale states, missing audit tasks, and other structural inconsistencies.

A subset of these checks also runs automatically on daemon startup (ADR-025). The `doctor` command runs the full suite and provides interactive repair.

This is a user-facing command. The underlying validation engine is reusable infrastructure shared with daemon startup checks and potentially CI pipelines.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--fix` | boolean | No | `false` | Skip the confirmation prompt and apply all fixes immediately |
| `--json` | boolean | No | `false` | Output as structured JSON instead of human-readable text |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity from `config.local.json`.
3. **Structural validation** -- Go code walks the engineer's tree at `projects/{identity}/`, checking invariants:
   - **Root index consistency**: Every node directory under `projects/{identity}/` has a corresponding entry in the root `state.json` index, and vice versa.
   - **Per-node state integrity**: Every node directory contains a valid `state.json` file with required fields.
   - **Parent-child consistency**: Every parent's `children` list matches the actual child directories and their `state.json` files.
   - **Orphaned files**: Directories exist under `projects/{identity}/` that are not registered in any parent's `children` list or the root index.
   - **Stale In Progress**: Tasks with `in_progress` state but no running daemon (detectable via PID file check).
   - **Missing audit tasks**: Orchestrator nodes where all children are `Complete` but no audit task exists.
   - **Description file presence**: Orchestrator nodes missing their companion `{slug}.md` description file.
   - **Task working document references**: Task `state.json` references a Markdown working document that does not exist (or vice versa -- orphaned `.md` files with no corresponding task).
4. **Report findings** -- list every issue with:
   - Severity: `error` (structural corruption), `warning` (inconsistency), `info` (cosmetic)
   - Location: node path, file path, and field name where applicable
   - Description of the issue
   - Proposed fix
5. If no issues found, print a clean bill of health and exit 0.
6. **User confirmation** (interactive, unless `--fix` is passed):
   - Present the list of issues and proposed fixes.
   - Prompt: `Fix all? [y]es / [s]elect individually / [a]bort`
   - If `--fix` is passed, skip the prompt and apply all fixes.
7. **Apply fixes** in two categories:
   - **Deterministic fixes** (Go code applies directly, no model needed):
     - Add missing entries to root `state.json` index for orphaned directories
     - Remove stale index entries for directories that no longer exist
     - Create missing audit task entries on orchestrator nodes with all children complete
     - Reset stale `in_progress` tasks to `not_started` when no daemon is running
     - Create missing `state.json` files with sensible defaults for orphaned directories
   - **Model-assisted fixes** (for ambiguous cases where intent is unclear):
     - Conflicting state between parent and children (e.g., parent says 3 children, 4 directories exist)
     - Task descriptions that reference nonexistent nodes
     - Multiple plausible resolutions where the correct one depends on project context
     - The model configured in `config.json` under `doctor.model` (default: `"mid"`) reasons about the resolution. Go code validates the model's proposed fix before applying it.
8. Report what was fixed and what was skipped.

### Output

Human-readable (default, issues found):

```
Wolfcastle Doctor
=================
Checking projects/wild-macbook/...

  error   attunement-tree/fire-impl/state.json
          Missing from root index. Fix: add to index.

  warning attunement-tree/water-impl/task-1/state.json
          State is 'in_progress' but no daemon is running. Fix: reset to not_started.

  info    attunement-tree/earth-impl/
          Missing description file earth-impl.md. Fix: create template.

Found 3 issues (1 error, 1 warning, 1 info).
Fix all? [y]es / [s]elect individually / [a]bort: y

Fixed 3 issues.
```

Human-readable (no issues):

```
Wolfcastle Doctor
=================
Checking projects/wild-macbook/...

No issues found. Everything looks good.
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "doctor_report",
  "identity": "wild-macbook",
  "issues": [
    {
      "severity": "error",
      "location": {
        "node": "attunement-tree/fire-impl",
        "file": "projects/wild-macbook/attunement-tree/fire-impl/state.json",
        "field": null
      },
      "description": "Missing from root index",
      "fix": "add to index",
      "fix_type": "deterministic"
    },
    {
      "severity": "warning",
      "location": {
        "node": "attunement-tree/water-impl/task-1",
        "file": "projects/wild-macbook/attunement-tree/water-impl/task-1/state.json",
        "field": "state"
      },
      "description": "State is 'in_progress' but no daemon is running",
      "fix": "reset to not_started",
      "fix_type": "deterministic"
    }
  ],
  "summary": {
    "total": 3,
    "errors": 1,
    "warnings": 1,
    "info": 1
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | No issues found, or all issues fixed successfully |
| 1 | `.wolfcastle/` not found or identity not configured |
| 2 | Issues found but user chose to abort (no fixes applied) |
| 3 | Some fixes failed to apply |

### Examples

```bash
# Run the doctor interactively
wolfcastle doctor

# Run the doctor and fix everything without prompting
wolfcastle doctor --fix

# Run the doctor and output JSON (useful for CI)
wolfcastle doctor --json

# Run after a crash to clean up stale state
wolfcastle doctor --fix
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| No identity | `fatal: identity not configured. Run 'wolfcastle init' first.` | 1 |
| User aborted | `Aborted. No fixes applied.` | 2 |
| Fix failed | `Failed to fix 2 of 5 issues. Run 'wolfcastle doctor' again for details.` | 3 |

### Configuration

The model used for ambiguous fixes is configured in `config.json`:

```json
{
  "doctor": {
    "model": "mid",
    "prompt_file": "doctor.md"
  }
}
```

The `prompt_file` is relative to `.wolfcastle/base/` and contains the system prompt for model-assisted fixes. The model receives the issue's location context (node path, file path, field, surrounding state) and proposes a resolution that Go code validates before applying.

### Daemon Startup Subset

When the daemon starts (`wolfcastle start`), a subset of the doctor's checks runs automatically (step 8 in `wolfcastle start` behavior). The startup subset includes:

- Root index consistency
- Per-node state integrity (required fields present)
- Stale `In Progress` detection

If the startup subset finds issues, the daemon prints a warning and continues. It does not attempt fixes automatically. The warning suggests running `wolfcastle doctor` for full diagnostics and repair.

---

## wolfcastle install

### Synopsis

```
wolfcastle install <target>
```

### Description

An extensible installation command for integrations that place files outside the `.wolfcastle/` directory. Wolfcastle never creates files outside `.wolfcastle/` by default -- this command is the explicit opt-in mechanism (ADR-026).

Currently supports one target: `skill`.

### Subcommands

#### wolfcastle install skill

```
wolfcastle install skill
```

Installs a Claude Code skill that enables CC users to interact with Wolfcastle natively from their conversation. The skill files are sourced from `.wolfcastle/base/skills/` (generated by `wolfcastle init` or `wolfcastle update`).

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `<target>` | string (positional) | Yes | -- | The integration target to install. Currently only `skill` is supported |

### Behavior (`wolfcastle install skill`)

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Verify that `.wolfcastle/base/skills/` exists and contains skill definition files. If not, fail (suggest running `wolfcastle update` to regenerate `base/`).
3. **Detect symlink support** on the current OS:
   - Attempt to create a test symlink in a temporary directory.
   - If successful: symlinks are supported.
   - If failed (e.g., Windows without developer mode): fall back to copy mode.
4. **Create the target directory**: ensure `{project_root}/.claude/` exists. Create it if it does not.
5. **Install the skill**:
   - **Symlink mode** (preferred): Create `.claude/wolfcastle/` as a symlink pointing to `.wolfcastle/base/skills/`. If `.claude/wolfcastle/` already exists:
     - If it is already a symlink to the correct target: print "already installed" and exit 0.
     - If it is a symlink to a different target or a regular directory: remove it and create the correct symlink.
   - **Copy mode** (fallback): Copy the contents of `.wolfcastle/base/skills/` to `.claude/wolfcastle/`. If `.claude/wolfcastle/` already exists, overwrite its contents.
6. Output the result.

### Output

Symlink mode:

```
Installed Claude Code skill (symlink)
  .claude/wolfcastle/ -> .wolfcastle/base/skills/
Skill will auto-update when you run 'wolfcastle update'.
```

Copy mode:

```
Installed Claude Code skill (copy)
  .claude/wolfcastle/ <- .wolfcastle/base/skills/
Note: run 'wolfcastle install skill' again after 'wolfcastle update' to refresh.
```

Already installed:

```
Claude Code skill is already installed and up to date.
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Skill installed successfully, or already installed |
| 1 | `.wolfcastle/` not found |
| 2 | Unknown target (not `skill`) |
| 3 | Source skill files not found in `.wolfcastle/base/skills/` |
| 4 | Cannot write to project root (permission denied) |

### Examples

```bash
# Install the Claude Code skill
wolfcastle install skill

# Re-install after wolfcastle update (only needed if copy mode was used)
wolfcastle update && wolfcastle install skill

# Verify the skill is installed
ls -la .claude/wolfcastle/
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| Unknown target | `fatal: unknown install target 'foo'. Available targets: skill` | 2 |
| No skill source | `fatal: skill files not found in .wolfcastle/base/skills/. Run 'wolfcastle update' to regenerate base/.` | 3 |
| Permission denied | `fatal: cannot write to .claude/: permission denied` | 4 |

### Extensibility

The `install` subcommand is designed to accept additional targets in the future (e.g., git hooks, editor plugins). Each target follows the same pattern: source files live in `.wolfcastle/base/`, and `install` places them outside `.wolfcastle/` with the user's explicit consent.

---

## Appendix: Exit Code Summary

All commands share a common set of exit codes for infrastructure errors. Command-specific codes start at 2.

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Infrastructure error (no `.wolfcastle/`, no identity, I/O failure, permission denied) |
| 2+ | Command-specific errors (documented per command above) |

## Appendix: Task State Machine

Tasks follow a strict state machine. Only the transitions listed below are valid; all others are rejected by the commands. State values in JSON are snake_case: `not_started`, `in_progress`, `complete`, `blocked`.

```
                    +-------------+
               +--->| not_started |
               |    +------+------+
               |           |
               |      task claim
               |           |
               |           v
               |    +-------------+
               |    | in_progress |
               |    +------+------+
               |           |
               |     +-----+-----+
               |     |           |
               |  complete     block
               |     |           |
               |     v           v
               | +----------+ +----------+
               | | complete | | blocked  |
               | +----------+ +----+-----+
               |                   |
               |              task unblock
               +-------------------+
                  (reset to
                  not_started)
```

Valid transitions:
- `not_started` -> `in_progress` (via `task claim`)
- `in_progress` -> `complete` (via `task complete`)
- `in_progress` -> `blocked` (via `task block`)
- `blocked` -> `not_started` (via `task unblock`)

## Appendix: JSON Envelope

All model-facing commands return a consistent JSON envelope:

```json
{
  "ok": true|false,
  "action": "verb_describing_what_happened",
  ...command-specific fields...
}
```

On error:

```json
{
  "ok": false,
  "error": "human-readable error description",
  "code": "MACHINE_READABLE_ERROR_CODE"
}
```

The `ok` field is always present. The `action` field is present on success. The `error` and `code` fields are present on failure.
