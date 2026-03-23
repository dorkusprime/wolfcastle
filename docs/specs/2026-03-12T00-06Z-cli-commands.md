# CLI Command Specification

This is the primary implementation reference for every Wolfcastle CLI command. The CLI has 60 leaf commands organized into 7 groups (Lifecycle, Work Management, Auditing, Documentation, Diagnostics, Integration, Additional Commands). All top-level commands are registered directly on the root; subcommand groups (`audit`, `task`, `project`, `orchestrator`, `inbox`, `spec`, `adr`, `archive`, `config`, `install`, `knowledge`) hold their children. There are no daemon-namespaced subcommands: `start`, `stop`, `status`, and `log` are top-level commands in the Lifecycle group.

## Conventions

These conventions apply to every command unless explicitly stated otherwise.

### Tree Addressing

All `--node` flags accept a slash-delimited path from the tree root to the target node (ADR-008). Example: `attunement-tree/fire-impl/task-3`. The path is resolved relative to the engineer's project directory (`projects/{identity}/`), where identity is `{user}-{machine}` from `local/config.json` (ADR-009).

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

Commands that need the engineer's identity resolve it from `local/config.json` as `{user}-{machine}`. If `local/config.json` is missing or identity fields are absent, the command exits with code 1 and the message: `fatal: identity not configured. Run 'wolfcastle init' first.`

---

## wolfcastle init

### Synopsis

```
wolfcastle init [--force]
```

### Description

Scaffolds the `.wolfcastle/` directory structure in the current working directory and auto-populates engineer identity in `local/config.json`. This must be run before any other wolfcastle command. If `.wolfcastle/` already exists, the command is a no-op unless `--force` is passed.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--force` | boolean | No | `false` | Re-scaffold `.wolfcastle/`, regenerating `base/` and refreshing `local/config.json` identity without overwriting existing config files. Migrates old-style root `config.json` and `config.local.json` if present |

### Behavior

1. Check whether `.wolfcastle/` already exists in the current directory.
   - If it exists and `--force` is not set, print a message and exit 0.
   - If it exists and `--force` is set, proceed (skip directory creation, regenerate `base/`, refresh identity).
2. Create the `.wolfcastle/` directory structure (ADR-009, ADR-063):
   ```
   .wolfcastle/
     .gitignore
     base/
       config.json
     custom/
       config.json
     local/
       config.json
     projects/
     archive/
     docs/
       decisions/
       specs/
     logs/
   ```
3. Write `.wolfcastle/.gitignore` with the content specified in ADR-009 (commit `custom/`, `projects/`, `archive/`, `docs/`; gitignore everything else).
4. Write `base/config.json` with compiled defaults for models, pipeline, failure thresholds, and log retention (ADRs 013, 006, 019, 012). Write an empty `custom/config.json` (`{}`).
5. Auto-detect engineer identity:
   - `user`: result of `whoami`
   - `machine`: result of `hostname`, with `.local` suffix stripped if present
6. Write `local/config.json` with the detected identity. If the file already exists (force mode), update identity fields only; preserve any other keys the user has added.
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
| 1 | `local/config.json` exists but is malformed JSON (force mode) |

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
| Malformed local/config.json (force mode) | `fatal: local/config.json exists but is not valid JSON` | 1 |

---

## wolfcastle start

### Synopsis

```
wolfcastle start [--node <path>] [--worktree <branch>] [-d] [-v]
```

### Description

Starts the Wolfcastle daemon, which begins the execution loop: navigate to the next active task, invoke the configured pipeline, commit results, and repeat. In foreground mode (default), the process runs in the current terminal. In background mode (`-d`), the process forks, writes a PID file, and returns control to the terminal.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--node <path>` | string | No | (none -- full tree) | Scope execution to a specific subtree. The path is a tree address (ADR-008) |
| `--worktree <branch>` | string | No | (none -- current branch) | Run in an isolated git worktree on the specified branch. Creates the branch from HEAD if it does not exist (ADR-015) |
| `-d`, `--daemon` | boolean | No | `false` | Run as a background daemon. Forks, writes PID file, returns immediately |
| `-v`, `--verbose` | boolean | No | `false` | Set console log level to debug |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity from `local/config.json`. Fail if not configured.
3. Load and merge configuration: `base/config.json` → `custom/config.json` → `local/config.json` (deep merge per ADR-018, ADR-063).
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

## wolfcastle execute

### Synopsis

```
wolfcastle execute [--node <path>]
```

### Description

Runs the daemon execution loop in the foreground, streaming interleaved output to stdout. A background goroutine tails the NDJSON log files as they are written and renders them through the same interleaved renderer used by `wolfcastle log --interleaved --follow`.

This is the non-daemon counterpart to `wolfcastle start`: same execution loop, but with formatted output directly on the terminal instead of requiring a separate `wolfcastle log` session. Where `wolfcastle start` can fork into the background, `execute` always runs in the foreground and always streams rendered output.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--node <path>` | string | No | (none, full tree) | Scope execution to a specific subtree. The path is a tree address (ADR-008) |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity from `local/config.json`. Fail if not configured.
3. Load and merge configuration: `base/config.json` → `custom/config.json` → `local/config.json`.
4. **Daemon check**: If a daemon process is already alive, exit with an error. `execute` and `start` are mutually exclusive; only one execution loop may run at a time.
5. **Node scoping** (if `--node` is specified):
   a. Validate that the node path exists in the tree.
   b. Record the scope root. Navigation will only traverse within this subtree.
6. **Renderer startup**: Launch a background goroutine that tails the NDJSON log directory (polling at 200ms intervals) and pipes records through the interleaved renderer to stdout. This produces the same output as `wolfcastle log -i -f`.
7. **Execution loop**: Run the same loop as `wolfcastle start` (navigate → invoke pipeline → commit → repeat) until all work is complete or the process receives a shutdown signal (SIGINT, SIGTERM).
8. **Shutdown**: When the execution loop returns (completion, signal, or error), cancel the renderer context and wait for it to drain remaining records before exiting.

### Output

```
wolfcastle v0.x.x
```

Followed by interleaved, color-coded log output from all active nodes as execution proceeds. Output format matches `wolfcastle log --interleaved --follow`.

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | All work complete, or graceful stop via signal |
| 1 | `.wolfcastle/` not found, identity not configured, or daemon construction failed |
| 1 | Another wolfcastle instance is already running |

### Examples

```bash
# Run the full tree with live output
wolfcastle execute

# Run scoped to a subtree
wolfcastle execute --node auth-module
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| No identity | `fatal: identity not configured. Run 'wolfcastle init' first.` | 1 |
| Already running | `daemon is already running. Use 'wolfcastle stop' first, or use 'wolfcastle log -i -f' to watch it` | 1 |
| Invalid node | `fatal: node 'foo/bar' not found in tree` | 1 |

---

## wolfcastle intake

### Synopsis

```
wolfcastle intake [--node <path>]
```

### Description

Processes pending inbox items in the foreground, streaming interleaved output to stdout. The intake loop watches `inbox.json` for new items and runs the intake stage for each one, just as the daemon would in the background.

Like `wolfcastle execute`, a background goroutine tails the NDJSON log files and renders them through the interleaved renderer. The difference is scope: `execute` runs the full execution loop (navigate, invoke pipeline, commit), while `intake` only processes inbox items.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--node <path>` | string | No | (none, full tree) | Scope intake to a specific subtree. The path is a tree address (ADR-008) |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity from `local/config.json`. Fail if not configured.
3. Load and merge configuration: `base/config.json` → `custom/config.json` → `local/config.json`.
4. **Daemon check**: If a daemon process is already alive, exit with an error. Only one instance may run at a time.
5. **Node scoping** (if `--node` is specified):
   a. Validate that the node path exists in the tree.
   b. Record the scope root.
6. **Renderer startup**: Launch a background goroutine that tails the NDJSON log directory (polling at 200ms intervals) and pipes records through the interleaved renderer to stdout.
7. **Inbox loop**: Watch `inbox.json` for pending items. For each item, run the intake stage. The loop blocks until the context is cancelled by a shutdown signal.
8. **Shutdown**: On signal (SIGINT, SIGTERM), cancel the renderer context and wait for it to drain before exiting.

### Output

```
wolfcastle intake v0.x.x
```

Followed by interleaved, color-coded log output as inbox items are processed. Output format matches `wolfcastle log --interleaved --follow`.

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Graceful stop via signal, or no inbox items remaining |
| 1 | `.wolfcastle/` not found, identity not configured, or daemon construction failed |
| 1 | Another wolfcastle instance is already running |

### Examples

```bash
# Process all pending inbox items with live output
wolfcastle intake

# Process inbox items scoped to a subtree
wolfcastle intake --node auth-module
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| No identity | `fatal: identity not configured. Run 'wolfcastle init' first.` | 1 |
| Already running | `daemon is already running. Use 'wolfcastle stop' first, or use 'wolfcastle log -i -f' to watch it` | 1 |
| Invalid node | `fatal: node 'foo/bar' not found in tree` | 1 |

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
wolfcastle status [--node <path>] [--all] [--expand] [--detail] [--archived] [--watch [-w]] [--interval <seconds>] [--json]
```

### Description

Displays the current state of the work tree, including active task, progress summary, blocked tasks, and daemon status. Works regardless of whether the daemon is running.

When `--node` is provided, shows status for only the specified subtree, consistent with the `--node` flag on `start` and `navigate`.

By default, shows only the current engineer's tree. With `--all`, aggregates state across all engineer directories at runtime (ADR-024). The `--all` mode is read-only -- it scans other engineers' `projects/` directories for their root `state.json` and per-node `state.json` files but never writes to them.

With `--watch` (or `-w`), the display refreshes on an interval until interrupted with Ctrl+C. The refresh interval defaults to 5 seconds and can be overridden with `--interval`.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--node <path>` | string | No | (none -- full tree) | Show status for only the specified subtree (ADR-008). Consistent with `--node` on `start` and `navigate` |
| `--all` | boolean | No | `false` | Aggregate status across all engineer directories, not just the current engineer's |
| `-w`, `--watch` | boolean | No | `false` | Refresh status on an interval until interrupted |
| `--interval <seconds>` | float64 | No | `5` | Refresh interval in seconds (only meaningful with `--watch`) |
| `--expand` | boolean | No | `false` | Show completed nodes and tasks expanded. By default, completed nodes are collapsed to a single line showing only a descendant/subtask count. Completed parent tasks whose children are all complete are similarly collapsed. When `--expand` is set, all nodes and tasks are shown in full regardless of completion state |
| `--detail` | boolean | No | `false` | Show extended detail for each node and task: task bodies, failure types, deliverable summaries, and the most recent breadcrumb for in-progress nodes. Combine with `--expand` for the fullest view |
| `--archived` | boolean | No | `false` | Show only archived nodes instead of the active tree. Renders each archived node's address, name, type, completion timestamp, and archive timestamp |
| `--json` | boolean | No | `false` | Output as structured JSON instead of human-readable text |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity from `local/config.json`.
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
7. **Display collapsing** (unless `--expand` is set): completed nodes are collapsed to a single summary line showing only the count of descendants or subtasks rather than listing each one. Completed parent tasks whose children are all complete are similarly collapsed, displaying only the child count. When `--expand` is set, all nodes and tasks are rendered in full regardless of completion state.
8. Output the status.

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

# Show completed nodes expanded instead of collapsed
wolfcastle status --expand

# Combine --expand with --node for a detailed subtree view
wolfcastle status --node attunement-tree/fire-impl --expand
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| No identity | `fatal: identity not configured. Run 'wolfcastle init' first.` | 1 |
| No root state file | `fatal: no tree state found at projects/{identity}/state.json. Start wolfcastle to initialize.` | 1 |

---

## wolfcastle log

Aliases: `wolfcastle follow`

### Synopsis

```
wolfcastle log [--follow [-f]] [--thoughts [-t]] [--interleaved [-i]] [--json] [--session [-s] <n>]
```

### Description

Displays daemon activity reconstructed from NDJSON log files. The default output is a groomed, human-readable summary of what the daemon did (or is doing). Flags control verbosity from summary-only to full agent output.

When the daemon is running, output streams in real time (implicit `--follow`). When the daemon is stopped, the last session's output is displayed and the command exits.

All output is reconstructed from NDJSON log files in `.wolfcastle/system/logs/`. The command never reads the daemon's stdout directly, so it works whether the daemon is running or stopped, and historical sessions are replayable.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `-f`, `--follow` | boolean | No | `false` (implicit `true` when daemon is running) | Follow live output. No-op when daemon is stopped. |
| `-t`, `--thoughts` | boolean | No | `false` | Raw agent output only. No stage headers or durations. |
| `-i`, `--interleaved` | boolean | No | `false` | Stage headers and agent output together with timestamps. |
| `--json` | boolean | No | `false` | Raw NDJSON output, no formatting. Pipe to `jq`. |
| `-s`, `--session` | integer | No | `0` | Session index. 0 = latest (current or most recent), 1 = previous, etc. |

`--thoughts`, `--interleaved`, and `--json` are mutually exclusive. If more than one is passed, the last one wins.

### Output Modes

Three modes control what the output looks like. The default is summary.

**Summary (default):** One line per completed stage showing stage type, node address, and duration.

```
[intake]  donut-stand-website                              (12s)
[plan]    donut-stand-website                              (45s)
[execute] donut-stand-website/site-specification/task-0001 (1m22s)
[execute] donut-stand-website/site-specification/task-0002 (58s)
[audit]   donut-stand-website/site-specification           (34s)
  report: .wolfcastle/system/projects/.../audit-2026-03-21T18-08.md
[execute] donut-stand-website/project-foundation/task-0001 (2m5s)
```

In follow mode, each stage prints a start line and a completion line using glyphs: `▶` for start, `✓` for completion, `✗` for failure or block.

```
▶ [execute] donut-stand-website/site-specification/task-0001
✓ [execute] donut-stand-website/site-specification/task-0001 (1m22s)
▶ [execute] donut-stand-website/site-specification/task-0002
✗ [execute] donut-stand-website/site-specification/task-0002 (3m41s)
```

When replaying a completed session (no `--follow`), only the completion lines are shown with `✓`/`✗` glyphs and durations.

