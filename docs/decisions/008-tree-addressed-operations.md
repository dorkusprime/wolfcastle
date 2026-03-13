# ADR-008: Tree-Addressed Operations

## Status
Accepted

## Date
2026-03-12

## Context
Wolfcastle uses an n-tier tree of projects, sub-projects, and leaf tasks. Scripts that mutate state need to target specific nodes. Ralph navigated by filesystem path since each node was a directory with its own STATUS.md.

## Decision
All script operations accept a tree address (path from root to target node) to identify which node to operate on. For example: `wolfcastle task add --node attunement-tree/fire-implementation "Wire stamina cost"`. Scripts for creating sub-projects, navigating, claiming tasks, and escalating audit gaps all use this addressing scheme.

## Consequences
- Every node in the tree is unambiguously addressable
- Scripts can validate that the target node exists and is the correct type (orchestrator vs leaf)
- The discovery-first pattern works by the model calling `wolfcastle task add` with the appropriate node address during discovery tasks
- Tree operations compose naturally with the navigation logic
