# ADR-004: Model-Agnostic Design

## Status
Accepted

## Date
2026-03-12

## Context
Ralph was hardcoded to Claude models (Haiku for expansion, Sonnet for filing, Opus for execution). Wolfcastle aims to be a general-purpose orchestration system.

## Decision
Wolfcastle is model-agnostic. The JSON config specifies which model and provider to use per pipeline role. Authentication is handled externally: secrets live in `.env` files or are managed by model-specific CLI auth mechanisms (e.g. `claude` CLI login, `OPENAI_API_KEY`, etc.).

Wolfcastle itself never stores or manages API keys.

## Consequences
- Config must define a provider + model per pipeline stage
- The daemon loop must support multiple invocation backends (Claude Code CLI, OpenAI API, etc.)
- Prompt files should avoid provider-specific assumptions where possible
- Auth is the user's responsibility; Wolfcastle documents requirements per provider
