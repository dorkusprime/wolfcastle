package task

import (
	"fmt"
	"os"
	"time"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

func newScopeCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scope",
		Short: "Manage file scope locks for parallel execution",
	}

	cmd.AddCommand(newScopeAddCmd(app))
	return cmd
}

func newScopeAddCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <file> [<file>...]",
		Short: "Acquire scope locks on files",
		Long: `Claims exclusive access to one or more files for the current task. If any
requested file conflicts with a lock held by another task, the entire
request is rejected (all-or-nothing). Locks already held by the same
task are silently accepted.

Examples:
  wolfcastle task scope add --node my-project/api-layer internal/daemon/iteration.go
  wolfcastle task scope add --node my-project/api-layer --task task-0001 internal/daemon/
  wolfcastle task scope add --node my-project/api-layer file1.go file2.go file3.go`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeAddr, _ := cmd.Flags().GetString("node")
			taskID, _ := cmd.Flags().GetString("task")

			taskAddr := nodeAddr + "/" + taskID

			var conflicts []state.ScopeConflict

			if err := app.State.MutateScopeLocks(func(table *state.ScopeLockTable) error {
				conflicts = state.FindConflicts(args, table, taskAddr)
				if len(conflicts) > 0 {
					return nil // don't write; we'll handle output below
				}

				now := time.Now().UTC()
				pid := os.Getpid()
				for _, file := range args {
					// Idempotent: overwriting own lock is a no-op in effect.
					table.Locks[file] = state.ScopeLock{
						Task:       taskAddr,
						Node:       nodeAddr,
						AcquiredAt: now,
						PID:        pid,
					}
				}
				return nil
			}); err != nil {
				return err
			}

			if len(conflicts) > 0 {
				errMsg := fmt.Sprintf("scope conflict: %s held by %s",
					conflicts[0].File, conflicts[0].HeldByTask)

				if app.JSON {
					output.Print(output.Response{
						OK:     false,
						Action: "task_scope_add",
						Error:  errMsg,
						Code:   1,
						Data:   map[string]any{"conflicts": conflicts},
					})
				} else {
					output.PrintError("%s", errMsg)
				}
				os.Exit(1)
			}

			if app.JSON {
				output.Print(output.Ok("task_scope_add", map[string]any{
					"acquired": args,
					"node":     nodeAddr,
					"task":     taskID,
				}))
			} else {
				output.PrintHuman("Acquired scope locks: %v", args)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Node address (required)")
	cmd.Flags().String("task", "", "Task ID (optional, derived from context if omitted)")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}
