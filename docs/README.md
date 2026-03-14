# Wolfcastle Documentation

Documentation is organized by audience. Each directory serves a different reader with different needs.

## [For Humans](humans/)

The user-facing field manual. How the system works, how to configure it, how to recover from failures, how audits verify work, how multiple engineers collaborate, and a complete CLI reference with individual pages for every command.

Start here if you want to use Wolfcastle.

## [For Agents](agents/)

Technical guides for AI agents (and human developers) working on the codebase. Architecture, code standards, command patterns, daemon internals, state management, documentation conventions, and the Wolfcastle voice guide.

Start here if you want to modify Wolfcastle.

## [Architecture Decisions](decisions/)

61 ADRs documenting every major design choice. Each records the context, decision, and consequences so future contributors understand why the system is built the way it is. ADRs are the authoritative source; specs defer to them on conflict.

## [Specifications](specs/)

14 living specs that describe the current system in detail: state machine, config schema, tree addressing, pipeline contracts, audit propagation, archive format, CLI commands, validation engine, CI/CD, testing, and more. Specs track implementation, not aspirations.
