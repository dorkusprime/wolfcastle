# ADR-021: CLI Command Surface

## Status
Accepted

## Date
2026-03-12

## Context
Wolfcastle needs a complete, consistent set of CLI commands that serve three audiences: the user (interactive control), the daemon (orchestration), and the executing model (state mutations via script calls). Commands were defined incrementally across multiple ADRs and need to be consolidated into a single reference.

## Decision

### Lifecycle Commands (user-facing)

| Command | Description |
|---------|-------------|
| `wolfcastle init` | Scaffold `.wolfcastle/` directory, auto-populate identity in `local/config.json` |
| `wolfcastle start [--node <path>] [--worktree <branch>] [-d]` | Run the daemon. Optional subtree scoping, worktree isolation, background mode |
| `wolfcastle stop [--force]` | Graceful stop (finish iteration) or hard kill via PID |
| `wolfcastle status` | Show current tree state, progress, active task |
| `wolfcastle follow` | Tail model output in real time (works in both foreground and background mode) |
| `wolfcastle update` | Update the Wolfcastle binary and regenerate `base/` |

### Task Commands (model and user)

| Command | Description |
|---------|-------------|
| `wolfcastle task add --node <path> "description"` | Add a task to a leaf node |
| `wolfcastle task claim --node <path>` | Mark a task as In Progress |
| `wolfcastle task complete --node <path>` | Mark a task as Complete |
| `wolfcastle task block --node <path> "reason"` | Mark a task as Blocked with explanation |
| `wolfcastle task unblock --node <path>` | Clear Blocked status, reset failure counter |

### Project Commands (model and user)

| Command | Description |
|---------|-------------|
| `wolfcastle project create --node <parent> "name"` | Create a new project or sub-project node |

### Navigation (daemon-internal, also callable by model)

| Command | Description |
|---------|-------------|
| `wolfcastle navigate [--node <path>]` | Depth-first traversal to find active leaf. Optional subtree scoping |

### Documentation Commands (model and user)

| Command | Description |
|---------|-------------|
| `wolfcastle adr create "title" [--stdin \| --file <path>]` | Create a new ADR. Body via stdin or file for lengthy content |

### Audit Commands (model-facing)

| Command | Description |
|---------|-------------|
| `wolfcastle audit breadcrumb --node <path> "text"` | Append a breadcrumb to a node's audit trail |
| `wolfcastle audit escalate --node <path> "gap"` | Escalate a gap to the parent node's audit |

### Archive Commands (daemon-internal)

| Command | Description |
|---------|-------------|
| `wolfcastle archive add --node <path>` | Generate archive entry from node's completed state |

### Spec Commands (model and user)

| Command | Description |
|---------|-------------|
| `wolfcastle spec create --node <path> "title"` | Create a new spec in `docs/specs/` and optionally link it to a node |
| `wolfcastle spec link --node <path> <filename>` | Link an existing spec to a node |
| `wolfcastle spec list [--node <path>]` | List all specs, or specs linked to a specific node |

### Inbox Commands (user-facing)

| Command | Description |
|---------|-------------|
| `wolfcastle inbox add "idea"` | Add an item to the inbox (alternative to editing the inbox file directly) |
| `wolfcastle inbox list` | Show all inbox items with status and timestamp |
| `wolfcastle inbox clear [--all]` | Remove filed/expanded items from inbox; `--all` removes everything |

## Conventions

- **Tree addressing**: All `--node` flags accept a path from the tree root (e.g. `attunement-tree/fire-impl/task-3`)
- **Audience**: Commands are usable by anyone, but designed with a primary audience in mind (user, model, or daemon)
- **Output**: Commands that the model calls should return structured output (JSON) so the model can parse results reliably
- **Errors**: Commands return non-zero exit codes on failure with a descriptive message

## Consequences
- Single, consolidated command surface for all Wolfcastle operations
- Model interacts with state exclusively through these commands (per ADR-003)
- The command reference is auto-generated into the system prompt (per ADR-017)
- All state-mutating commands are deterministic and testable
- The surface is intentionally minimal: new commands can be added as needs arise during implementation
- Additional commands added in later ADRs: `wolfcastle doctor` (ADR-025), `wolfcastle install <target>` (ADR-026), `wolfcastle unblock` (ADR-028), `wolfcastle spec create/link/list` (ADR-031). See the CLI commands spec for the complete reference.

## Amendment (2026-03-23)

**ADR-073** renamed `wolfcastle follow` to `wolfcastle log`. The old name is preserved as a hidden alias. The lifecycle commands table above reflects the original design; the current command name is `wolfcastle log`.
