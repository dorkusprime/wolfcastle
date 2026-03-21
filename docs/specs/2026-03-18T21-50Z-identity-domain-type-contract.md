# Identity Domain Type Contract

## Overview

The `Identity` type in `internal/config` is the domain representation of a resolved user+machine pair. It replaces scattered namespace resolution (previously in `tree.ResolveNamespace` and ad-hoc OS detection calls) with a single authoritative type.

## Type

```go
type Identity struct {
    User      string
    Machine   string
    Namespace string // always User + "-" + Machine
}
```

## Constructors

### IdentityFromConfig(cfg *Config) (*Identity, error)

Extracts identity from a loaded Config. Returns an error if:
- `cfg.Identity` is nil ("identity not configured")
- `cfg.Identity.User` is empty ("identity.user must be set")
- `cfg.Identity.Machine` is empty ("identity.machine must be set")

On success, returns a fully populated `*Identity` with Namespace derived as `User + "-" + Machine`.

### DetectIdentity() *Identity

Reads username and hostname from the OS. Always succeeds: falls back to `"unknown"` for either field if the corresponding system call fails. Strips the DNS suffix from hostname (everything after the first `.`) and lowercases the result.

## Methods

### ProjectsDir(wolfcastleRoot string) string

Returns the path `{wolfcastleRoot}/system/projects/{Namespace}`. This is where per-identity project state is stored.

## Invariants

- Namespace is always `User + "-" + Machine`. It is computed at construction time and never recomputed.
- An `*Identity` returned by `IdentityFromConfig` always has non-empty User and Machine.
- An `*Identity` returned by `DetectIdentity` always has non-empty User and Machine (at minimum `"unknown"`).

## Thread Safety

Identity is a plain value struct with no mutable state after construction. Safe for concurrent read access. Not intended to be mutated after creation.
