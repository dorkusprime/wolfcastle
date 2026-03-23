# Shell

When the codebase you're working in has established conventions that differ from what's described here, follow the codebase.

## Style

Prefer `set -euo pipefail` at the top of Bash scripts. `-e` exits on error, `-u` treats unset variables as errors, `-o pipefail` propagates failures through pipes. For scripts that must run under POSIX `sh`, use `set -eu` (pipefail is a Bash extension).

Prefer double-quoting all variable expansions and command substitutions: `"$var"`, `"$(command)"`. Unquoted expansions undergo word splitting and glob expansion, which silently breaks on filenames with spaces or special characters. The only safe place to omit quotes is inside `[[ ]]` test brackets in Bash.

Prefer `$()` over backticks for command substitution. `$()` nests cleanly, reads more clearly, and avoids backslash-escaping ambiguities that backticks introduce.

Prefer builtins over external commands when performance or portability matters. `${var%pattern}` and `${var#pattern}` are faster and more portable than calling `sed` or `awk` for simple string manipulation. Use `printf` over `echo` when output must be predictable across platforms (`echo` behavior varies with flags and escape sequences).

Prefer `local` for variables inside functions to avoid polluting the global namespace. In POSIX `sh`, which lacks `local`, use subshells or careful naming conventions to contain scope.

Prefer `readonly` for constants and configuration values set once at script startup. This catches accidental reassignment and communicates intent to readers.

Prefer `[[ ]]` over `[ ]` in Bash for conditional tests. `[[ ]]` handles empty variables without quoting errors, supports pattern matching with `=~`, and does not perform word splitting on its operands. In POSIX `sh` scripts, use `[ ]` with careful quoting.

Prefer functions over repeated inline code. Name them with underscores (`process_file`, `validate_input`) and define them before their first call. Place the `main` function at the bottom of the script and invoke it with `main "$@"` as the last line.

## Build and Test

Prefer ShellCheck for static analysis. Run `shellcheck -x script.sh` to follow sourced files. ShellCheck catches quoting errors, deprecated syntax, and portability issues that are invisible to manual review. When the project has a `.shellcheckrc`, respect its directives; add inline `# shellcheck disable=SC2xxx` comments only with a justifying explanation.

Prefer `shfmt` for formatting. Run `shfmt -w -i 2 -ci -s` (or the project's chosen indent width) to normalize style. The `-ci` flag indents switch cases, and `-s` simplifies code. `shfmt -d` shows diffs without writing, useful for CI gates. Run both ShellCheck and shfmt as pre-commit hooks.

Prefer Bats (Bash Automated Testing System) for structured tests. The `bats-core` project is the maintained fork. Install `bats-support` and `bats-assert` helper libraries for readable assertions.

## Testing

Prefer one `.bats` file per script or logical module, stored in a `test/` or `tests/` directory. Name test files to mirror what they exercise: `test/deploy.bats` for `scripts/deploy.sh`.

Prefer `setup()` and `teardown()` functions for test fixture management. Create temporary directories with `BATS_TMPDIR` or `mktemp -d` in `setup()` and clean them in `teardown()` to avoid cross-test pollution.

```bash
setup() {
    load 'test_helper/bats-support/load'
    load 'test_helper/bats-assert/load'
    TEST_DIR="$(mktemp -d)"
}

teardown() {
    rm -rf "$TEST_DIR"
}

@test "backup creates archive" {
    run backup.sh "$TEST_DIR/source" "$TEST_DIR/dest"
    assert_success
    assert [ -f "$TEST_DIR/dest/backup.tar.gz" ]
}
```

Prefer `run` to capture command output and exit status separately. `run` sets `$status` and `$output`, letting you assert on both without conflating them. Use `assert_success`, `assert_failure`, `assert_output`, and `assert_line` from the bats-assert library for clear, intention-revealing assertions.

## Common Pitfalls

Unquoted variables undergo word splitting and glob expansion. `for f in $files` splits on whitespace and expands globs; `for f in "$files"` treats the whole string as one token. To iterate over a list, prefer arrays: `for f in "${files[@]}"`.

Temporary files created with predictable names in shared directories (`/tmp/myapp.tmp`) invite symlink attacks and race conditions. Prefer `mktemp` to create temp files and directories with unique names. Store the path in a variable and clean up in a `trap`: `trap 'rm -rf "$tmpdir"' EXIT`.

Heredocs indented with spaces break when used with `<<-`, which strips only leading tabs. When heredocs appear inside indented functions or conditionals, use tabs for the heredoc body indentation, or use `<<` with no indentation.

Command substitution strips trailing newlines. `output="$(printf "hello\n\n")"` sets `output` to `"hello"`, losing both trailing newlines. When trailing whitespace matters, append a sentinel character and strip it: `output="$(printf "hello\n\n"; printf x)"; output="${output%x}"`.

Pipelines create subshells, and variable assignments inside subshells do not propagate to the parent. `cat file | while read -r line; do count=$((count+1)); done` leaves `count` unchanged after the loop. Prefer process substitution (`while read -r line; do ...; done < <(cat file)`) or redirect from a file directly.

Signal handlers set with `trap` are replaced, not stacked. A second `trap '...' EXIT` overwrites the first. When multiple cleanup actions are needed, write a single cleanup function that handles all of them, and register that function once.

`cd` in a script without `|| exit` continues execution in the wrong directory if the `cd` fails. Prefer `cd /target || exit 1`, or use `CDPATH=""` to prevent `cd` from matching unexpected directories through the search path.
