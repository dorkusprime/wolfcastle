# CWD-based instance routing

## Status
Accepted

## Date
2026-04-06

## Context

With multiple wolfcastle daemons running in separate worktrees, CLI commands need a way to determine which instance to target. The user should not have to specify the instance explicitly in the common case (they're already inside the worktree they care about).

## Options Considered

1. **Always explicit.** Every command requires `--instance <path>`. Safe but tedious for the 95% case where you're already in the right directory.

2. **CWD-based with explicit override.** Resolve the current working directory, match it against registered instances by worktree path. Fall back to a selector or error when ambiguous. `--instance` flag for explicit override.

3. **Environment variable.** `WOLFCASTLE_INSTANCE` points to the active instance. Set by `wolfcastle start` in the shell environment. Breaks when switching terminals or using tools that don't inherit the env.

## Decision

CWD-based with explicit override. The routing algorithm:

1. If `--instance` is set, use it directly.
2. Resolve CWD with `filepath.EvalSymlinks`.
3. Scan live registry entries. A match requires the CWD to equal the worktree path or be a subdirectory, with a path separator boundary (prevents `/repo/feat/auth` from matching `/repo/feat/auth-v2`).
4. If multiple candidates match, use the longest worktree path (most specific).
5. One match: route to it. Zero matches: error. Ambiguous after longest-prefix (safety net): interactive selector or non-interactive error with candidate list.

The `--instance` flag is available only on commands that talk to a running daemon (start, stop, status, log, inbox), not as a global persistent flag.

## Consequences

- The common case (user is inside the worktree) requires zero extra flags.
- Symlinked paths resolve to the same canonical path as direct paths.
- Non-interactive contexts (CI, piped output, `--json`) get a structured error instead of a selector prompt.
- The `--instance` flag provides an escape hatch for automation and scripts that need deterministic routing.
