# User items as they come up. Don't process these until directed.

# New

- Restructure `.wolfcastle/` to separate system internals from model outputs.
  - Move `base/`, `custom/`, `local/`, `projects/`, `logs/` under `.wolfcastle/system/`.
  - Keep `docs/` and `artifacts/` at `.wolfcastle/` top level.
  - Prompt rule becomes: "Write to `.wolfcastle/docs/` and `.wolfcastle/artifacts/`. Never touch `.wolfcastle/system/`."
  - Breaking change: every path in the codebase adds a `system/` segment (config loading, state store, log directory, scaffold, resolver).
  - Needs an ADR. Migration path for existing `.wolfcastle/` directories (rescaffold handles it).
- Git progress check fails when model commits its own changes. `checkGitProgress` only checks `git status --porcelain` (uncommitted changes), but the execute prompt tells models to commit via `git commit`. After committing, the working tree is clean, so the progress check reports no progress even though real work was done and committed.
  - Fix: record the HEAD commit SHA before invocation, then after invocation check BOTH `git status --porcelain` (uncommitted) AND `git rev-parse HEAD` != saved SHA (new commits). Either condition means progress.
- Spinner visible during model execution. Should only spin during idle (IterationNoWork).
  - The spinner reappears after `Executing...` is printed. Suspect: the console logger's `PauseSpinner()`/`ResumeSpinner()` cycle re-activates the spinner, or `Stop()` isn't clearing `activeSpinner` before the next `writeConsole` call resumes it.
  - Reproduction: start daemon with empty tree, add inbox item, watch the spinner persist through execution.

## Done

- ~~Let's add `requirements` to the README~~ — added: Go 1.26+, Git, a coding agent
- ~~Terminal marker not detected~~ — misdiagnosis. `scanTerminalMarker` already handles the `result` envelope correctly. The actual problem was the deliverable unchanged check clearing the marker.
- ~~Deliverable globs don't recurse into subdirectories~~ — `globRecursive` now walks subdirs when the filename part contains wildcards. `cmd/*.go` matches `cmd/task/add.go`.
- ~~Deliverable unchanged false failures on retry~~ — replaced baseline hashing with git-diff progress detection.
- ~~CLI header cleanup~~ — [INFO] lines suppressed, version deduped, header reordered.
- ~~Status detail for completed tasks~~ — summary shown for completed tasks.
