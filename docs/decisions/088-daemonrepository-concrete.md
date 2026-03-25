# DaemonRepository uses concrete struct with explicit parameters

## Status
Accepted (partially superseded by ADR-094 for the validate-daemon boundary)

## Date
2026-03-18

## Context
The daemon package had free functions (WritePID, ReadPID, RemovePID) that each independently constructed filesystem paths from a wolfcastle directory string. Stop file operations lived in daemon.go with the same scattered path construction. This made the daemon's filesystem footprint implicit and spread across files.

## Options Considered
1. Define an interface (DaemonStore) and a concrete implementation, allowing test doubles
2. Concrete struct only, tested against real temp directories
3. Keep free functions, consolidate path logic into a shared helper

## Decision
Concrete struct (DaemonRepository) with no interface. Methods accept explicit parameters rather than reaching for process globals (e.g., WritePID takes a pid int instead of calling os.Getpid() internally).

Explicit parameters make the struct testable without process-level side effects. An interface is unnecessary because there is exactly one implementation and the struct already works with temp directories in tests; a test double would add indirection without improving confidence.

## Consequences
Callers that need to mock daemon filesystem operations must use temp directories instead of fake implementations. This is the same pattern used by StateStore and tierfs.Resolver throughout the codebase.
