# Use concrete Identity struct with constructor validation

## Status
Accepted

## Date
2026-03-18

## Status
Accepted

## Context
The codebase had namespace resolution scattered across tree.ResolveNamespace and ad-hoc detectIdentity calls. Consolidating into a single Identity domain type required choosing between an interface and a concrete struct, and deciding where validation lives.

## Options Considered
1. **Interface with multiple implementations** (e.g., ConfigIdentity, DetectedIdentity). Offers testability seams but adds indirection for a type with no polymorphic behavior.
2. **Concrete struct with constructor validation.** IdentityFromConfig validates at construction time; DetectIdentity uses OS fallbacks. Callers receive a fully valid *Identity or an error.
3. **Unvalidated struct with method-level checks.** Defers validation to point of use, scattering nil/empty checks throughout callers.

## Decision
Option 2: concrete struct with two constructors. IdentityFromConfig enforces that User and Machine are non-empty, returning an error otherwise. DetectIdentity always succeeds by falling back to "unknown" when OS calls fail. Both constructors compute Namespace as User + "-" + Machine, keeping the derivation in one place.

## Consequences
No interface exists for Identity, so test code that needs a specific identity constructs the struct directly. This is acceptable because Identity carries no I/O or side effects beyond its constructors. If a second implementation becomes necessary (e.g., identity from environment variables), an interface can be extracted at that point.
