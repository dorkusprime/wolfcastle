# Use interface for tierfs.Resolver

## Status
Accepted

## Date
2026-03-18

## Status
Accepted

## Context
The domain-repository-architecture spec calls for concrete types throughout, reserving interfaces for testability seams. The tierfs package provides tier resolution that multiple repositories (ConfigRepository, PromptRepository, ClassRepository) will depend on.

## Options Considered
1. Concrete FS type only, no interface
2. Resolver interface with FS implementation

## Decision
Define a Resolver interface alongside the concrete FS implementation. Repositories accept Resolver in their constructors, enabling tests to substitute in-memory implementations without touching the filesystem. This is one of the few places the architecture explicitly calls for an interface because it sits at a dependency boundary shared by many consumers.

## Consequences
Repositories can be tested with lightweight stubs. The interface is small (five methods) and unlikely to grow, keeping the seam narrow. The concrete FS type satisfies Resolver and remains the only production implementation.
