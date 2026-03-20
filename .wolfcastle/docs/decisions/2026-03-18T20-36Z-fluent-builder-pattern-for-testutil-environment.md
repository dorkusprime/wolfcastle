# Fluent builder pattern for testutil.Environment

## Status
Accepted

## Date
2026-03-18

## Status
Accepted

## Context
The testutil package needed a structured way to construct .wolfcastle/ directories for tests. The existing helpers (SetupWolfcastle, SetupTree) use imperative function calls that return raw paths and require callers to manage state files directly. As the domain-repository migration proceeds, tests will need richer setup: config overrides, project trees, prompt files, and rule fragments, all composed together.

## Options Considered
1. Extend existing SetupWolfcastle/SetupTree with more parameters
2. Fluent builder pattern (NewEnvironment().WithConfig().WithProject().WithPrompt())
3. Option-func pattern (NewEnvironment(WithConfig(...), WithProject(...)))

## Decision
Fluent builder pattern. Each With* method mutates the environment in place and returns the receiver for chaining. The Environment struct holds a *testing.T so that setup errors can t.Fatalf immediately rather than requiring error-return plumbing in every test.

## Consequences
- Test setup reads as a declarative description of the desired state
- New With* methods can be added as repository packages are built, without breaking existing callers
- Storing *testing.T in the struct means Environment cannot be shared across tests or goroutines, which is the correct constraint for test fixtures
- The older SetupWolfcastle/SetupTree helpers remain for existing callers; they will be migrated incrementally
