# ADR-006: Configurable Pipelines

## Status
Accepted

## Date
2026-03-12

## Context
Ralph had a hardcoded three-stage pipeline: Haiku expands inbox → Sonnet files into tree → Opus executes tasks. This worked well but is inflexible. Different projects may need different preprocessing, or no preprocessing at all.

## Decision
Wolfcastle generalizes the pipeline concept. A pipeline is a sequence of stages defined in config, where each stage specifies a role, model/provider, and prompt. The Ralph-style expand→file→execute flow ships as the default pipeline, but users can define custom pipelines or simplify to a single-stage executor.

## Consequences
- The inbox expansion and filing steps become optional, configurable stages
- Users can add preprocessing stages (e.g. linting, dependency checking) or remove stages they don't need
- Each stage can use a different model tier appropriate to its complexity
- The daemon loop iterates through pipeline stages generically rather than hardcoding specific steps
