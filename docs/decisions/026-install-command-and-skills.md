# ADR-026: wolfcastle install Command and Claude Code Skill

## Status
Accepted

## Date
2026-03-13

## Context
Users of Claude Code should be able to interact with Wolfcastle natively from their CC session. This requires a Claude Code skill installed in the project. Additionally, future integrations with other tools may need similar installation steps. Wolfcastle should not proactively manage files outside `.wolfcastle/`, but should provide a way to opt-in to integrations.

## Decision

### wolfcastle install <target>
An extensible installation command for integrations. Currently supports one target:

- `wolfcastle install skill` — installs a Claude Code skill for interacting with Wolfcastle

### Claude Code Skill Installation
`wolfcastle install skill`:
1. Detects symlink support on the current OS
2. If symlinks supported: creates `project_root/.claude/wolfcastle/` as a symlink to the skill definition in `.wolfcastle/system/base/skills/`
3. If no symlink support: copies the skill files to `.claude/wolfcastle/`
4. The skill enables CC users to run Wolfcastle commands natively from conversation

### Symlink vs Copy
Symlinks are preferred because `wolfcastle update` regenerates `base/`, and the symlink means the skill automatically gets updates. With copies, the user would need to re-run `wolfcastle install skill` after updates.

### Extensibility
The `install` subcommand is designed to accept other targets in the future. No current plans beyond `skill`, but the command structure supports it.

### Wolfcastle Does Not Touch Files Outside .wolfcastle/ By Default
This is an explicit opt-in action. Wolfcastle never creates files in the project root, `.claude/`, or anywhere else outside `.wolfcastle/` unless the user runs an `install` command.

## Consequences
- CC users get native Wolfcastle interaction via slash commands
- Symlink approach means skills stay current with Wolfcastle updates
- Fallback to copy ensures cross-platform compatibility
- The install pattern is reusable for future integrations (hooks, editor plugins, etc.)
- Users maintain full control over what goes outside `.wolfcastle/`
