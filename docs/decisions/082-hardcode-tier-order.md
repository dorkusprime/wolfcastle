# Hardcode tier order in tierfs

## Status
Accepted

## Date
2026-03-18

## Status
Accepted

## Context
The three-tier override hierarchy (base < custom < local) is fundamental to Wolfcastle's configuration and prompt system. ADR-063 established this pattern. The tierfs package needs to define resolution order.

## Options Considered
1. Accept tier names/order as constructor parameters
2. Hardcode tier order as a package-level constant

## Decision
Hardcode the tier list as a package-level var: `["base", "custom", "local"]`. The tierfs package is the single source of truth for tier names and resolution order, as specified by ADR-063. Making this configurable would scatter a core invariant across call sites.

## Consequences
Adding or reordering tiers requires changing tierfs source code. This is intentional: tier structure is a system-wide invariant, not a per-caller concern. All consumers inherit the same resolution order automatically.
