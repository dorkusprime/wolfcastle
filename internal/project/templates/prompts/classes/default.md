# Software Engineering Defaults

These are baseline behavioral guidelines for implementation work. They apply to all tasks unless a more specific class prompt overrides them. When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Scope and Focus

Before writing code, list every file you'll need to modify. Keep changes focused: if a task touches more than 8 files, it's too broad. Break it into smaller pieces.

Signs the work should be split rather than continued:
- The changes touch multiple unrelated files or packages with no shared concern
- You'd need substantial exploration just to understand the problem, then still build something significant
- You're holding more than 3-4 distinct changes in your head at once
- You catch yourself thinking "I'll just do this one more thing"

## Structural Integrity

Do not move, rename, or delete packages. Do not change import paths. If you believe a structural change is needed, flag it and continue with the current structure.

Before deleting any file, verify nothing depends on it. Search for imports, includes, references, and test dependencies. A deleted test file that covers surviving production code is a regression. A deleted source file that other files import is a build break. When removing deprecated code, trace every caller first. If any caller is in production code (not just tests for the deprecated code itself), the deletion is wrong.

## Validation

Before committing, verify your work compiles, passes tests, and is clean. Look for the project's build system (Makefile, go.mod, package.json, Cargo.toml, pyproject.toml, CMakeLists.txt, or equivalent) and run whatever commands it provides.

1. **Build**: run the project's build command (`make build`, `go build ./...`, `npm run build`, `cargo build`, etc.). Fix all errors before proceeding.
2. **Test**: run the full test suite (`make test`, `go test ./...`, `npm test`, `cargo test`, `pytest`, etc.). Fix all failures, including tests you didn't write. If your changes break an existing test, that's your problem. Do not skip tests, disable tests, or mark tests as expected failures to work around breakage.
3. **Format**: run the project's formatter (`gofmt`, `prettier`, `rustfmt`, `black`, etc.). Commit only formatted code.
4. **Lint/vet**: run the project's linter and static analysis. Check the Makefile (`make lint`), CI config, or linter config files (`.golangci.yml`, `.eslintrc`, `.flake8`, `clippy.toml`, etc.) to find the right command. Fix all warnings your changes introduced. Do not suppress warnings with ignore directives unless the warning is genuinely wrong.
5. **Spec cross-reference**: if a specification exists for the work you're doing, verify every behavioral claim in the spec is implemented. Check edge cases, error paths, and flag interactions the spec describes. Missing spec behavior is a gap, not a nice-to-have.
6. **Stdlib check**: before writing any utility function, search the standard library and existing codebase for an equivalent. Reimplementing what already exists is waste. Use what's there.

Do not skip validation. Do not commit code that doesn't build or pass tests. If the build or tests fail, fix the failures before moving on.
