# CLI Commands

## Command Registration Pattern

All subcommand groups follow the same pattern:

1. A `register.go` file in the subpackage exports a `Register(app *cmdutil.App, parent *cobra.Command)` function
2. Individual command files export `newXxxCmd(app *cmdutil.App) *cobra.Command`
3. `Register()` calls `newXxxCmd()` for each command and adds them to the parent

Example:
```go
// cmd/task/register.go
func Register(app *cmdutil.App, parent *cobra.Command) {
    taskCmd := &cobra.Command{Use: "task", Short: "..."}
    taskCmd.AddCommand(newAddCmd(app))
    taskCmd.AddCommand(newClaimCmd(app))
    parent.AddCommand(taskCmd)
}
```

Root-level commands (not in subpackages) use `init()` to register with `rootCmd` directly.

## Command Checklist

When adding a new command:

- [ ] Follow the registration pattern above
- [ ] Add `--json` support (check `app.JSONOutput`)
- [ ] Use `output.PrintHuman()` / `output.Print()` for all output
- [ ] Call `app.RequireResolver()` early if the command needs the tree
- [ ] Add shell completion via `cmdutil.CompleteNodeAddresses()` or `CompleteTaskAddresses()`
- [ ] Include `Long` help text with examples
- [ ] Add the command to `cmd/completions.go` if it has completable flags
- [ ] Update `internal/pipeline/scriptref.go` to include the new command in the script reference

## Flag Conventions

- Required flags: use `cmd.MarkFlagRequired("name")`. Cobra enforces this
- `--node`: tree address (node path or node/task-id), validated via `tree.ParseAddress()`
- `--all`: batch operations (approve all, reject all)
- `--json`: global flag on root, available everywhere via `app.JSONOutput`

## Config-Skip Commands

These commands skip config loading in `PersistentPreRunE`: `init`, `version`, `help`. If you add a command that must work without `.wolfcastle/`, add it to the switch statement in `cmd/root.go:37`.