**Thoughts (`--thoughts`):** Raw agent output only. No stage headers, no durations, no timestamps, no glyphs. Just what the model said, as it said it.

```
I'll start by creating the site specification document...
Reading the project requirements from the inbox item...
The inbox item asks for a donut stand website, so I'll need...
```

In follow mode, this tails the active log file and extracts agent assistant records in real time. For historical sessions, all agent output from every iteration in the session is shown.

**Interleaved (`--interleaved`):** Stage headers and agent thoughts together, chronologically, with wall-clock timestamps and glyphs.

```
18:01:34 ▶ [execute] donut-stand-website/site-specification/task-0001
18:01:35     I'll start by creating the site specification document...
18:01:36     Reading the project requirements from the inbox item...
18:02:56 ✓ [execute] donut-stand-website/site-specification/task-0001 (1m22s)
18:02:57 ▶ [execute] donut-stand-website/site-specification/task-0002
18:02:58     Now I need to write the HTML structure...
```

Agent thoughts are indented to visually separate them from stage headers. Timestamps are `HH:MM:SS` in local time.

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Identify the target session from `--session`. Session boundaries are detected by finding the iteration-1 log file or by timestamp discontinuity.
3. Read NDJSON log files for the target session from `.wolfcastle/system/logs/`.
4. Render output according to the selected mode (summary, thoughts, interleaved, or raw JSON).
5. **Without `--follow`**: display the session's output and exit 0.
6. **With `--follow`**: stream new records as they are written, rendering each according to the selected mode.
7. Continue until the user presses Ctrl+C or the daemon exits.

### Formatting Rules

Stage labels: `[intake]`, `[plan]`, `[execute]`, `[audit]`, `[remediate]`, `[spec-review]`. Duration uses compact human shorthand with no spaces: `34s`, `1m22s`, `12m5s`, `1h3m`. Terminal markers (`WOLFCASTLE_COMPLETE`, etc.) and log levels (`[INFO]`) are never shown in output. Audit report paths appear indented below the audit completion line when a report was generated.

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Success, or follow mode interrupted by Ctrl+C |
| 1 | No log files found, or not initialized |

### Examples

```bash
# Summary of the current/last session (streams if daemon is running)
wolfcastle log

# Raw agent thoughts, streamed live
wolfcastle log --thoughts

# Interleaved stage headers and thoughts with timestamps
wolfcastle log -i -f

# View the previous session
wolfcastle log --session 1

# Raw NDJSON piped to jq
wolfcastle log --json | jq '.type'
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| No logs found | `No log files found. Start wolfcastle first.` | 1 |

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

## wolfcastle version

### Synopsis

```
wolfcastle version [--json]
```

### Description

Prints the Wolfcastle binary's version, git commit hash, and build date. Does not require a `.wolfcastle/` directory or identity. This command always succeeds.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--json` | boolean | No | `false` | Output as structured JSON with version, commit, and date fields |

### Behavior

1. Read the version, commit, and date values injected at build time via ldflags.
2. Output them.

No filesystem access, no identity resolution, no `.wolfcastle/` directory required.

### Output

Human-readable:

```
wolfcastle v0.4.0 (a1b2c3d, 2026-03-14T10:00:00Z)
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "version",
  "data": {
    "version": "v0.4.0",
    "commit": "a1b2c3d",
    "date": "2026-03-14T10:00:00Z"
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Always |

### Examples

```bash
# Print version
wolfcastle version

# Get version as JSON
wolfcastle version --json

# Extract just the version string
wolfcastle version --json | jq -r '.data.version'
```

### Error Cases

None. This command always exits 0.

---

## wolfcastle task add

### Synopsis

```
wolfcastle task add --node <path> "<title>" [--body "<text>"] [--stdin] [--deliverable "<path>"] [--type <type>] [--class <class>] [--constraint "<text>"] [--acceptance "<text>"] [--reference "<path>"] [--integration "<text>"] [--parent <task-id>]
```

### Description

Adds a new task to a leaf node's task list, inserting it before the audit task (which is always last per ADR-007). The target node must be a leaf node. The new task is created in `not_started` state. This command is called by both models (during discovery/decomposition) and users.

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `"<title>"` | string (positional) | Yes | -- | Human-readable title of the task |
| `--node <path>` | string | Yes | -- | Tree address of the leaf node to add the task to (ADR-008) |
| `--body "<text>"` | string | No | `""` | Detailed task description/body text |
| `--stdin` | bool | No | `false` | Read task body from stdin (overrides `--body`) |
| `--deliverable "<path>"` | string slice | No | `[]` | Expected output file path (repeatable) |
| `--type <type>` | string | No | `""` | Task type: `discovery`, `spec`, `adr`, `implementation`, `integration`, `cleanup` |
| `--class <class>` | string | No | `""` | Task class override (e.g., `lang-go`) |
| `--constraint "<text>"` | string slice | No | `[]` | What not to do (repeatable) |
| `--acceptance "<text>"` | string slice | No | `[]` | Acceptance criterion (repeatable) |
| `--reference "<path>"` | string slice | No | `[]` | Reference material path (repeatable) |
| `--integration "<text>"` | string | No | `""` | How this task connects to other work |
| `--parent <task-id>` | string | No | `""` | Parent task ID for hierarchical decomposition (e.g., `task-0001`) |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Validate the title positional argument is non-empty (whitespace-only is rejected).
4. Validate `--node` is provided and resolves to a valid leaf node address.
5. Resolve the task body:
   - If `--stdin` is set, read all of stdin and use it as the body. This overrides any value passed via `--body`.
   - Otherwise, use the `--body` value (empty string if omitted).
6. If `--type` is provided, validate it against the allowed set: `discovery`, `spec`, `adr`, `implementation`, `integration`, `cleanup`. Fail with an error on invalid values.
7. Load the leaf's `state.json` via `MutateNode`.
8. Generate the next task ID:
   - If `--parent` is provided, create a hierarchical child ID under the parent (e.g., `task-0001.0001`). The parent task must exist. The parent auto-completes when all children finish.
   - Otherwise, generate a top-level ID (`task-N` where N is one greater than the current highest).
9. Insert a new task entry into the leaf's `tasks` array with:
   - `id`: the generated task ID
   - `title`: the provided title
   - `state`: `"not_started"`
   - `body`: the resolved body text (stored in `Body` field)
   - `deliverables`: the `--deliverable` values (stored in `Deliverables`)
   - `task_type`: the `--type` value (stored in `TaskType`)
   - `class`: the `--class` value (stored in `Class`)
   - `constraints`: the `--constraint` values (stored in `Constraints`)
   - `acceptance_criteria`: the `--acceptance` values (stored in `AcceptanceCriteria`)
   - `references`: the `--reference` values (stored in `References`)
   - `integration`: the `--integration` value (stored in `Integration`)
   Only non-empty/non-nil values are written; omitted flags leave their fields at zero value.
10. Write the updated leaf `state.json`. Adding a `not_started` task does not change the node's state, so no propagation is needed.
11. Write a `{task-id}.md` file in the node directory containing the title as a heading and the body (if any) as content.
12. Output the result.

### Output

JSON mode (`--json`):

```json
{
  "ok": true,
  "action": "task_add",
  "address": "attunement-tree/fire-impl/task-4",
  "task_id": "task-4",
  "description": "Wire stamina cost into fire ability",
  "state": "not_started",
  "deliverables": ["internal/fire/stamina.go"]
}
```

The `deliverables` key is present only when `--deliverable` was provided.

Human-readable mode prints: `Added task attunement-tree/fire-impl/task-4: Wire stamina cost into fire ability`

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Task added successfully |
| 1 | `.wolfcastle/` not found or identity not configured |
| 2 | Node path not found |
| 3 | Target node is not a leaf (tasks can only be added to leaves) |
| 4 | Title is empty |
| 5 | Invalid `--type` value |

### Examples

```bash
# Add a simple task
wolfcastle task add --node attunement-tree/fire-impl "Wire stamina cost into fire ability"

# Add a task with a body describing the work
wolfcastle task add --node auth/login "Add rate limiting" \
  --body "Implement token-bucket rate limiting at 100 req/s per user."

# Read body from stdin (useful for long descriptions or piping)
echo "Detailed implementation spec..." | wolfcastle task add --node my-project "Implement caching" --stdin

# Add a task with deliverables the daemon will verify on completion
wolfcastle task add --node my-project/auth "Write auth middleware" \
  --deliverable "internal/auth/middleware.go" \
  --deliverable "internal/auth/middleware_test.go"

# Add a typed task with constraints and acceptance criteria
wolfcastle task add --node my-project/api "Design REST endpoints" \
  --type spec \
  --acceptance "All endpoints documented with request/response schemas" \
  --constraint "Do not introduce GraphQL"

# Create a child task under an existing parent for decomposition
wolfcastle task add --node my-project/auth "Implement token refresh" \
  --parent task-0001 \
  --type implementation

# Add a task to a deeply nested leaf
wolfcastle task add --node attunement-tree/balance-pass/pvp "Adjust fire spell damage for PvP"
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Node not found | `{"ok": false, "error": "node 'foo/bar' not found in tree", "code": "NODE_NOT_FOUND"}` | 2 |
| Node is not a leaf | `{"ok": false, "error": "cannot add tasks to orchestrator node 'foo/bar': use wolfcastle project create for child nodes", "code": "INVALID_NODE_TYPE"}` | 3 |
| Empty title | `{"ok": false, "error": "task title cannot be empty. Name the target", "code": "EMPTY_TITLE"}` | 4 |
| Invalid task type | `{"ok": false, "error": "invalid task type \"foo\": must be one of discovery, spec, adr, implementation, integration, cleanup", "code": "INVALID_TASK_TYPE"}` | 5 |

---

## wolfcastle task amend

### Synopsis

```
wolfcastle task amend --node <task-address> [--body "<text>"] [--type <type>] [--integration "<text>"] [--add-deliverable "<path>"] [--add-constraint "<text>"] [--add-acceptance "<text>"] [--add-reference "<text>"]
```

### Description

Modifies fields on a task that has not yet started or is blocked. Tasks in `in_progress` or `complete` state cannot be amended. Only the flags you provide are applied; everything else stays untouched. List fields (deliverables, constraints, acceptance criteria, references) are appended with deduplication, so adding a value that already exists is a no-op.

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `--node <task-address>` | string | Yes | -- | Full task address: node-path/task-id (e.g. `my-project/task-0001`) |
| `--body "<text>"` | string | No | -- | Replace the task's body/description text |
| `--type <type>` | string | No | -- | Set task type. Must be one of: `discovery`, `spec`, `adr`, `implementation`, `integration`, `cleanup` |
| `--integration "<text>"` | string | No | -- | Set how this task connects to other work |
| `--add-deliverable "<path>"` | string slice | No | -- | Append a deliverable path (repeatable, deduplicated) |
| `--add-constraint "<text>"` | string slice | No | -- | Append a constraint (repeatable, deduplicated) |
| `--add-acceptance "<text>"` | string slice | No | -- | Append an acceptance criterion (repeatable, deduplicated) |
| `--add-reference "<text>"` | string slice | No | -- | Append a reference (repeatable, deduplicated) |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Parse `--node` as a task address using `tree.SplitTaskAddress`, extracting the node path and task ID. Fail if the address does not contain a task ID component.
4. Load the node's `state.json` via `MutateNode`.
5. Find the task by ID within the node's task list. Fail if no task matches.
6. Validate the task's state is `not_started` or `blocked`. Fail if the task is `in_progress` or `complete`.
7. If `--type` is provided, validate it against the allowed set (`discovery`, `spec`, `adr`, `implementation`, `integration`, `cleanup`). Fail on invalid values.
8. Apply provided scalar fields:
   - If `--body` is non-empty, replace the task's body.
   - If `--type` is non-empty, replace the task's type.
   - If `--integration` is non-empty, replace the task's integration text.
9. Append list fields with deduplication (values already present are silently skipped):
   - `--add-deliverable` values appended to `Deliverables`.
   - `--add-constraint` values appended to `Constraints`.
   - `--add-acceptance` values appended to `AcceptanceCriteria`.
   - `--add-reference` values appended to `References`.
10. Write the updated node `state.json`.
11. Output the result.

### Output

```json
{
  "ok": true,
  "action": "task_amend",
  "address": "my-project/task-0001",
  "task_id": "task-0001"
}
```

Human-readable mode prints: `Amended task my-project/task-0001`

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Task amended successfully |
| 1 | `.wolfcastle/` not found, identity not configured, or `--node` missing |
| 2 | `--node` is not a valid task address (no task ID component) |
| 3 | Task not found in the specified node |
| 4 | Task state is `in_progress` or `complete` (cannot amend) |
| 5 | Invalid `--type` value |

### Examples

