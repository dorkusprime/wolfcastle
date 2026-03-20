# Use interfaces in ScaffoldService to break import cycles

## Status
Accepted

## Date
2026-03-18

## Status
Accepted

## Context
ScaffoldService in the project package depends on pipeline.PromptRepository (for WriteAllBase) and daemon.DaemonRepository (for future daemon operations). The pipeline package's tests import project (for WriteBasePrompts fixture setup), and daemon imports pipeline. This creates transitive import cycles: pipeline_test -> project -> pipeline, and pipeline_test -> project -> daemon -> pipeline.

## Options Considered
1. Move ScaffoldService to a separate package (e.g., internal/scaffold) that sits above both project and pipeline
2. Use interfaces in project to break the dependency on concrete types
3. Move the pipeline test helper out of project into a test-only package

## Decision
Define a promptWriter interface in project with a single WriteAllBase method that pipeline.PromptRepository satisfies implicitly. Accept daemon as `any` since ScaffoldService stores but does not yet call any daemon methods. This keeps ScaffoldService co-located with the other scaffold functions it replaces while breaking both cycles at the point of use.

## Consequences
Callers constructing ScaffoldService pass concrete *pipeline.PromptRepository and *daemon.DaemonRepository values, which satisfy the interface and any respectively. When ScaffoldService eventually needs daemon methods, we either define a second interface or, if the pipeline_test -> project cycle is resolved by then, revert to the concrete type.
