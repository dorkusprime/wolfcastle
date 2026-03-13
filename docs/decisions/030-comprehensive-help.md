# ADR-030: Comprehensive Help at Every CLI Level

## Status
Accepted

## Date
2026-03-13

## Context
Wolfcastle has a growing command surface (20+ commands) with subcommands, flags, and discoverable features like audit scopes. Users and models need reliable, complete help text at every level.

## Decision

### Help at Every Level
Every command and subcommand supports `-h` / `--help`:

```
wolfcastle -h                    # top-level overview of all commands
wolfcastle task -h               # task subcommand overview
wolfcastle task add -h           # specific command help
wolfcastle audit -h              # includes dynamically discovered scopes
wolfcastle install -h            # includes available install targets
```

### Dynamic Help Content
Help text for commands with discoverable features (like `wolfcastle audit`) includes dynamically generated content. `wolfcastle audit -h` lists all available scopes found across `base/audits/`, `custom/audits/`, and `local/audits/`.

### Top-Level Help Structure
`wolfcastle -h` groups commands by category:

- **Lifecycle**: init, start, stop, status, follow, update
- **Task**: task add/claim/complete/block/unblock
- **Project**: project create
- **Audit**: audit (codebase), audit breadcrumb, audit escalate
- **Documentation**: adr create
- **Archive**: archive add
- **Inbox**: inbox add
- **Navigation**: navigate
- **Diagnostics**: doctor, unblock
- **Integration**: install

### Implementation
Go's cobra framework handles this natively with command grouping and dynamic help generation. Scope/target discovery hooks into cobra's help template system.

## Consequences
- Users can always find what they need without consulting docs
- Models can use `-h` to discover available commands and flags
- Dynamic help stays current as scopes and targets are added
- Consistent help format across all commands
