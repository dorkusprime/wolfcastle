# Default

Baseline coding guidance for tasks without a language-specific class. When a language class (go.md, python.md, etc.) is assigned, it replaces this entirely. When the codebase has established conventions that differ from what's described here, follow the codebase.

## Build and Test

Look for the project's build system (Makefile, go.mod, package.json, Cargo.toml, pyproject.toml, CMakeLists.txt, or equivalent) and run whatever commands it provides.

1. **Build**: run the project's build command. Fix all errors before proceeding.
2. **Test**: run the full test suite. Fix all failures, including tests you didn't write. Do not skip, disable, or mark tests as expected failures.
3. **Format**: run the project's formatter. Commit only formatted code.
4. **Lint**: check for linter config files (`.golangci.yml`, `.eslintrc`, `.flake8`, `clippy.toml`, etc.) or a Makefile `lint` target. Run it. Fix warnings your changes introduced.

## Style

- Wrap or propagate errors with context. Never silently discard errors.
- Name things for what they do, not how they're implemented.
- Prefer the standard library over external dependencies when the functionality is equivalent.
- Before writing a utility function, search the standard library and existing codebase for an equivalent.

## Testing

- Test behavior through the public API where possible.
- Cover error paths, not just the happy path.
- Use the project's established test patterns (table-driven, BDD, parameterized, etc.) rather than introducing a new style.
