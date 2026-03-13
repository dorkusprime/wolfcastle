# ADR-003: Deterministic Scripts with Static Documentation

## Status
Accepted

## Date
2026-03-12

## Context
In Ralph, the model was responsible for understanding and correctly mutating state files. This coupled execution correctness to prompt quality. We want state operations to be reliable regardless of model capability.

## Decision
All state mutations happen through deterministic scripts. A static Markdown file documents the available scripts and their usage so the model knows what to call. This documentation is never modified by Wolfcastle or by the model — it is a fixed reference.

The model's job is to decide *what* to do (claim a task, mark complete, add a subtask). The scripts' job is to do it *correctly*.

## Consequences
- Models interact with Wolfcastle state exclusively through script invocations
- Scripts validate inputs, enforce invariants (e.g. audit task always last), and handle tree addressing
- Static docs become part of the system prompt so models know available operations
- Testing scripts is straightforward — they're deterministic functions over JSON
