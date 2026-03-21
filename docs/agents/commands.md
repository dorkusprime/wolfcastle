# CLI Commands

## Command Registration Pattern

All subcommand groups follow the same pattern:

1. A `register.go` file in the subpackage exports a `Register(app *cmdutil.App, parent *cobra.Command)` function
2. Individual command files export `newXxxCmd(app *cmdutil.App) *cobra.Command`
3. `Register()` calls `newXxxCmd()` for each command and adds them to the parent

Example:
```go
// cmd/task/register.go
func Register(app *cmdutil.App, parent *cobra.Command) {
    taskCmd := &cobra.Command{Use: "task", Short: "..."}
    taskCmd.AddCommand(newAddCmd(app))
    taskCmd.AddCommand(newClaimCmd(app))
    parent.AddCommand(taskCmd)
}
```

Root-level commands (not in subpackages) use `init()` to register with `rootCmd` directly.

## Command Checklist

When adding a new command:

- [ ] Follow the registration pattern above
- [ ] Add `--json` support (check `app.JSONOutput`)
- [ ] Use `output.PrintHuman()` / `output.Print()` for all output
- [ ] Call `app.RequireResolver()` early if the command needs the tree
- [ ] Add shell completion via `cmdutil.CompleteNodeAddresses()` or `CompleteTaskAddresses()`
- [ ] Include `Long` help text with examples
- [ ] Add the command to `cmd/completions.go` if it has completable flags
- [ ] Update `internal/pipeline/scriptref.go` to include the new command in the script reference

## Flag Conventions

- Required flags: use `cmd.MarkFlagRequired("name")`. Cobra enforces this
- `--node`: tree address (node path or node/task-id), validated via `tree.ParseAddress()`
- `--all`: batch operations (approve all, reject all)
- `--json`: global flag on root, available everywhere via `app.JSONOutput`

## Config-Skip Commands

These commands skip config loading in `PersistentPreRunE`: `init`, `version`, `help`. If you add a command that must work without `.wolfcastle/`, add it to the switch statement in `cmd/root.go:37`.

## Command Reference

### Standalone Commands

| Command | Purpose |
|---------|---------|
| `init` | Initialize a Wolfcastle workspace |
| `version` | Display version information |
| `update` | Update the CLI |
| `navigate` | Navigate to a node directory |
| `doctor` | Diagnose and repair broken state |
| `unblock` | Unblock a blocked task interactively |
| `install` | Install extra tools (tree, jq, etc.) |

### Daemon Commands

Registered at root level (not grouped under a parent):

| Command | Purpose |
|---------|---------|
| `start` | Start the execution daemon |
| `stop` | Stop the daemon |
| `log` | Stream daemon logs |
| `status` | Show work status, optionally scoped to a node |

### archive

| Subcommand | Purpose |
|------------|---------|
| `add` | Archive a completed root-level node |
| `restore` | Restore an archived node to active state |
| `delete` | Permanently delete an archived node |

### adr

| Subcommand | Purpose |
|------------|---------|
| `create` | Create an architecture decision record (supports `--stdin`, `--file`) |

### audit

| Subcommand | Purpose |
|------------|---------|
| `aar` | Record an After Action Review for a completed task |
| `breadcrumb` | Add a breadcrumb entry to audit trail |
| `enrich` | Enrich audit findings with additional information |
| `escalate` | Escalate an issue in the audit trail |
| `gap` | Record a gap in the audit trail |
| `fix-gap` | Mark an audit gap as fixed |
| `history` | Review past audit decisions |
| `list` | List available audit scopes |
| `pending` | List findings awaiting judgment |
| `approve` | Approve a finding (promote to project) |
| `reject` | Dismiss a finding |
| `report` | Generate an audit report |
| `resolve` | Resolve an escalation |
| `run` | Run a codebase audit with a specific scope |
| `scope` | Set structured audit scope on a node |
| `show` | Display audit information for a node |
| `summary` | Set the result summary on a node's audit record |

### config

| Subcommand | Purpose |
|------------|---------|
| `show` | Display configuration, optionally filtered by tier or section |
| `set` | Set a configuration value in the local tier |
| `unset` | Remove a configuration value from the local tier |
| `append` | Append to a configuration list value |
| `remove` | Remove an item from a configuration list |

### inbox

| Subcommand | Purpose |
|------------|---------|
| `add` | Capture an idea or task into the inbox |
| `list` | Review inbox items |
| `clear` | Clear inbox items |

### orchestrator

| Subcommand | Purpose |
|------------|---------|
| `criteria` | Set or manage success criteria for an orchestrator node |

### project

| Subcommand | Purpose |
|------------|---------|
| `create` | Create a new project in the work tree |

### spec

| Subcommand | Purpose |
|------------|---------|
| `create` | Create a new spec document, optionally linked to a node |
| `link` | Link an existing spec file to a node |
| `list` | List specs, optionally filtered by node |

### task

| Subcommand | Purpose |
|------------|---------|
| `add` | Add a task to a leaf node |
| `amend` | Amend task metadata |
| `block` | Block a task with a reason |
| `claim` | Claim ownership of a task |
| `complete` | Mark a task as complete |
| `deliverable` | Manage task deliverables |
| `unblock` | Unblock a blocked task |