```bash
# Replace a task's body text
wolfcastle task amend --node my-project/task-0001 --body "Updated requirements after discovery phase"

# Add deliverables to an existing task
wolfcastle task amend --node my-project/task-0001 --add-deliverable "docs/api.md" --add-deliverable "docs/schema.md"

# Set the task type and integration context
wolfcastle task amend --node my-project/task-0001 --type implementation --integration "feeds into auth module"

# Add acceptance criteria and constraints
wolfcastle task amend --node my-project/task-0001 --add-acceptance "all tests pass" --add-constraint "no new dependencies"

# Combine multiple amendments in one call
wolfcastle task amend --node my-project/task-0001 --body "Revised spec" --type spec --add-reference "docs/prior-art.md"
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Invalid task address | `{"ok": false, "error": "--node must be a task address: ...", "code": "INVALID_ADDRESS"}` | 2 |
| Task not found | `{"ok": false, "error": "task task-0099 not found in node my-project", "code": "TASK_NOT_FOUND"}` | 3 |
| Wrong state | `{"ok": false, "error": "cannot amend task task-0001: state is in_progress (must be not_started or blocked)", "code": "INVALID_STATE"}` | 4 |
| Invalid type | `{"ok": false, "error": "invalid task type \"bogus\": must be one of discovery, spec, adr, implementation, integration, cleanup", "code": "INVALID_TYPE"}` | 5 |

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
wolfcastle task block --node attunement-tree/earth-impl/task-5 "Flaky test infrastructure: needs human intervention"
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

## wolfcastle task deliverable

### Synopsis

```
wolfcastle task deliverable --node <path> "<file-path>"
```

### Description

Declares a file that a task is expected to produce. The daemon verifies all deliverables exist before accepting `WOLFCASTLE_COMPLETE`. Missing deliverables count as a failure and the model is re-invoked. Deliverables accumulate: multiple calls append to the task's deliverable list.

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Tree address of the task to attach the deliverable to (ADR-008) |
| `"<file-path>"` | string (positional) | Yes | -- | Path to the file the task must produce (relative to the repository root) |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the root state index at `projects/{identity}/state.json` (ADR-024).
4. Validate the `--node` path exists and points to a leaf task.
5. Load the node's co-located `state.json`.
6. Append the file path to the task's `deliverables` array. Reject duplicates.
7. Write the updated node `state.json`.
8. Output the result as JSON.

### Output

```json
{
  "ok": true,
  "action": "deliverable_added",
  "node": "my-project/task-0001",
  "path": "docs/pos-research.md",
  "deliverable_count": 2
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Deliverable added successfully |
| 1 | `.wolfcastle/` not found or identity not configured |
| 2 | Node path not found |
| 3 | Node is not a leaf task |
| 4 | File path is empty |

### Examples

```bash
# Declare a deliverable on a task
wolfcastle task deliverable "docs/pos-research.md" --node pizza-docs/task-0001

# Declare a code file as a deliverable
wolfcastle task deliverable "src/api/handler.go" --node my-project/task-0002
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Node not found | `{"ok": false, "error": "node 'foo/bar' not found in tree", "code": "NODE_NOT_FOUND"}` | 2 |
| Not a leaf | `{"ok": false, "error": "node 'foo/bar' is not a leaf task", "code": "INVALID_NODE_TYPE"}` | 3 |
| Empty path | `{"ok": false, "error": "deliverable path must not be empty", "code": "EMPTY_PATH"}` | 4 |
| Duplicate | `{"ok": false, "error": "deliverable 'foo.md' already declared on this task", "code": "DUPLICATE_DELIVERABLE"}` | 1 |

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
wolfcastle project create "<name>" [--node <parent>] [--type <leaf|orchestrator>] [--description "<text>"] [--scope "<text>"]
```

### Description

Creates a new project or sub-project node. When `--node` is omitted, creates a root-level project. When `--node` is provided, creates a child node under the specified parent. The `--type` flag controls whether the new node is a leaf (holds tasks) or an orchestrator (holds child projects); it defaults to `leaf`. Used during discovery to structure work into a tree hierarchy.

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `"<name>"` | string (positional) | Yes | -- | Name for the new project node (used as its slug in the tree path) |
| `--node <parent>` | string | No | (none, root level) | Tree address of the parent node (ADR-008). When omitted, the project is created at the root level |
| `--type <leaf\|orchestrator>` | string | No | `"leaf"` | Node type. `leaf` nodes hold tasks; `orchestrator` nodes hold child projects |
| `--description "<text>"` | string | No | `""` | Project description text written to the project `.md` file. When empty, a placeholder is used |
| `--scope "<text>"` | string | No | `""` | Planning scope for orchestrator nodes. Sets the node's `Scope` field, which the planning agent reads to understand what the orchestrator covers. Ignored for leaf nodes |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the root state index at `projects/{identity}/state.json` (ADR-024).
4. Validate `--type` is either `"leaf"` or `"orchestrator"`. Fail with an error if unrecognized.
5. If `--node` is provided, validate the parent path exists in the tree index. If the parent is currently a leaf node with no non-audit tasks, auto-promote it to an orchestrator (clear its task list, update its type in both node state and root index). If the parent is a leaf with existing tasks, fail with an error.
6. Generate a slug from the name (lowercase, hyphens for spaces, strip special characters). Validate the slug.
7. Check that no sibling node already has the same slug. If collision, append a numeric suffix.
8. Create the new project's directory at `projects/{identity}/{parent-path}/{slug}/` and write its co-located `state.json` (ADR-024):
   - `name`: the provided name
   - `type`: the value of `--type` (`"leaf"` or `"orchestrator"`)
   - For orchestrator nodes: set `Scope` to `--scope` if provided, otherwise `--description` if provided, otherwise the project name
   - `children`: `[]`
   - `audit`: `null`
9. For leaf nodes, write the audit task Markdown template into the node directory.
10. Create the project description Markdown file at `projects/{identity}/{parent-path}/{slug}/{slug}.md` containing the project name as a heading and `--description` as body text (or a placeholder if `--description` is empty).
11. Append the new node to the parent's `children` list in the parent's `state.json`.
12. Update the root `state.json` index to register the new node. If this is the first root-level project, set it as the tree root.
13. Output the result as JSON.
14. If overlap advisory is enabled in config, run overlap detection using the project name and description.

### Output

```json
{
  "ok": true,
  "action": "project_create",
  "address": "attunement-tree/fire-implementation",
  "type": "leaf",
  "name": "Fire Implementation"
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Project created successfully |
| 1 | `.wolfcastle/` not found or identity not configured |
| 2 | Parent node path not found |
| 3 | Parent is a leaf with existing tasks (cannot nest under a leaf that has tasks) |
| 4 | Name is empty or slug is invalid |
| 5 | Unknown `--type` value (not `"leaf"` or `"orchestrator"`) |

### Examples

```bash
# Create a root-level leaf project (default type)
wolfcastle project create "Attunement Tree"

# Create an orchestrator project
wolfcastle project create --type orchestrator "API Gateway"

# Create a sub-project under an existing parent
wolfcastle project create --node attunement-tree "Fire Implementation"

# Create a leaf with a description
wolfcastle project create --node attunement-tree "Water Implementation" --description "Implement water attunement abilities and resistance calculations"

# Create an orchestrator with scope for the planning agent
wolfcastle project create --type orchestrator --node api-gateway "Auth Module" --scope "JWT-based authentication: token issuance, validation, refresh, and revocation"
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Parent not found | `{"ok": false, "error": "parent node \"foo/bar\" not found. Check your address", "code": "NODE_NOT_FOUND"}` | 2 |
| Parent is leaf with tasks | `{"ok": false, "error": "cannot create child under leaf \"foo/bar\": it has 3 existing task(s). Remove tasks before decomposing", "code": "INVALID_NODE_TYPE"}` | 3 |
| Empty/invalid name | `{"ok": false, "error": "invalid project name: ...", "code": "INVALID_NAME"}` | 4 |
| Invalid type | `{"ok": false, "error": "unknown node type \"custom\": pick 'leaf' or 'orchestrator'", "code": "INVALID_TYPE"}` | 5 |

---

## wolfcastle orchestrator criteria

### Synopsis

```
wolfcastle orchestrator criteria --node <path> ["<criterion>"] [--list]
```

### Description

Manages success criteria on an orchestrator node. In its default (add) mode, it appends a success criterion to the node's `SuccessCriteria` list in `state.json`. Duplicates are silently ignored. In list mode (`--list`), it displays the node's current criteria without modification. Exactly one of a positional criterion argument or `--list` must be provided; omitting both is an error.

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Tree address of the target node (ADR-008) |
| `"<criterion>"` | string (positional) | No | -- | Success criterion text to add. Required unless `--list` is set |
| `--list` | boolean | No | `false` | List current success criteria instead of adding one |

### Behavior

#### Add mode (default)

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Validate `--node` is provided. Fail if absent.
4. Validate a positional criterion argument is present. Fail if missing and `--list` is not set.
5. Validate the criterion text is not blank after trimming whitespace.
6. Call `MutateNode` on the target node:
   - Iterate the existing `SuccessCriteria` slice. If the criterion already exists (exact string match), return without modification.
   - Otherwise, append the criterion to the slice.
7. Write the updated node `state.json`.
8. Output the result.

#### List mode (`--list`)

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Validate `--node` is provided. Fail if absent.
4. Read the node's `state.json`.
5. Output the node's `SuccessCriteria` list.
   - Human mode: if the list is empty, print `No success criteria defined for <node>`. Otherwise print each criterion as a bulleted line.
   - JSON mode: output the full criteria array.

### Output

**Add mode (JSON):**

```json
{
  "ok": true,
  "action": "success_criteria_add",
  "node": "my-project",
  "criterion": "all tests pass"
}
```

**List mode (JSON):**

```json
{
  "ok": true,
  "action": "success_criteria",
  "node": "my-project",
  "criteria": ["all tests pass", "lint clean"]
}
```

**Add mode (human):**

```
Added success criterion to my-project: all tests pass
```

**List mode (human):**

```
Success criteria for my-project:
  - all tests pass
  - lint clean
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Criterion added or criteria listed successfully |
| 1 | `.wolfcastle/` not found, identity not configured, or `--node` missing |
| 2 | Node path not found |
| 3 | Criterion text is empty or missing (neither positional arg nor `--list` provided) |

### Examples

```bash
# Add a success criterion to a project node
wolfcastle orchestrator criteria --node my-project "all tests pass"

# Add another (duplicates are silently ignored)
wolfcastle orchestrator criteria --node my-project "all tests pass"

# List current criteria
wolfcastle orchestrator criteria --node my-project --list

# JSON output for add
wolfcastle orchestrator criteria --node my-project --json "lint clean"

# JSON output for list
wolfcastle orchestrator criteria --node my-project --list --json
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Node not found | `{"ok": false, "error": "node 'foo/bar' not found in tree", "code": "NODE_NOT_FOUND"}` | 2 |
| No criterion and no --list | `{"ok": false, "error": "criterion text is required (or use --list to view existing criteria)", "code": "MISSING_CRITERION"}` | 3 |
| Empty criterion text | `{"ok": false, "error": "criterion text cannot be empty", "code": "EMPTY_TEXT"}` | 3 |
| --node missing | `{"ok": false, "error": "--node is required: specify the target node address", "code": "MISSING_FLAG"}` | 1 |

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

## wolfcastle spec create

### Synopsis

```
wolfcastle spec create [--node <path>] [--body "<text>"] [--stdin] "<title>"
```

### Description

Creates a new specification document in `.wolfcastle/docs/specs/` and optionally links it to a node. The filename follows the ISO 8601 timestamp format (ADR-011). Specs are Markdown files that travel with work: when linked to a node, they are injected into the model's context during task execution on that node. The spec body can be provided inline via `--body`, piped through `--stdin`, or left empty for a placeholder template.

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `"<title>"` | string (positional) | Yes | -- | Title of the spec |
| `--node <path>` | string | No | -- | Link the new spec to this node immediately |
| `--body "<text>"` | string | No | `""` | Spec body text. When provided, this content replaces the default placeholder template |
| `--stdin` | boolean | No | `false` | Read spec body from standard input instead of using `--body` or the template. Takes precedence over `--body` |
| `--json` | boolean | No | `false` | Output as structured JSON |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. If `--node` is provided, resolve engineer identity and validate the node exists.
3. Generate the filename:
   - Get the current UTC timestamp with minute precision.
   - Format as `{YYYY}-{MM}-{DD}T{HH}-{mm}Z-{slug}.md` (ADR-011).
   - Slug is derived from the title (lowercase, hyphens for spaces, strip special characters).
4. Ensure the `docs/specs/` directory exists (create if needed).
5. Assemble the spec content:
   - If `--stdin` is set, read body from standard input and trim surrounding whitespace.
   - Otherwise, if `--body` is provided, use that string as the body.
   - If neither is provided, use the placeholder: `[Spec content goes here.]`
6. Write the spec file with the title as a Markdown heading followed by the assembled body:
   ```markdown
   # {title}

   {body}
   ```
7. If `--node` is provided, load the node's `state.json` and append the filename to its `specs` array.
8. Output the result.

### Output

Human-readable:

```
Created spec: .wolfcastle/docs/specs/2026-03-14T12-00Z-authentication-protocol.md
Linked to node: backend/auth
```

Without `--node`:

```
Created spec: .wolfcastle/docs/specs/2026-03-14T12-00Z-authentication-protocol.md
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "spec_create",
  "data": {
    "title": "Authentication Protocol",
    "filename": "2026-03-14T12-00Z-authentication-protocol.md",
    "path": ".wolfcastle/docs/specs/2026-03-14T12-00Z-authentication-protocol.md",
    "node": "backend/auth"
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Spec created |
| 1 | `.wolfcastle/` not found, title is empty, node not found, or filesystem error |

### Examples

```bash
# Create a spec (no node link) -- uses placeholder template
wolfcastle spec create "Authentication Protocol"

# Create a spec and link it to a node
wolfcastle spec create --node backend/auth "Authentication Protocol"

# Create a spec with inline body text
wolfcastle spec create "Token Refresh Spec" --body "## Overview\n\nTokens are refreshed using a sliding-window strategy with a 15-minute grace period."

# Create a spec with body piped from stdin
echo "## API Contract\n\nAll endpoints return JSON." | wolfcastle spec create --stdin "API Contract Spec"

# Combine --stdin with --node to create and link in one step
cat design-notes.md | wolfcastle spec create --stdin --node backend/auth "Auth Design Notes"

# Create with JSON output
wolfcastle spec create "Token Refresh Spec" --json
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Empty title | `{"ok": false, "error": "spec title cannot be empty", "code": "EMPTY_TITLE"}` | 1 |
| Node not found | `{"ok": false, "error": "node 'foo/bar' not found in tree", "code": "NODE_NOT_FOUND"}` | 1 |

---

## wolfcastle spec link

### Synopsis

```
wolfcastle spec link --node <path> "<filename>"
```

### Description

Links an existing spec file to a project node. The spec must already exist in `.wolfcastle/docs/specs/`. Once linked, the spec is injected into the model's context during task execution on that node. A single spec can be linked to multiple nodes for cross-cutting concerns.

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `"<filename>"` | string (positional) | Yes | -- | Spec filename to link (must exist in `docs/specs/`) |
| `--node <path>` | string | Yes | -- | Target node to link the spec to (ADR-008) |
| `--json` | boolean | No | `false` | Output as structured JSON |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Verify the spec file exists in `.wolfcastle/docs/specs/`. Fail if not found.
4. Validate the `--node` path exists in the tree index.
5. Load the node's `state.json`.
6. Check the node's `specs` array for duplicates. If the filename is already linked, fail.
7. Append the filename to the node's `specs` array.
8. Write the updated `state.json`.
9. Output the result.

### Output

Human-readable:

```
Linked 2026-03-14T12-00Z-authentication-protocol.md to backend/auth
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "spec_link",
  "data": {
    "filename": "2026-03-14T12-00Z-authentication-protocol.md",
    "node": "backend/auth"
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Spec linked |
| 1 | `.wolfcastle/` not found or identity not configured |
| 2 | Node not found |

### Examples

```bash
# Link a spec to a node
wolfcastle spec link --node backend/auth 2026-03-14T12-00Z-authentication-protocol.md

# Link with JSON output
wolfcastle spec link --node backend/auth auth-spec.md --json
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| No identity | `fatal: identity not configured. Run 'wolfcastle init' first.` | 1 |
| Spec not found | `spec file not found: .wolfcastle/docs/specs/foo.md` | 1 |
| Node not found | `fatal: node 'foo/bar' not found in tree` | 2 |
| Already linked | `spec foo.md is already linked to backend/auth` | 1 |
| Missing --node | `--node is required: specify the target node to link the spec to` | 1 |

---

## wolfcastle spec list

### Synopsis

```
wolfcastle spec list [--node <path>] [--json]
```

### Description

Lists spec files. Without `--node`, lists all `.md` files in `.wolfcastle/docs/specs/`. With `--node`, filters to only specs referenced in that node's `state.json`. Read-only.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--node <path>` | string | No | -- | Filter to specs linked to this node |
| `--json` | boolean | No | `false` | Output as structured JSON |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. If `--node` is provided, resolve engineer identity and validate the node exists.
3. Read all `.md` files in `.wolfcastle/docs/specs/` (excluding `README.md`). Skip directories and non-Markdown files. Deduplicate by filename.
4. If `--node` is provided, load the node's `state.json` and filter the file list to only those filenames present in the node's `specs` array.
5. Display the list.

### Output

Human-readable:

```
  2026-03-14T12-00Z-authentication-protocol.md
  2026-03-12T09-30Z-rate-limiting-design.md
```

No specs found:

```
No specs found
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "spec_list",
  "data": {
    "specs": [
      {"filename": "2026-03-14T12-00Z-authentication-protocol.md"},
      {"filename": "2026-03-12T09-30Z-rate-limiting-design.md"}
    ],
    "count": 2
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Success (including empty list) |
| 1 | `.wolfcastle/` not found, identity not configured, or node not found |

### Examples

```bash
# List all specs
wolfcastle spec list

# List specs linked to a specific node
wolfcastle spec list --node backend/auth

# List as JSON
wolfcastle spec list --json
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| Node not found | `fatal: node 'foo/bar' not found in tree` | 1 |

---

## wolfcastle knowledge add

### Synopsis

```
wolfcastle knowledge add "<entry>"
```

### Description

Appends a knowledge entry to the current namespace's knowledge file at `.wolfcastle/docs/knowledge/<namespace>.md`. Knowledge files accumulate codebase-specific insights across tasks: build quirks, undocumented conventions, architecture gotchas, test patterns. The entire file is injected into every task's execution context, so entries should be concrete, durable, and non-obvious.

The entry is checked against the configured token budget (`knowledge.max_tokens`, default 2000) before writing. If the new entry would push the file over budget, the command fails without writing anything. No entry is silently lost to truncation.

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `"<entry>"` | string (positional) | Yes | -- | The knowledge entry to append |
| `--json` | boolean | No | `false` | Output as structured JSON |

### Behavior

1. Validate that the entry argument is present. Fail if missing.
2. Resolve engineer identity. Fail if not configured.
3. Validate the entry is not blank after trimming whitespace.
4. Load configuration to read `knowledge.max_tokens`.
5. Check the token budget: estimate the token count of the existing file plus the new entry. If the total would exceed `max_tokens`, fail with an error directing the user to run `wolfcastle knowledge prune`.
6. Append the entry to the namespace's knowledge file. The `knowledge.Append` function handles formatting (adds `- ` prefix if missing, appends a newline). Creates the file and parent directories if they do not exist.
7. Output the result.

### Output

Human-readable:

```
Knowledge: the integration tests require docker compose up before running
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "knowledge_add",
  "data": {
    "entry": "the integration tests require docker compose up before running",
    "namespace": "wild-macbook-pro",
    "path": ".wolfcastle/docs/knowledge/wild-macbook-pro.md"
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Entry appended successfully |
| 1 | `.wolfcastle/` not found, identity not configured, or config load failure |

### Examples

```bash
# Add a build quirk
wolfcastle knowledge add "the integration tests require docker compose up before running"

# Add an architecture note
wolfcastle knowledge add "state.Store serializes mutations through a file lock"
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Missing argument | `missing required argument: <entry>` | 1 |
| Empty entry | `empty entry. Knowledge worth recording shouldn't be blank` | 1 |
| Over budget | `Knowledge file exceeds budget (N/M tokens). Run 'wolfcastle knowledge prune' to review and consolidate.` | 1 |

---

## wolfcastle knowledge edit

### Synopsis

```
wolfcastle knowledge edit
```

### Description

Opens the current namespace's knowledge file in `$EDITOR` for free-form editing. If the file does not exist, creates it with an empty template (`# Codebase Knowledge\n\n`) before opening. Falls back to `vi` when `$EDITOR` is not set.

In JSON mode, the command reports the file path and editor name without launching the editor. This allows programmatic callers to handle editing themselves.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--json` | boolean | No | `false` | Report path and editor as JSON instead of opening the editor |

### Behavior

1. Resolve engineer identity. Fail if not configured.
2. Determine the knowledge file path for the current namespace.
3. If the file does not exist, create it with an empty template. Create parent directories as needed.
4. Read `$EDITOR` from the environment. Fall back to `vi` if unset.
5. If `--json`: output the path and editor name, then return without launching the editor.
6. Otherwise, launch the editor with the knowledge file. Connect stdin, stdout, and stderr to the terminal.
7. If the editor exits with a non-zero status, propagate the error.

### Output

JSON (`--json`):

```json
{
  "ok": true,
  "action": "knowledge_edit",
  "data": {
    "path": ".wolfcastle/docs/knowledge/wild-macbook-pro.md",
    "editor": "vim"
  }
}
```

In interactive mode, the editor takes over the terminal. No command output is printed.

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Editor exited successfully |
| 1 | `.wolfcastle/` not found, identity not configured, directory creation failure, or editor error |

### Examples

```bash
# Open in default editor
wolfcastle knowledge edit

# Open in a specific editor
EDITOR=nano wolfcastle knowledge edit

# Get file path without opening (for programmatic use)
wolfcastle knowledge edit --json
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Identity not configured | `fatal: identity not configured. Run 'wolfcastle init' first.` | 1 |
| Directory creation failure | `creating knowledge directory: <os error>` | 1 |
| Editor failure | `editor exited with error: <exit status>` | 1 |

---

## wolfcastle knowledge prune

### Synopsis

```
wolfcastle knowledge prune
```

### Description

Opens the knowledge file in `$EDITOR` for manual pruning: removing stale entries, consolidating related ones, and bringing the file under its token budget. After the editor closes, reports the new token count relative to the configured budget. Falls back to `vi` when `$EDITOR` is not set.

In JSON mode (used by the daemon's maintenance task), the command skips the editor entirely and reports the current token count and budget status. This allows non-interactive callers to check whether pruning is needed.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--json` | boolean | No | `false` | Report token count and budget status without opening the editor |

### Behavior

1. Resolve engineer identity. Fail if not configured.
2. Load configuration to read `knowledge.max_tokens`.
3. Determine the knowledge file path for the current namespace.
4. If `--json`:
   a. Read the knowledge file content.
   b. Count tokens and compare against the budget.
   c. Output the status as JSON. Return without opening the editor.
5. Otherwise (interactive mode):
   a. If the file does not exist, create it with an empty template.
   b. Read `$EDITOR` from the environment. Fall back to `vi` if unset.
   c. Launch the editor. Connect stdin, stdout, and stderr to the terminal.
   d. If the editor exits with an error, propagate it.
   e. After the editor closes, re-read the file and count tokens.
   f. Print the token count relative to the budget. If still over budget, indicate so.

### Output

Interactive (under budget after editing):

```
Token count: 1200/2000
```

Interactive (still over budget):

```
Token count: 2300/2000 (still over budget)
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "knowledge_prune",
  "data": {
    "namespace": "wild-macbook-pro",
    "path": ".wolfcastle/docs/knowledge/wild-macbook-pro.md",
    "token_count": 1847,
    "max_tokens": 2000,
    "over_budget": false
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Prune completed (or status reported in JSON mode) |
| 1 | `.wolfcastle/` not found, identity not configured, config load failure, or editor error |

### Examples

```bash
# Open for manual pruning
wolfcastle knowledge prune

# Check budget status without editing (daemon use)
wolfcastle knowledge prune --json
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Identity not configured | `fatal: identity not configured. Run 'wolfcastle init' first.` | 1 |
| Config load failure | `loading config: <error>` | 1 |
| Editor failure | `editor exited with error: <exit status>` | 1 |

---

## wolfcastle knowledge show

### Synopsis

```
wolfcastle knowledge show
```

### Description

Reads and prints the current namespace's knowledge file. If no knowledge has been recorded yet (the file does not exist or is empty), reports that clearly rather than printing nothing. In JSON mode, includes the token count alongside the content for budget awareness.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--json` | boolean | No | `false` | Output as structured JSON with token count |

### Behavior

1. Resolve engineer identity. Fail if not configured.
2. Determine the knowledge file path for the current namespace.
3. Read the knowledge file via `knowledge.Read`. If the file does not exist, returns empty content (not an error).
4. If `--json`: output the content, namespace, path, and token count as structured JSON.
5. If not `--json`:
   - If content is empty, print a message indicating no knowledge has been recorded.
   - Otherwise, print the raw file content.

### Output

Human-readable (knowledge exists):

```
## Build and Environment

- The `make test` target runs with `-short` by default.
- Go 1.26 changed the loop variable semantics.
```

Human-readable (no knowledge):

```
No codebase knowledge recorded yet for wild-macbook-pro.
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "knowledge_show",
  "data": {
    "namespace": "wild-macbook-pro",
    "content": "## Build and Environment\n\n- The `make test` target ...",
    "path": ".wolfcastle/docs/knowledge/wild-macbook-pro.md",
    "token_count": 342
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Success (including when no knowledge file exists) |
| 1 | `.wolfcastle/` not found or identity not configured |

### Examples

```bash
# Display knowledge
wolfcastle knowledge show

# Get as JSON with token count
wolfcastle knowledge show --json
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Identity not configured | `fatal: identity not configured. Run 'wolfcastle init' first.` | 1 |

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
     "task": "<resolved from daemon execution context: full tree address of the active task>",
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

## wolfcastle audit enrich

### Synopsis

```
wolfcastle audit enrich --node <path> "<text>"
```

### Description

Appends enrichment text to a node's audit enrichment list. Enrichment entries provide additional context an auditor should consider when evaluating the node, such as areas to scrutinize or cross-cutting concerns that surfaced during implementation. Duplicate entries are silently ignored.

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Tree address of the target node (ADR-008) |
| `"<text>"` | string (positional) | Yes | -- | Enrichment text describing the context to add |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the node's co-located `state.json` at `projects/{identity}/{node-path}/state.json`.
4. Validate the `--node` path exists in the tree.
5. Trim and validate that the positional text argument is non-empty.
6. Scan the node's existing `AuditEnrichment` list for an exact match. If the text already exists, skip the append (silent deduplication).
7. Append the text to the node's `AuditEnrichment` list.
8. Write the updated node `state.json`.
9. Output the result.

### Output

```json
{
  "ok": true,
  "action": "audit_enrich",
  "node": "my-project/auth",
  "text": "check error handling in auth module"
}
```

Human-readable mode prints:

```
Added audit enrichment to my-project/auth: check error handling in auth module
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Enrichment added (or already present) |
| 1 | `.wolfcastle/` not found or identity not configured |
| 2 | Node path not found |
| 3 | Text is empty |

### Examples

```bash
# Add an enrichment note for auditors
wolfcastle audit enrich --node my-project "check error handling in auth module"

# Add a second enrichment; duplicates are silently ignored
wolfcastle audit enrich --node my-project "verify backward compatibility"
wolfcastle audit enrich --node my-project "check error handling in auth module"  # no-op
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Node not found | `{"ok": false, "error": "node 'foo/bar' not found in tree", "code": "NODE_NOT_FOUND"}` | 2 |
| Empty text | `{"ok": false, "error": "enrichment text cannot be empty. Describe the context to add", "code": "EMPTY_TEXT"}` | 3 |
| Missing --node | `{"ok": false, "error": "--node is required: specify the target node address", "code": "MISSING_FLAG"}` | 1 |

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
  "gap": "Integration between fire and stamina modules not verified: needs cross-module test"
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
  "Integration between fire and stamina modules not verified: needs cross-module test"

# Escalate a gap at the sub-project level to the project level
wolfcastle audit escalate --node attunement-tree/fire-impl \
  "No end-to-end test covering the full fire attunement flow"
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Node not found | `{"ok": false, "error": "node 'foo/bar' not found in tree", "code": "NODE_NOT_FOUND"}` | 2 |
| Root node | `{"ok": false, "error": "cannot escalate from root node: no parent to escalate to", "code": "NO_PARENT"}` | 3 |
| Empty gap | `{"ok": false, "error": "gap description must not be empty", "code": "EMPTY_GAP"}` | 4 |

---

## wolfcastle audit show

### Synopsis

```
wolfcastle audit show --node <path> [--json]
```

### Description

Displays the complete audit record for a node: scope, breadcrumbs, gaps, escalations, status, and result summary. A single command that gives the full picture of a node's audit state. Read-only.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Tree address of the target node (ADR-008) |
| `--json` | boolean | No | `false` | Output as structured JSON instead of human-readable text |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the root state index at `projects/{identity}/state.json` (ADR-024).
4. Validate the `--node` path exists in the tree index.
5. Load the node's co-located `state.json` at `projects/{identity}/{node-path}/state.json`.
6. Render every field in the node's `audit` object:
   - Status
   - Scope (description, files, systems, criteria)
   - Breadcrumbs (chronological with timestamps)
   - Gaps (ID, status, description)
   - Escalations (ID, status, source node, description)
   - Result summary (if present)
7. Output as human-readable text or JSON.

### Output

Human-readable (default):

```
Audit for attunement-tree/fire-impl
  Status: in_progress
  Scope: Verify fire attunement combat integration
    Files: ["fire_ability.go","stamina.go"]
    Systems: ["combat","stamina"]
    Criteria: ["no regressions in PvP balance","stamina cost applied correctly"]
  Breadcrumbs (3):
    [2026-03-12 18:30] attunement-tree/fire-impl/task-1: Implemented base fire ability
    [2026-03-12 18:35] attunement-tree/fire-impl/task-2: Added stamina cost deduction
    [2026-03-12 18:40] attunement-tree/fire-impl/task-3: Wrote integration tests
  Gaps (1):
    gap-fire-impl-1 [open]: Missing edge case test for zero stamina
  Escalations (0):
  Result Summary: (none)
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "audit_show",
  "data": {
    "status": "in_progress",
    "scope": {
      "description": "Verify fire attunement combat integration",
      "files": ["fire_ability.go", "stamina.go"],
      "systems": ["combat", "stamina"],
      "criteria": ["no regressions in PvP balance", "stamina cost applied correctly"]
    },
    "breadcrumbs": [
      {
        "timestamp": "2026-03-12T18:30:00Z",
        "task": "attunement-tree/fire-impl/task-1",
        "text": "Implemented base fire ability"
      }
    ],
    "gaps": [
      {
        "id": "gap-fire-impl-1",
        "status": "open",
        "description": "Missing edge case test for zero stamina"
      }
    ],
    "escalations": [],
    "result_summary": ""
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Audit state displayed |
| 1 | `.wolfcastle/` not found, identity not configured, or node not found |

### Examples

```bash
# Show full audit state for a node
wolfcastle audit show --node attunement-tree/fire-impl

# Show as JSON for scripting
wolfcastle audit show --node attunement-tree/fire-impl --json

# Pipe JSON to jq to inspect gaps
wolfcastle audit show --node attunement-tree/fire-impl --json | jq '.data.gaps'
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| No identity | `fatal: identity not configured. Run 'wolfcastle init' first.` | 1 |
| Node not found | `fatal: node 'foo/bar' not found in tree` | 1 |

---

## wolfcastle audit summary

### Synopsis

```
wolfcastle audit summary --node <path> "<text>"
```

### Description

Sets the final result summary on a node's audit record. This is the short, human-readable conclusion that goes into the archive entry. Typically called by the model before signaling `WOLFCASTLE_COMPLETE` on the final task of a node.

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Tree address of the target node (ADR-008) |
| `"<text>"` | string (positional) | Yes | -- | Summary text describing the outcome of the work |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the root state index at `projects/{identity}/state.json` (ADR-024).
4. Validate the `--node` path exists in the tree index.
5. Load the node's co-located `state.json`.
6. Set the `audit.summary` field to the provided text. Overwrites any previous summary.
7. Write the updated node `state.json`.
8. Output the result as JSON.

### Output

```json
{
  "ok": true,
  "action": "summary_set",
  "node": "my-project",
  "text": "Implemented JWT auth with full test coverage"
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Summary set successfully |
| 1 | `.wolfcastle/` not found or identity not configured |
| 2 | Node path not found |
| 3 | Text is empty |

### Examples

```bash
# Record the result summary before completing
wolfcastle audit summary --node my-project "Implemented JWT auth with full test coverage"

# Summarize a sub-project
wolfcastle audit summary --node auth/login "Refactored login flow to use OAuth2"
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Node not found | `{"ok": false, "error": "node 'foo/bar' not found in tree", "code": "NODE_NOT_FOUND"}` | 2 |
| Empty text | `{"ok": false, "error": "summary text cannot be empty. State the outcome", "code": "EMPTY_TEXT"}` | 3 |

---

## wolfcastle audit scope

### Synopsis

```
wolfcastle audit scope --node <path> [--description <text>] [--files <list>] [--systems <list>] [--criteria <list>] [--json]
```

### Description

Sets structured audit scope on a node: what to verify, which files, which systems, which acceptance criteria. The audit task uses this scope to determine what "correct" looks like when verifying the node's work. Fields not specified are left unchanged, so the scope can be built incrementally across multiple calls.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Tree address of the target node (ADR-008) |
| `--description <text>` | string | No | -- | Audit scope description |
| `--files <list>` | string | No | -- | Pipe-delimited list of files to audit |
| `--systems <list>` | string | No | -- | Pipe-delimited list of systems to audit |
| `--criteria <list>` | string | No | -- | Pipe-delimited list of acceptance criteria |
| `--json` | boolean | No | `false` | Output as structured JSON |

At least one of `--description`, `--files`, `--systems`, or `--criteria` is required.

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the root state index at `projects/{identity}/state.json` (ADR-024).
4. Validate the `--node` path exists in the tree index.
5. Load the node's co-located `state.json`.
6. If the node's `audit.scope` is null, initialize it as an empty scope object.
7. For each provided flag, update the corresponding scope field:
   - `--description`: set `scope.description`
   - `--files`: parse pipe-delimited string, deduplicate, set `scope.files`
   - `--systems`: parse pipe-delimited string, deduplicate, set `scope.systems`
   - `--criteria`: parse pipe-delimited string, deduplicate, set `scope.criteria`
8. Write the updated node `state.json`.
9. Output the result.

### Output

Human-readable:

```
Audit scope updated for attunement-tree/fire-impl
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "audit_scope",
  "data": {
    "description": "Verify fire attunement combat integration",
    "files": ["fire_ability.go", "stamina.go"],
    "systems": ["combat", "stamina"],
    "criteria": ["no regressions in PvP balance"]
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Scope updated |
| 1 | `.wolfcastle/` not found, identity not configured, node not found, or no fields specified |

### Examples

```bash
# Set description and files
wolfcastle audit scope --node attunement-tree/fire-impl \
  --description "Verify fire attunement combat integration" \
  --files "fire_ability.go|stamina.go"

# Add criteria on a subsequent call (description and files are preserved)
wolfcastle audit scope --node attunement-tree/fire-impl \
  --criteria "no regressions in PvP balance|stamina cost applied correctly"

# Set systems
wolfcastle audit scope --node attunement-tree/fire-impl --systems "combat|stamina"
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| No identity | `fatal: identity not configured. Run 'wolfcastle init' first.` | 1 |
| Node not found | `fatal: node 'foo/bar' not found in tree` | 1 |
| No fields | `at least one scope field is required (--description, --files, --systems, --criteria)` | 1 |

---

## wolfcastle audit gap

### Synopsis

```
wolfcastle audit gap --node <path> "<description>"
```

### Description

Records a gap in a node's audit record. Gaps are issues found during audit that need resolution before the audit can pass. Each gap receives a deterministic ID (e.g., `gap-my-project-1`), a timestamp, and an `open` status. Gaps accumulate until they are fixed with `audit fix-gap` or escalated with `audit escalate`.

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Tree address of the target node (ADR-008) |
| `"<description>"` | string (positional) | Yes | -- | What the gap is. Cannot be empty |
| `--json` | boolean | No | `false` | Output as structured JSON |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the root state index at `projects/{identity}/state.json` (ADR-024).
4. Validate the `--node` path exists in the tree index.
5. Load the node's co-located `state.json`.
6. Generate a gap ID: `gap-{node-id}-{N}` where N is one greater than the current number of gaps.
7. Append a new gap entry to the node's `audit.gaps` array:
   ```json
   {
     "id": "gap-fire-impl-1",
     "timestamp": "2026-03-12T18:45:03Z",
     "description": "missing error handling in auth module",
     "source": "attunement-tree/fire-impl",
     "status": "open"
   }
   ```
8. Write the updated node `state.json`.
9. Output the result.

### Output

Human-readable:

```
Gap gap-fire-impl-1 recorded on attunement-tree/fire-impl
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "audit_gap",
  "data": {
    "node": "attunement-tree/fire-impl",
    "gap_id": "gap-fire-impl-1"
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Gap recorded |
| 1 | `.wolfcastle/` not found, identity not configured, node not found, or empty description |

### Examples

```bash
# Record a gap during audit
wolfcastle audit gap --node attunement-tree/fire-impl "Missing error handling in auth module"

# Record a gap with JSON output
wolfcastle audit gap --node api/endpoints "No rate limiting tests" --json
```

### Error Cases

| Error | Output (JSON) | Code |
|-------|---------------|------|
| Node not found | `{"ok": false, "error": "node 'foo/bar' not found in tree", "code": "NODE_NOT_FOUND"}` | 1 |
| Empty description | `{"ok": false, "error": "gap description cannot be empty", "code": "EMPTY_DESCRIPTION"}` | 1 |

---

## wolfcastle audit fix-gap

### Synopsis

```
wolfcastle audit fix-gap --node <path> <gap-id>
```

### Description

Marks an open audit gap as fixed. The gap stays in the record for traceability (nothing is erased), but its status changes from `open` to `fixed` with a timestamp and attribution. Refuses to fix a gap that is already fixed.

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Tree address of the node containing the gap (ADR-008) |
| `<gap-id>` | string (positional) | Yes | -- | The ID of the gap to fix (e.g., `gap-my-project-1`) |
| `--json` | boolean | No | `false` | Output as structured JSON |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the root state index at `projects/{identity}/state.json` (ADR-024).
4. Validate the `--node` path exists in the tree index.
5. Load the node's co-located `state.json`.
6. Search the node's `audit.gaps` array for the given gap ID.
7. If not found, fail.
8. If the gap's status is already `fixed`, fail.
9. Transition the gap's status from `open` to `fixed`.
10. Record `fixed_by` (node address) and `fixed_at` (timestamp).
11. Write the updated node `state.json`.
12. Output the result.

### Output

Human-readable:

```
Gap gap-fire-impl-1 marked as fixed on attunement-tree/fire-impl
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "audit_fix_gap",
  "data": {
    "node": "attunement-tree/fire-impl",
    "gap_id": "gap-fire-impl-1"
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Gap marked as fixed |
| 1 | `.wolfcastle/` not found, identity not configured, node not found, gap not found, or gap already fixed |

### Examples

```bash
# Fix a gap by ID
wolfcastle audit fix-gap --node attunement-tree/fire-impl gap-fire-impl-1

# Fix with JSON output
wolfcastle audit fix-gap --node attunement-tree/fire-impl gap-fire-impl-1 --json
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| Node not found | `fatal: node 'foo/bar' not found in tree` | 1 |
| Gap not found | `gap gap-fire-impl-99 not found in attunement-tree/fire-impl` | 1 |
| Already fixed | `gap gap-fire-impl-1 is already fixed` | 1 |

---

## wolfcastle audit resolve

### Synopsis

```
wolfcastle audit resolve --node <path> <escalation-id>
```

### Description

Marks an open escalation as resolved. The escalation stays in the record for traceability, but its status changes from `open` to `resolved` with a timestamp and attribution. Refuses to resolve an escalation that is already resolved.

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Tree address of the node containing the escalation (ADR-008) |
| `<escalation-id>` | string (positional) | Yes | -- | The ID of the escalation to resolve (e.g., `escalation-my-project-1`) |
| `--json` | boolean | No | `false` | Output as structured JSON |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load the root state index at `projects/{identity}/state.json` (ADR-024).
4. Validate the `--node` path exists in the tree index.
5. Load the node's co-located `state.json`.
6. Search the node's `audit.escalations` array for the given escalation ID.
7. If not found, fail.
8. If the escalation's status is already `resolved`, fail.
9. Transition the escalation's status from `open` to `resolved`.
10. Record `resolved_by` (node address) and `resolved_at` (timestamp).
11. Write the updated node `state.json`.
12. Output the result.

### Output

Human-readable:

```
Escalation escalation-fire-impl-1 resolved on attunement-tree/fire-impl
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "audit_resolve",
  "data": {
    "node": "attunement-tree/fire-impl",
    "escalation_id": "escalation-fire-impl-1"
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Escalation resolved |
| 1 | `.wolfcastle/` not found, identity not configured, node not found, escalation not found, or already resolved |

### Examples

```bash
# Resolve an escalation by ID
wolfcastle audit resolve --node attunement-tree/fire-impl escalation-fire-impl-1

# Resolve with JSON output
wolfcastle audit resolve --node attunement-tree/fire-impl escalation-fire-impl-1 --json
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| Node not found | `fatal: node 'foo/bar' not found in tree` | 1 |
| Escalation not found | `escalation escalation-fire-impl-99 not found in attunement-tree/fire-impl` | 1 |
| Already resolved | `escalation escalation-fire-impl-1 is already resolved` | 1 |

---

## wolfcastle audit run

### Synopsis

```
wolfcastle audit run [--scope <scopes>] [--list] [--json]
```

### Description

Runs a read-only codebase audit against composable scopes. Discovers available scopes from `base/audits/`, `custom/audits/`, and `local/audits/` (all three configuration tiers). For each requested scope, invokes a model to analyze the codebase and collect findings. Saves the findings as a pending batch in `audit-state.json`.

The audit is strictly read-only. The model reads code and produces a report. It does not modify files, create branches, or write code.

Findings do not become tasks automatically. They go through an approval gate: use `wolfcastle audit approve` or `wolfcastle audit reject` to decide their fate (ADR-038).

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--scope <scopes>` | string | No | (all discovered scopes) | Comma-separated scope IDs to run. Defaults to all |
| `--list` | boolean | No | `false` | List available scopes and exit (equivalent to `audit list`) |
| `--json` | boolean | No | `false` | Output as structured JSON |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Discover available scopes by scanning `base/audits/`, `custom/audits/`, and `local/audits/` for `.md` files. Higher tiers override lower tiers when scope IDs collide. Each scope's ID is its filename without extension; its description is the first non-heading, non-empty line of the file.
4. **If `--list` is set**: display the discovered scopes (ID and description) and exit 0.
5. **Check for existing pending batch**: load `audit-state.json`. If a pending batch already exists, fail with a message directing the user to review or discard it.
6. **Filter scopes**: if `--scope` is provided, select only the requested scope IDs. Fail if any requested ID is unknown.
7. If no scopes are available after filtering, fail.
8. Resolve the audit model from `config.json` under `audit.model`.
9. Resolve the base audit prompt from `audit.prompt_file` via three-tier merge, then append each selected scope's prompt file content.
10. Invoke the model with the assembled prompt and the repository root as working directory.
11. Parse findings from the model's output (headings and numbered bold items are recognized as finding titles; subsequent text becomes the description).
12. Build a batch with a timestamped ID, the scope IDs, status `"pending"`, and the parsed findings.
13. Save the batch to `.wolfcastle/audit-state.json`.
14. Output the result.

### Output

Human-readable:

```
Running audit with 2 scope(s): security, performance

Saved 3 finding(s) for review.
  1. Missing input validation on API endpoints
  2. Unbounded query in user search
  3. No connection pool limits

Review with: wolfcastle audit pending
Approve:     wolfcastle audit approve <id>
Reject:      wolfcastle audit reject <id>
```

With `--list`:

```
Available audit scopes:
  security             Check for common security vulnerabilities
  performance          Identify performance bottlenecks
  dry                  Detect DRY violations and duplicated logic
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "audit_run",
  "data": {
    "batch_id": "audit-20260314T120000Z",
    "finding_count": 3,
    "scopes": ["security", "performance"]
  }
}
```

JSON (`--json --list`):

```json
{
  "ok": true,
  "action": "audit_list",
  "data": {
    "scopes": [
      {"id": "security", "description": "Check for common security vulnerabilities", "prompt_file": ".wolfcastle/system/base/audits/security.md"},
      {"id": "performance", "description": "Identify performance bottlenecks", "prompt_file": ".wolfcastle/system/custom/audits/performance.md"}
    ]
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Audit complete, or scopes listed |
| 1 | `.wolfcastle/` not found, identity not configured, pending batch exists, unknown scope, no scopes found, or model invocation failed |

### Examples

```bash
# Run all scopes
wolfcastle audit run

# Run specific scopes
wolfcastle audit run --scope security,performance

# List available scopes
wolfcastle audit run --list

# Run and get JSON output
wolfcastle audit run --json
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| Pending batch exists | `pending review batch exists with 3 finding(s). Use 'audit pending' to review or 'audit reject --all' to discard` | 1 |
| Unknown scope | `unknown scope "foo". Use --list to see available scopes` | 1 |
| No scopes | `no audit scopes found. Add .md files to base/audits/, custom/audits/, or local/audits/` | 1 |
| Model failure | `audit invocation failed: {error}` | 1 |

### Configuration

The model and prompt are configured in `config.json`:

```json
{
  "audit": {
    "model": "heavy",
    "prompt_file": "audit.md"
  }
}
```

---

## wolfcastle audit list

### Synopsis

```
wolfcastle audit list [--json]
```

### Description

Lists available audit scopes discovered from `base/audits/`, `custom/audits/`, and `local/audits/`. This is a standalone alias for the `audit run --list` behavior, provided for discoverability.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--json` | boolean | No | `false` | Output as structured JSON |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Discover available scopes by scanning all three configuration tiers for `.md` files under `audits/`. Higher tiers override lower tiers when scope IDs collide.
3. Display each scope's ID and description.

### Output

Human-readable:

```
Available audit scopes:
  security             Check for common security vulnerabilities
  performance          Identify performance bottlenecks
  dry                  Detect DRY violations and duplicated logic
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "audit_list",
  "data": {
    "scopes": [
      {"id": "security", "description": "Check for common security vulnerabilities", "prompt_file": ".wolfcastle/system/base/audits/security.md"}
    ]
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Scopes listed (including empty list) |
| 1 | `.wolfcastle/` not found |

### Examples

```bash
# List all available scopes
wolfcastle audit list

# List as JSON
wolfcastle audit list --json
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |

---

## wolfcastle audit pending

### Synopsis

```
wolfcastle audit pending [--json]
```

### Description

Displays the current batch of audit findings that have not yet been approved or rejected. Shows finding IDs, titles, and description previews. If no pending batch exists, reports that. This is the entry point for the approval gate (ADR-038).

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--json` | boolean | No | `false` | Output as structured JSON |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Load `audit-state.json` from `.wolfcastle/`.
3. If no file exists or the batch is null, report "No pending audit review batch." and exit 0.
4. Filter findings with status `"pending"`.
5. If all findings have been decided but the batch hasn't been archived yet, report that and suggest approving or rejecting the final finding to trigger archival.
6. Display each pending finding's ID, title, and first line of description (truncated to 80 characters).
7. Print usage hints for `approve` and `reject`.

### Output

Human-readable:

```
Pending audit findings (batch audit-20260314T120000Z, 2 scope(s)):

  [finding-1] Missing input validation on API endpoints
         Endpoints in api/handlers/ accept unvalidated user input...
  [finding-2] Stale database migration files
         Three migration files reference tables that no longer exist...

  Approve: wolfcastle audit approve <id>
  Reject:  wolfcastle audit reject <id>
  Detail:  wolfcastle audit pending --json | jq '.data.findings[] | select(.id=="<id>")'
```

No pending batch:

```
No pending audit review batch.
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "audit_pending",
  "data": {
    "batch_id": "audit-20260314T120000Z",
    "scopes": ["security", "performance"],
    "pending": 2,
    "total": 3,
    "findings": [
      {
        "id": "finding-1",
        "title": "Missing input validation on API endpoints",
        "description": "Endpoints in api/handlers/ accept unvalidated user input...",
        "status": "pending"
      },
      {
        "id": "finding-2",
        "title": "Stale database migration files",
        "description": "Three migration files reference tables that no longer exist...",
        "status": "pending"
      }
    ]
  }
}
```

JSON (no pending batch):

```json
{
  "ok": true,
  "action": "audit_pending",
  "data": {
    "pending": 0
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Always (informational command) |
| 1 | `.wolfcastle/` not found |

### Examples

```bash
# View pending findings
wolfcastle audit pending

# Get full finding details as JSON
wolfcastle audit pending --json

# Inspect a specific finding
wolfcastle audit pending --json | jq '.data.findings[] | select(.id=="finding-1")'
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |

---

## wolfcastle audit approve

### Synopsis

```
wolfcastle audit approve <finding-id>
wolfcastle audit approve --all
```

### Description

Approves a pending audit finding, creating a leaf project in the work tree. The finding's title becomes the project name; its description becomes the project's description file content. Use `--all` to approve every remaining pending finding in one pass. When all findings in the batch have been decided (approved or rejected), the batch is archived to `audit-review-history.json` with retention (100 entries, 90 days) and the pending file is removed (ADR-038).

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `<finding-id>` | string (positional) | Yes (unless `--all`) | -- | ID of the finding to approve (e.g., `finding-1`) |
| `--all` | boolean | No | `false` | Approve all remaining pending findings |
| `--json` | boolean | No | `false` | Output as structured JSON |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load `audit-state.json`. Fail if no pending batch exists.
4. Load the root state index.
5. For each targeted finding (by ID, or all pending if `--all`):
   a. Generate a slug from the finding's title.
   b. Validate the slug. If invalid (e.g., title produces empty slug), skip with a warning in `--all` mode or fail in single-finding mode.
   c. If a project with that slug already exists in the root index, mark the finding as approved without creating a duplicate.
   d. Otherwise, create a leaf project: directory, `state.json`, and description `.md` file containing the finding's title, batch ID, and description.
   e. Update the finding's status to `"approved"` with timestamp and created node address.
6. Save the updated batch file.
7. Save the updated root index (new projects were added).
8. If all findings in the batch are decided, archive the batch to `audit-review-history.json` and remove `audit-state.json`.
9. Output the result.

### Output

Human-readable:

```
  Approved: finding-1 → missing-input-validation
  Approved: finding-2 → stale-database-migrations

Approved 2 finding(s).
Batch audit-20260314T120000Z complete. Archived to history.
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "audit_approve",
  "data": {
    "approved": 2,
    "decisions": [
      {
        "finding_id": "finding-1",
        "title": "Missing input validation on API endpoints",
        "action": "approved",
        "timestamp": "2026-03-14T12:05:00Z",
        "created_node": "missing-input-validation"
      },
      {
        "finding_id": "finding-2",
        "title": "Stale database migration files",
        "action": "approved",
        "timestamp": "2026-03-14T12:05:00Z",
        "created_node": "stale-database-migrations"
      }
    ]
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Finding(s) approved |
| 1 | `.wolfcastle/` not found, identity not configured, no pending batch, finding not found, or finding already decided |

### Examples

```bash
# Approve a single finding
wolfcastle audit approve finding-1

# Approve all remaining findings
wolfcastle audit approve --all

# Approve with JSON output
wolfcastle audit approve finding-1 --json
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| No batch | `no pending review batch. Run 'wolfcastle audit run' first` | 1 |
| No args | `provide a finding ID or use --all` | 1 |
| Not found | `finding "finding-99" not found or already decided` | 1 |
| No pending | `no pending findings to approve` | 1 |

---

## wolfcastle audit reject

### Synopsis

```
wolfcastle audit reject <finding-id>
wolfcastle audit reject --all
```

### Description

Rejects a pending audit finding without creating any project. The decision is recorded for audit trail purposes. Use `--all` to reject every remaining pending finding. When all findings in the batch have been decided, the batch is archived to `audit-review-history.json` and the pending file is removed (ADR-038).

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `<finding-id>` | string (positional) | Yes (unless `--all`) | -- | ID of the finding to reject |
| `--all` | boolean | No | `false` | Reject all remaining pending findings |
| `--json` | boolean | No | `false` | Output as structured JSON |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Load `audit-state.json`. Fail if no pending batch exists.
3. For each targeted finding (by ID, or all pending if `--all`):
   a. Update the finding's status to `"rejected"` with timestamp.
   b. No project is created.
4. Save the updated batch file.
5. If all findings in the batch are decided, archive the batch to `audit-review-history.json` with retention (100 entries, 90 days) and remove `audit-state.json`.
6. Output the result.

### Output

Human-readable:

```
  Rejected: finding-3 (No connection pool limits)

Rejected 1 finding(s).
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "audit_reject",
  "data": {
    "rejected": 1,
    "decisions": [
      {
        "finding_id": "finding-3",
        "title": "No connection pool limits",
        "action": "rejected",
        "timestamp": "2026-03-14T12:10:00Z"
      }
    ]
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Finding(s) rejected |
| 1 | `.wolfcastle/` not found, no pending batch, finding not found, or finding already decided |

### Examples

```bash
# Reject a single finding
wolfcastle audit reject finding-3

# Reject all remaining findings
wolfcastle audit reject --all

# Reject with JSON output
wolfcastle audit reject finding-3 --json
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| No batch | `no pending review batch. Run 'wolfcastle audit run' first` | 1 |
| No args | `provide a finding ID or use --all` | 1 |
| Not found | `finding "finding-99" not found or already decided` | 1 |
| No pending | `no pending findings to reject` | 1 |

---

## wolfcastle audit history

### Synopsis

```
wolfcastle audit history [--json]
```

### Description

Displays the history of completed audit review batches with their decisions. Most recent batches are shown first. Each entry shows the batch ID, completion timestamp, scopes, and a summary of approved versus rejected findings with individual outcomes (ADR-038).

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--json` | boolean | No | `false` | Output as structured JSON |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Load `audit-review-history.json` from `.wolfcastle/`.
3. If no file exists or the history is empty, report "No audit review history." and exit 0.
4. Display entries in reverse chronological order (most recent first):
   - Batch ID and completion timestamp
   - Scopes that were audited
   - Counts of approved and rejected findings
   - Individual finding outcomes with `[+]` for approved and `[-]` for rejected, including created node addresses where applicable.

### Output

Human-readable:

```
Batch audit-20260314T120000Z (completed 2026-03-14 12:10)
  Scopes: [security performance]
  Decisions: 2 approved, 1 rejected
    [+] Missing input validation on API endpoints → missing-input-validation
    [+] Stale database migration files → stale-database-migrations
    [-] No connection pool limits

Batch audit-20260310T090000Z (completed 2026-03-10 09:15)
  Scopes: [dry]
  Decisions: 1 approved, 0 rejected
    [+] Duplicated HTTP client setup → duplicated-http-client
```

No history:

```
No audit review history.
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "audit_history",
  "data": {
    "entries": [
      {
        "batch_id": "audit-20260314T120000Z",
        "completed_at": "2026-03-14T12:10:00Z",
        "scopes": ["security", "performance"],
        "decisions": [
          {
            "finding_id": "finding-1",
            "title": "Missing input validation on API endpoints",
            "action": "approved",
            "timestamp": "2026-03-14T12:05:00Z",
            "created_node": "missing-input-validation"
          },
          {
            "finding_id": "finding-3",
            "title": "No connection pool limits",
            "action": "rejected",
            "timestamp": "2026-03-14T12:10:00Z"
          }
        ]
      }
    ],
    "count": 1
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | History displayed (including empty history) |
| 1 | `.wolfcastle/` not found |

### Examples

```bash
# View audit history
wolfcastle audit history

# Get history as JSON
wolfcastle audit history --json

# Count total approved findings across all batches
wolfcastle audit history --json | jq '[.data.entries[].decisions[] | select(.action=="approved")] | length'
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |

---

## wolfcastle audit aar

### Synopsis

```
wolfcastle audit aar --node <path> --task <task-id> --objective <text> --what-happened <text> [--went-well <text>]... [--improvements <text>]... [--action-items <text>]... [--json]
```

### Description

Records a structured After Action Review (AAR) for a completed task. AARs capture what was attempted, what happened, what went well, and what should change. They flow into the iteration context for subsequent tasks and into audit reports and archives.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Target node address (ADR-008) |
| `--task <task-id>` | string | Yes | -- | Task ID the AAR covers |
| `--objective <text>` | string | Yes | -- | What the task set out to do |
| `--what-happened <text>` | string | Yes | -- | What actually happened |
| `--went-well <text>` | []string | No | nil | Things that went well (repeatable) |
| `--improvements <text>` | []string | No | nil | Things that could be improved (repeatable) |
| `--action-items <text>` | []string | No | nil | Follow-up items for subsequent tasks (repeatable) |
| `--json` | boolean | No | `false` | Output as structured JSON |

### Behavior

1. Validate that `--node`, `--task`, `--objective`, and `--what-happened` are provided and non-empty.
2. Construct an AAR record with the provided fields and the current timestamp.
3. Append the AAR to the node's `audit.aars` array via `state.AddAAR`.
4. Output confirmation.

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | AAR recorded successfully |
| 1 | Missing required flags or node not found |

---

## wolfcastle audit report

### Synopsis

```
wolfcastle audit report --node <path> [--path] [--json]
```

### Description

Displays the latest audit report for a node. If a persisted report file exists on disk (from a prior `WriteAuditReport` call), displays its contents. If no report file exists, generates a live preview from the node's current audit state.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Target node address (ADR-008) |
| `--path` | boolean | No | `false` | Print only the report file path (empty string if no persisted report exists) |
| `--json` | boolean | No | `false` | Output as structured JSON |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Check for the latest persisted report file in the node's directory (files matching `audit-*.md`).
3. If `--path` is set, output the report file path (or empty/message if none exists).
4. If a report file exists, display its contents.
5. If no report file exists, load the node's `state.json`, generate a report preview via `state.GenerateAuditReport`, and display it.

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Report displayed or path printed |
| 1 | Node not found or `.wolfcastle/` missing |

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

## wolfcastle archive delete

### Synopsis

```
wolfcastle archive delete --node <path> --confirm
```

### Description

Permanently removes an archived node and its entire subtree from the index and the archive store. This operation is irreversible: once deleted, the node's state directories and index entries cannot be recovered. The `--confirm` flag is mandatory as a safeguard against accidental deletion.

Only root-level archived nodes (those present in `archived_root`) can be deleted. To delete a child of an archived subtree, you would need to restore the parent first, then manage it through normal project commands.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Tree address of the archived node to delete |
| `--confirm` | bool | Yes | `false` | Confirm permanent deletion. Command refuses to run without this flag |

### Behavior

1. Require identity. Fail if not configured.
2. Validate `--confirm` is set. If not, exit with an error explaining the flag is required.
3. Validate `--node` is provided.
4. Load the root index.
5. Verify the node exists in the index and has `archived: true`.
6. Verify the node is in `archived_root` (root-level archived entry).
7. Collect the full subtree: the target node and all its descendants via `Children` links.
8. Remove the archived state directory tree at `{store}/.archive/{node-path}/` using recursive deletion.
9. Atomically update the root index:
   - Remove the node from `archived_root`.
   - Delete all subtree entries from the `Nodes` map.
10. Output the result.

### Output

Human-readable:

```
Permanently deleted archived node my-project
```

With `--json`:

```json
{
  "ok": true,
  "action": "archive_delete",
  "node": "my-project"
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Node permanently deleted |
| 1 | `.wolfcastle/` not found, identity not configured, or `--confirm` not set |
| 2 | Node not found in index |
| 3 | Node is not archived or not a root-level archived node |

### Examples

```bash
# Permanently delete an archived project
wolfcastle archive delete --node my-project --confirm

# Attempting without --confirm fails with an explanatory error
wolfcastle archive delete --node my-project
# Error: --confirm is required: this permanently deletes the archived node and cannot be undone
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| No --confirm | `--confirm is required: this permanently deletes the archived node and cannot be undone` | 1 |
| Node not found | `node "foo" not found in index` | 2 |
| Not archived | `node "foo" is not archived` | 3 |
| Not root-level | `node "foo" is not a root-level archived node` | 3 |

---

## wolfcastle archive restore

### Synopsis

```
wolfcastle archive restore --node <path>
```

### Description

Restores a previously archived node and its entire subtree back to active state. The node must be a root-level archived entry (present in `archived_root`). After restoration, the node appears in the active `root` list and all subtree entries have their `archived` flag cleared.

State directories are moved from `.archive/` back to their active locations. The node resumes its pre-archive state; tasks, audit records, and breadcrumbs are preserved.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--node <path>` | string | Yes | -- | Tree address of the archived node to restore |

### Behavior

1. Require identity. Fail if not configured.
2. Validate `--node` is provided.
3. Load the root index.
4. Verify the node exists in the index and has `archived: true`.
5. Verify the node is in `archived_root` (root-level archived entry).
6. Collect the full subtree: the target node and all its descendants via `Children` links.
7. For each node in the subtree, move its state directory from `{store}/.archive/{node-path}/` to `{store}/{node-path}/`. Skip entries whose archive directory does not exist. Create parent directories as needed.
8. Atomically update the root index:
   - Remove the node from `archived_root`.
   - Add the node to `root`.
   - For all subtree entries, set `archived: false` and clear `archived_at`.
9. Output the result.

### Output

Human-readable:

```
Restored my-project from archive
```

With `--json`:

```json
{
  "ok": true,
  "action": "archive_restore",
  "node": "my-project"
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Node restored to active state |
| 1 | `.wolfcastle/` not found or identity not configured |
| 2 | Node not found in index |
| 3 | Node is not archived or not a root-level archived node |

### Examples

```bash
# Restore an archived project
wolfcastle archive restore --node my-project

# Verify it's back in the active tree
wolfcastle status --node my-project
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Node not found | `node "foo" not found in index` | 2 |
| Not archived | `node "foo" is not archived` | 3 |
| Not root-level | `node "foo" is not a root-level archived node` | 3 |

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
wolfcastle inbox add "Refactor the auth middleware to support OAuth2: currently only supports basic auth and API keys"
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| No identity | `fatal: identity not configured. Run 'wolfcastle init' first.` | 1 |
| Empty idea | `fatal: idea text must not be empty` | 2 |

---

## wolfcastle inbox list

### Synopsis

```
wolfcastle inbox list [--json]
```

### Description

Shows everything in the inbox, grouped by status. Read-only. Items are displayed with their index number, status, text, and timestamp.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--json` | boolean | No | `false` | Output as structured JSON |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load `inbox.json` from `projects/{identity}/`.
4. Display all items with their status (`new`, `expanded`, `filed`), text, and timestamp.

### Output

Human-readable:

```
  1. [new] Add caching layer for database queries (2026-03-14T12:00:00Z)
  2. [expanded] Refactor auth middleware (2026-03-13T09:30:00Z)
  3. [filed] Add rate limiting to API (2026-03-12T16:00:00Z)
```

Empty inbox:

```
Inbox is empty
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "inbox_list",
  "data": {
    "items": [
      {
        "timestamp": "2026-03-14T12:00:00Z",
        "text": "Add caching layer for database queries",
        "status": "new"
      }
    ],
    "count": 1
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Success (including empty inbox) |
| 1 | `.wolfcastle/` not found or identity not configured |

### Examples

```bash
# List all inbox items
wolfcastle inbox list

# List as JSON
wolfcastle inbox list --json

# Count new items
wolfcastle inbox list --json | jq '[.data.items[] | select(.status=="new")] | length'
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| No identity | `fatal: identity not configured. Run 'wolfcastle init' first.` | 1 |

---

## wolfcastle inbox clear

### Synopsis

```
wolfcastle inbox clear [--all] [--json]
```

### Description

Removes processed items from the inbox. Without `--all`, only removes items with status `filed` or `expanded` (items the daemon has already processed). With `--all`, clears everything including unprocessed `new` items.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--all` | boolean | No | `false` | Remove everything, including unprocessed `new` items |
| `--json` | boolean | No | `false` | Output as structured JSON |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity.
3. Load `inbox.json` from `projects/{identity}/`.
4. **Without `--all`**: remove items with status `filed` or `expanded`, keep items with status `new`.
5. **With `--all`**: remove all items.
6. Save the updated `inbox.json`.
7. Output the count of removed and remaining items.

### Output

Human-readable:

```
Cleared 5 items from inbox (2 remaining)
```

With `--all`:

```
Cleared 7 items from inbox (0 remaining)
```

JSON (`--json`):

```json
{
  "ok": true,
  "action": "inbox_clear",
  "data": {
    "removed": 5,
    "remaining": 2
  }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Inbox cleared (even if nothing was removed) |
| 1 | `.wolfcastle/` not found or identity not configured |

### Examples

```bash
# Clear processed items only
wolfcastle inbox clear

# Clear everything
wolfcastle inbox clear --all

# Clear and verify
wolfcastle inbox clear && wolfcastle inbox list
```

### Error Cases

| Error | Message | Code |
|-------|---------|------|
| Not initialized | `fatal: not a wolfcastle project (no .wolfcastle/ found)` | 1 |
| No identity | `fatal: identity not configured. Run 'wolfcastle init' first.` | 1 |

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

## wolfcastle config show

### Synopsis

```
wolfcastle config show [section] [--tier <base|custom|local>] [--raw] [--json]
```

### Description

Prints the Wolfcastle configuration to stdout as indented JSON. By default, the output reflects the fully resolved config: hardcoded defaults merged with base, custom, and local tiers (the same object returned by `config.Load()`). The structure matches the `Config` type in `internal/config/types.go`.

An optional positional `section` argument filters the output to a single top-level key. If the section does not exist, the command exits with an error listing valid section names. The section filter applies after mode resolution, so it composes freely with `--tier`, `--raw`, and `--json`.

Two flags modify what is shown:

- `--tier` restricts output to a single tier file's raw JSON content, before merge.
- `--raw` suppresses the hardcoded defaults layer, showing only what the tier files contain.

### Arguments and Flags

| Argument / Flag | Type | Required | Default | Description |
|-----------------|------|----------|---------|-------------|
| `section` | positional | No | (none) | Filter output to a single top-level key (e.g. `pipeline`, `models`). Unknown keys exit 1 with valid section list |
| `--tier` | string | No | (none) | Display a single tier's raw JSON. Accepted: `base`, `custom`, `local` |
| `--raw` | boolean | No | `false` | Suppress hardcoded defaults layer. Merges only tier files. No-op when combined with `--tier` |
| `--json` | boolean | No | `false` | Wrap output in the standard `{ok, action, data}` envelope |

### Behavior

1. Locate `.wolfcastle/` directory. Exit 1 if not found.
2. Resolve config based on mode:
   - **Default** (no flags): call `config.Load()` for fully merged config.
   - **`--tier`**: read the single tier file (`.wolfcastle/system/{tier}/config.json`). Missing files produce `{}`.
   - **`--raw`** (without `--tier`): deep-merge the three tier files without seeding from `Defaults()`.
   - **`--raw` with `--tier`**: identical to `--tier` alone.
3. If a `section` argument is given, extract that top-level key from the result. Exit 1 if the key does not exist.
4. Marshal to indented JSON (two-space indent, HTML escaping disabled).
5. If `--json`, wrap in the standard envelope with action `"config_show"`.

### Output

Default (merged config):

```
$ wolfcastle config show
{
  "version": 1,
  "models": { ... },
  "pipeline": { ... },
  ...
}
```

Filtered by section:

```
$ wolfcastle config show pipeline
{
  "stages": [ ... ],
  "planning": { "enabled": true, ... }
}
```

JSON envelope:

```json
{
  "ok": true,
  "action": "config_show",
  "data": { "version": 1, "models": { ... } }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Success |
| 1 | No `.wolfcastle/` directory, malformed tier file, invalid `--tier` value, or unknown section name |

### Examples

```bash
# Show fully merged config
wolfcastle config show

# Show only the pipeline section
wolfcastle config show pipeline

# Show what the local tier overrides
wolfcastle config show --tier local

# Show merged tier files without defaults
wolfcastle config show --raw

# JSON envelope for a specific section from a specific tier
wolfcastle config show models --tier base --json
```

### Error Cases

| Scenario | Behavior |
|----------|----------|
| `.wolfcastle/` not found | Exit 1: `fatal: not a wolfcastle project (no .wolfcastle/ found)` |
| Invalid `--tier` value | Exit 1: `error: --tier must be one of: base, custom, local` |
| Tier file missing | Print `{}` (not an error) |
| Tier file is malformed JSON | Exit 1: `error: {tier}/config.json is not valid JSON: {parse error}` |
| Unknown section argument | Exit 1: `error: unknown section "X"; valid sections: a, b, c` (alphabetical) |

---

## wolfcastle config set

### Synopsis

```
wolfcastle config set <key> <value> [--tier local|custom] [--json]
```

### Description

Sets a configuration key to a value in a tier overlay file. Uses dot-notation paths to address nested keys (e.g. `daemon.poll_interval_seconds`). Intermediate maps are created if they do not exist. If the key already exists, its value is replaced.

The value argument is parsed as JSON first (numbers, booleans, null, objects, arrays). If JSON parsing fails, the raw string is stored as a JSON string value. This means `wolfcastle config set logs.max_files 5` stores an integer, while `wolfcastle config set identity.user alice` stores a string, without quoting gymnastics.

### Arguments and Flags

| Argument / Flag | Type | Required | Default | Description |
|-----------------|------|----------|---------|-------------|
| `key` | positional | Yes | -- | Dot-notation config path |
| `value` | positional | Yes | -- | Value to set. Parsed as JSON first; bare strings become JSON strings |
| `--tier` | string | No | `local` | Target tier: `local` or `custom`. Writing to `base` is rejected |
| `--json` | boolean | No | `false` | Wrap output in the standard `{ok, action, data}` envelope |

### Behavior

1. Locate `.wolfcastle/` directory. Exit 1 if not found.
2. Validate the key (reject empty segments, trailing dots, array indexing syntax).
3. Parse the value argument (JSON first, then bare string fallback).
4. Read the current tier overlay file. If absent, start with `{}`.
5. Walk the dot-notation path, creating intermediate maps as needed, and assign the value at the leaf.
6. Write the modified overlay back to the tier file.
7. Validate: call `config.Load()` to produce the merged config. If validation fails, restore the original tier file and report the error.
8. Output confirmation.

### Output

Human mode:

```
Set daemon.poll_interval_seconds = 10 in local/config.json
```

JSON mode:

```json
{
  "ok": true,
  "action": "config_set",
  "data": { "key": "daemon.poll_interval_seconds", "value": 10, "tier": "local" }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Success |
| 1 | No `.wolfcastle/` directory, invalid tier, malformed key, or validation failure |

### Examples

```bash
# Set a numeric value
wolfcastle config set logs.max_files 5

# Set a string value
wolfcastle config set identity.user alice

# Set a boolean
wolfcastle config set pipeline.enabled true

# Set a complex value
wolfcastle config set identity.tags '["a","b"]'

# Write to custom tier instead of local
wolfcastle config set logs.level warn --tier custom
```

### Error Cases

| Scenario | Behavior |
|----------|----------|
| `.wolfcastle/` not found | Exit 1: `fatal: not a wolfcastle project (no .wolfcastle/ found)` |
| `--tier base` | Exit 1: `error: cannot write to base tier (base is managed by the system)` |
| Invalid `--tier` value | Exit 1: `error: --tier must be "local" or "custom"` |
| Malformed key (empty segment) | Exit 1: `error: invalid key "daemon..poll": empty path segment` |
| Malformed key (trailing dot) | Exit 1: `error: invalid key "daemon.": trailing dot` |
| Malformed key (array indexing) | Exit 1: `error: invalid key "commands[0]": array indexing is not supported` |
| Validation failure after write | Exit 1: `error: validation failed, rolled back: {validation error}` |

---

## wolfcastle config append

### Synopsis

```
wolfcastle config append <key> <value> [--tier local|custom] [--json]
```

### Description

Appends a value to an array at the given config key. If the key does not exist, a new single-element array is created. If the key exists but is not an array, the command returns an error. The value is parsed with the same JSON-then-string rules as `config set`.

### Arguments and Flags

| Argument / Flag | Type | Required | Default | Description |
|-----------------|------|----------|---------|-------------|
| `key` | positional | Yes | -- | Dot-notation config path to an array |
| `value` | positional | Yes | -- | Value to append. Parsed as JSON first; bare strings become JSON strings |
| `--tier` | string | No | `local` | Target tier: `local` or `custom` |
| `--json` | boolean | No | `false` | Wrap output in the standard `{ok, action, data}` envelope |

### Behavior

1. Locate `.wolfcastle/` directory. Exit 1 if not found.
2. Validate the key.
3. Parse the value argument.
4. Read the current tier overlay file. If absent, start with `{}`.
5. Walk the dot-notation path to the leaf:
   - If the leaf is an array, append the parsed value.
   - If the leaf does not exist, create a single-element array.
   - If the leaf exists but is not an array, return an error.
6. Write the modified overlay back to the tier file.
7. Validate via `config.Load()`. Roll back on failure.
8. Output confirmation.

### Output

Human mode:

```
Appended "review" to pipeline.stage_order in custom/config.json
```

JSON mode:

```json
{
  "ok": true,
  "action": "config_append",
  "data": { "key": "pipeline.stage_order", "value": "review", "tier": "custom" }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Success |
| 1 | No `.wolfcastle/` directory, invalid tier, malformed key, non-array target, or validation failure |

### Examples

```bash
# Append a string to an array
wolfcastle config append identity.tags foo

# Append an object
wolfcastle config append pipeline.steps '{"name":"lint"}'

# Append to custom tier
wolfcastle config append identity.tags bar --tier custom
```

### Error Cases

| Scenario | Behavior |
|----------|----------|
| `.wolfcastle/` not found | Exit 1: `fatal: not a wolfcastle project (no .wolfcastle/ found)` |
| `--tier base` | Exit 1: `error: cannot write to base tier (base is managed by the system)` |
| Invalid `--tier` value | Exit 1: `error: --tier must be "local" or "custom"` |
| Existing value is not an array | Exit 1: `error: cannot append to "logs.max_files": existing value is not an array` |
| Validation failure after write | Exit 1: `error: validation failed, rolled back: {validation error}` |

---

## wolfcastle config remove

### Synopsis

```
wolfcastle config remove <key> <value> [--tier local|custom] [--json]
```

### Description

Removes the first matching value from an array at the given config key. Equality is determined by JSON serialization: both the candidate and each array element are marshaled to JSON and the resulting strings are compared. If the value is not found, the command exits with an error. If removal leaves an empty array, the empty array is written (use `config unset` to delete the key entirely).

### Arguments and Flags

| Argument / Flag | Type | Required | Default | Description |
|-----------------|------|----------|---------|-------------|
| `key` | positional | Yes | -- | Dot-notation config path to an array |
| `value` | positional | Yes | -- | Value to remove. Parsed as JSON first; bare strings become JSON strings |
| `--tier` | string | No | `local` | Target tier: `local` or `custom` |
| `--json` | boolean | No | `false` | Wrap output in the standard `{ok, action, data}` envelope |

### Behavior

1. Locate `.wolfcastle/` directory. Exit 1 if not found.
2. Validate the key.
3. Parse the value argument.
4. Read the current tier overlay file. If absent, start with `{}`.
5. Walk the dot-notation path to the leaf:
   - If the leaf is not an array, return an error.
   - Search for the value by JSON-string equality and remove the first match.
   - If the value is not found, return an error.
6. Write the modified overlay back to the tier file.
7. Validate via `config.Load()`. Roll back on failure.
8. Output confirmation.

### Output

Human mode:

```
Removed "review" from pipeline.stage_order in custom/config.json
```

JSON mode:

```json
{
  "ok": true,
  "action": "config_remove",
  "data": { "key": "pipeline.stage_order", "value": "review", "tier": "custom" }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Success |
| 1 | No `.wolfcastle/` directory, invalid tier, malformed key, non-array target, value not found, or validation failure |

### Examples

```bash
# Remove a string from an array
wolfcastle config remove identity.tags foo

# Remove a numeric value
wolfcastle config remove identity.tags 42

# Remove an object
wolfcastle config remove pipeline.steps '{"name":"lint"}'

# Remove from custom tier
wolfcastle config remove identity.tags bar --tier custom
```

### Error Cases

| Scenario | Behavior |
|----------|----------|
| `.wolfcastle/` not found | Exit 1: `fatal: not a wolfcastle project (no .wolfcastle/ found)` |
| `--tier base` | Exit 1: `error: cannot write to base tier (base is managed by the system)` |
| Invalid `--tier` value | Exit 1: `error: --tier must be "local" or "custom"` |
| Existing value is not an array | Exit 1: `error: cannot remove from "logs.max_files": existing value is not an array` |
| Value not found in array | Exit 1: `error: value not found in array at "pipeline.stage_order"` |
| Validation failure after write | Exit 1: `error: validation failed, rolled back: {validation error}` |

---

## wolfcastle config unset

### Synopsis

```
wolfcastle config unset <key> [--tier local|custom] [--json]
```

### Description

Removes a key from a tier overlay file. The key and any nested structure beneath it are deleted from the target tier's `config.json`. On the next `config.Load()`, the value reverts to whatever lower tiers or hardcoded defaults provide. If no lower tier sets the key, it disappears from the resolved config entirely. If the key does not exist in the tier file, the command succeeds silently.

### Arguments and Flags

| Argument / Flag | Type | Required | Default | Description |
|-----------------|------|----------|---------|-------------|
| `key` | positional | Yes | -- | Dot-notation config path to remove |
| `--tier` | string | No | `local` | Target tier: `local` or `custom` |
| `--json` | boolean | No | `false` | Wrap output in the standard `{ok, action, data}` envelope |

### Behavior

1. Locate `.wolfcastle/` directory. Exit 1 if not found.
2. Validate the key.
3. Read the current tier overlay file. If absent, start with `{}`.
4. Walk the dot-notation path and delete the key at the leaf. If the key does not exist, succeed silently.
5. Write the modified overlay back to the tier file.
6. Validate via `config.Load()`. Roll back on failure.
7. Output confirmation.

### Output

Human mode:

```
Unset pipeline.stages.audit.enabled from local/config.json
```

JSON mode:

```json
{
  "ok": true,
  "action": "config_unset",
  "data": { "key": "pipeline.stages.audit.enabled", "tier": "local" }
}
```

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Success (including when the key did not exist) |
| 1 | No `.wolfcastle/` directory, invalid tier, malformed key, or validation failure |

### Examples

```bash
# Unset a key from local tier
wolfcastle config unset logs.level

# Unset from custom tier
wolfcastle config unset pipeline.timeout --tier custom
```

### Error Cases

| Scenario | Behavior |
|----------|----------|
| `.wolfcastle/` not found | Exit 1: `fatal: not a wolfcastle project (no .wolfcastle/ found)` |
| `--tier base` | Exit 1: `error: cannot write to base tier (base is managed by the system)` |
| Invalid `--tier` value | Exit 1: `error: --tier must be "local" or "custom"` |
| Validation failure after write | Exit 1: `error: validation failed, rolled back: {validation error}` |

---

## wolfcastle doctor

### Synopsis

```
wolfcastle doctor [--fix] [--json]
```

### Description

Validates the structural integrity of the Wolfcastle project and offers to fix issues. The doctor runs 20+ categories of checks against the distributed state files (ADR-024), identifying orphaned files, missing index entries, stale states, missing audit tasks, and other structural inconsistencies.

A subset of these checks also runs automatically on daemon startup (ADR-025). The `doctor` command runs the full suite and provides interactive repair.

This is a user-facing command. The underlying validation engine is reusable infrastructure shared with daemon startup checks and potentially CI pipelines.

### Arguments and Flags

| Flag | Type | Required | Default | Description |
|------|------|----------|---------|-------------|
| `--fix` | boolean | No | `false` | Skip the confirmation prompt and apply all fixes immediately |
| `--json` | boolean | No | `false` | Output as structured JSON instead of human-readable text |

### Behavior

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Resolve engineer identity from `local/config.json`.
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

The `prompt_file` is relative to `.wolfcastle/system/base/` and contains the system prompt for model-assisted fixes. The model receives the issue's location context (node path, file path, field, surrounding state) and proposes a resolution that Go code validates before applying.

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

Installs a Claude Code skill that enables CC users to interact with Wolfcastle natively from their conversation. The skill files are sourced from `.wolfcastle/system/base/skills/` (generated by `wolfcastle init` or `wolfcastle update`).

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `<target>` | string (positional) | Yes | -- | The integration target to install. Currently only `skill` is supported |

### Behavior (`wolfcastle install skill`)

1. Locate `.wolfcastle/` directory. Fail if not found.
2. Verify that `.wolfcastle/system/base/skills/` exists and contains skill definition files. If not, fail (suggest running `wolfcastle update` to regenerate `base/`).
3. **Detect symlink support** on the current OS:
   - Attempt to create a test symlink in a temporary directory.
   - If successful: symlinks are supported.
   - If failed (e.g., Windows without developer mode): fall back to copy mode.
4. **Create the target directory**: ensure `{project_root}/.claude/` exists. Create it if it does not.
5. **Install the skill**:
   - **Symlink mode** (preferred): Create `.claude/wolfcastle/` as a symlink pointing to `.wolfcastle/system/base/skills/`. If `.claude/wolfcastle/` already exists:
     - If it is already a symlink to the correct target: print "already installed" and exit 0.
     - If it is a symlink to a different target or a regular directory: remove it and create the correct symlink.
   - **Copy mode** (fallback): Copy the contents of `.wolfcastle/system/base/skills/` to `.claude/wolfcastle/`. If `.claude/wolfcastle/` already exists, overwrite its contents.
6. Output the result.

### Output

Symlink mode:

```
Installed Claude Code skill (symlink)
  .claude/wolfcastle/ -> .wolfcastle/system/base/skills/
Skill will auto-update when you run 'wolfcastle update'.
```

Copy mode:

```
Installed Claude Code skill (copy)
  .claude/wolfcastle/ <- .wolfcastle/system/base/skills/
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
| 3 | Source skill files not found in `.wolfcastle/system/base/skills/` |
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
| No skill source | `fatal: skill files not found in .wolfcastle/system/base/skills/. Run 'wolfcastle update' to regenerate base/.` | 3 |
| Permission denied | `fatal: cannot write to .claude/: permission denied` | 4 |

### Extensibility

The `install` subcommand is designed to accept additional targets in the future (e.g., git hooks, editor plugins). Each target follows the same pattern: source files live in `.wolfcastle/system/base/`, and `install` places them outside `.wolfcastle/` with the user's explicit consent.

---

## wolfcastle completion

### Synopsis

```
wolfcastle completion <shell>
```

### Description

Generates shell completion scripts for wolfcastle. This is Cobra's built-in completion generator, providing tab-completion for commands, subcommands, and flags. Each shell has its own subcommand that outputs the appropriate script format.

### Subcommands

| Subcommand | Description |
|------------|-------------|
| `bash` | Generate the autocompletion script for bash |
| `fish` | Generate the autocompletion script for fish |
| `powershell` | Generate the autocompletion script for powershell |
| `zsh` | Generate the autocompletion script for zsh |

### Behavior

1. The specified shell subcommand writes a completion script to stdout.
2. The user sources or installs this script according to their shell's completion mechanism.
3. No `.wolfcastle/` directory or identity is required. This command works anywhere.

### Output

A shell-specific completion script written to stdout. The exact format varies by shell. Each subcommand's `--help` output includes installation instructions for that shell.

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Completion script generated |
| 1 | Invalid shell or generation error |

### Examples

```bash
# Generate and source bash completions
source <(wolfcastle completion bash)

# Generate zsh completions and install them
wolfcastle completion zsh > "${fpath[1]}/_wolfcastle"

# Generate fish completions
wolfcastle completion fish | source

# Generate powershell completions
wolfcastle completion powershell | Out-String | Invoke-Expression
```

---

## wolfcastle help

### Synopsis

```
wolfcastle help [command]
```

### Description

Displays help information for any wolfcastle command. This is Cobra's built-in help command. When called without arguments, it shows the top-level help (equivalent to `wolfcastle --help`). When given a command path, it shows help for that specific command.

### Arguments and Flags

| Flag/Arg | Type | Required | Default | Description |
|----------|------|----------|---------|-------------|
| `[command]` | string (positional) | No | -- | Command path to get help for (e.g., `task add`, `audit breadcrumb`) |

### Behavior

1. If no command argument is given, display the root help text listing all available commands and groups.
2. If a command path is given, resolve it through the command tree and display that command's help text (long description, usage, flags, and examples).
3. No `.wolfcastle/` directory or identity is required. This command works anywhere.

### Output

Human-readable help text written to stdout, including the command's description, usage pattern, available flags, and subcommands (if any).

### Exit Codes

| Code | Condition |
|------|-----------|
| 0 | Help displayed |
| 1 | Unknown command |

### Examples

```bash
# Show top-level help
wolfcastle help

# Show help for a specific command
wolfcastle help task add

# Show help for a command group
wolfcastle help audit
```

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
