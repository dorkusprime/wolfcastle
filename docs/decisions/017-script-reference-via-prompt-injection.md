# ADR-017: Script Reference via Prompt Injection

## Status
Accepted

## Date
2026-03-12

## Context
The executing model needs to know what Wolfcastle commands are available and how to call them (arguments, tree addressing, expected behavior). This reference must stay in sync with the actual command implementations — drift between docs and reality would cause the model to call nonexistent or misformatted commands.

## Decision

### Generated from Source
The script reference is a prompt fragment generated from Wolfcastle's own command definitions in the Go code. When commands change, the prompt fragment changes automatically. There is no separately maintained reference doc.

### Injected via System Prompt
Wolfcastle assembles the script reference into the system prompt sent to the model each iteration, alongside the orchestrator prompt and composable rule fragments (per ADR-005). The model doesn't discover commands at runtime — it's told what it can call before it starts working.

### Part of `base/`
The generated prompt fragment lives in `base/` and is regenerated on `wolfcastle init` and `wolfcastle update`. Since `base/` is gitignored and Wolfcastle-managed (ADR-009), updates are automatic and non-disruptive.

## Consequences
- Zero risk of prompt/implementation drift — single source of truth in Go code
- No separate reference doc to maintain
- Models always have an accurate, complete command reference in context
- Adding a new command to Wolfcastle automatically makes it available to models after `wolfcastle update`
- Prompt fragment size grows with the command surface — worth monitoring for context budget
