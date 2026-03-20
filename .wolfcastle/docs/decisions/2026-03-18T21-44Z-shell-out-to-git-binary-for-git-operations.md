# Shell out to git binary for git operations

## Status
Accepted

## Date
2026-03-18

## Status
Accepted

## Context
The git package needs to perform repository operations (branch detection, dirty checks, worktree management). Two approaches were considered.

## Options Considered
1. Use the go-git library for pure-Go git operations
2. Shell out to the system git binary via os/exec

## Decision
Shell out to the system git binary. Each Service method constructs an exec.Command with cmd.Dir set to the repository root.

## Consequences
The package depends on git being installed on the host, which is already a hard requirement for wolfcastle's worktree-based execution model. Operations are slightly slower than in-process calls but avoid a large dependency tree and match the exact behavior of the git version the user has installed. Error messages from git are human-readable without translation.
