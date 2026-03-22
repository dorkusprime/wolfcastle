# Coding

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Design

**Tests first, then clarity, then deduplication, then simplicity.** In that priority order. Code that passes tests but is unreadable needs rewriting. Code that's readable but duplicated needs consolidating. Code that's consolidated but over-abstracted needs simplifying. When two of these conflict, favor the one higher on the list.

**One reason to change.** Each file, class, or function should be responsible for one concern. A function that fetches data from a database, transforms it, and formats it for display has three concerns. When concerns are mixed, a change to the display format risks breaking the database query. When they're separated, each piece can be understood, tested, and modified independently.

**Build in self-contained pieces.** Code should be organized into modules, packages, or components that can be understood in isolation. Each piece has a clear boundary: it defines what it offers (its public interface) and hides how it works internally. A well-modularized codebase lets you change the internals of one piece without affecting others, test each piece independently, and replace a piece entirely as long as the interface stays the same. When you add new functionality, it should fit into an existing module or become a new one. It should not spread across several unrelated modules.

**Minimize what each piece knows about other pieces.** A function should work with the data it receives directly, not reach through chains of objects to find what it needs. If function A calls B, and B returns an object, A should not then dig into that object's internals to call methods on its sub-objects. Instead, B should provide what A needs directly. This keeps pieces independent: changing one doesn't cascade into changes across the codebase.

## Build and Test

Look for the project's build system (Makefile, go.mod, package.json, Cargo.toml, pyproject.toml, CMakeLists.txt, or equivalent) and run whatever commands it provides.

1. **Build**: run the project's build command. Fix all errors before proceeding.
2. **Test**: run the full test suite. Fix all failures, including tests you didn't write. Do not skip, disable, or mark tests as expected failures.
3. **Format**: run the project's formatter. Commit only formatted code.
4. **Lint**: check for linter config files (`.golangci.yml`, `.eslintrc`, `.flake8`, `clippy.toml`, etc.) or a Makefile `lint` target. Run it. Fix warnings your changes introduced.

## Style

- When an operation can fail, communicate the failure to the caller with context. Never silently discard an error or exception.
- Name things for what they do or represent, not how they're implemented internally.
- Prefer the standard library over external dependencies when the functionality is equivalent.
- Before writing a utility function, search the standard library and existing codebase for an equivalent.
- Keep functions short and focused. If a function does two things, it's two functions.
- Avoid premature abstraction. Abstract when a pattern repeats with the same reason to change, not just because two blocks look similar.

## Comments

When the language or format supports comments:

- Comment why, not what. `// increment i` is noise. `// retry with backoff because the API rate-limits at 100 req/s` is useful.
- Do not add comments to code that already explains itself through clear naming and structure.
- Do not write commented-out code. If you wrote it and it's not needed, delete it. If a human left commented-out code in the codebase, leave it alone; they may have a reason.
- Do not write comments that reference a previous state of the code ("moved from X," "used to be Y," "renamed from Z"). The code should describe what it is now, not what it was before. History lives in version control.
- Public APIs (exported functions, classes, modules) should have doc comments describing what they do, what they accept, and what they return. Internal helpers generally do not need doc comments unless their behavior is non-obvious.
- When a workaround or non-obvious decision is necessary, leave a comment explaining the reasoning so the next developer doesn't "fix" it.

When the language or format does not support comments (JSON, YAML in some contexts, binary formats), do not try to work around the limitation. Use clear key names, companion documentation, or schema files instead.

## Security

- Never hardcode secrets, API keys, passwords, or tokens in source code. Use environment variables, config files excluded from version control, or secret management systems.
- Sanitize and validate all input that crosses a trust boundary: user input, HTTP request parameters, file contents, data from external APIs. Assume external input is hostile until validated.
- When constructing database queries, commands, or markup from user input, use parameterized queries or escaping functions provided by the language or framework. Do not concatenate user input into query strings, shell commands, or HTML.
- When handling sensitive data (passwords, personal information, payment details), minimize how long it stays in memory, avoid logging it, and follow the project's established patterns for encryption and storage.

## Testing

- Test behavior through the public API where possible. Tests that depend on internal implementation details break when internals are refactored, even if behavior is unchanged.
- Cover error paths, not just the happy path. If a function can fail, test that it fails correctly.
- Use the project's established test patterns rather than introducing a new style.
- Tests are documentation. A well-named test explains what the code does and what happens when things go wrong. A reader should be able to understand the system's behavior by reading the test names alone.
- Prefer real dependencies over simulated ones when practical. Simulated dependencies (mocks, stubs, fakes) are for external boundaries: network services, databases, clocks, filesystems. For internal functions in the same codebase, call the real code.
