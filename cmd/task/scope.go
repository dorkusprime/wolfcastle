package task

import (
	"fmt"
	"os"
	"sort"
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
	cmd.AddCommand(newScopeListCmd(app))
	cmd.AddCommand(newScopeReleaseCmd(app))
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

func newScopeListCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List current scope locks",
		Long: `Lists file scope locks currently held by tasks. Without flags, shows all locks.
Use --node or --task to filter results.

Examples:
  wolfcastle task scope list
  wolfcastle task scope list --node my-project/api-layer
  wolfcastle task scope list --task my-project/api-layer/task-0001`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeFilter, _ := cmd.Flags().GetString("node")
			taskFilter, _ := cmd.Flags().GetString("task")

			table, err := app.State.ReadScopeLocks()
			if err != nil {
				return err
			}

			type lockEntry struct {
				File       string    `json:"file"`
				Task       string    `json:"task"`
				Node       string    `json:"node"`
				AcquiredAt time.Time `json:"acquired_at"`
			}

			var filtered []lockEntry
			for file, lock := range table.Locks {
				if nodeFilter != "" && lock.Node != nodeFilter {
					continue
				}
				if taskFilter != "" && lock.Task != taskFilter {
					continue
				}
				filtered = append(filtered, lockEntry{
					File:       file,
					Task:       lock.Task,
					Node:       lock.Node,
					AcquiredAt: lock.AcquiredAt,
				})
			}

			sort.Slice(filtered, func(i, j int) bool {
				return filtered[i].File < filtered[j].File
			})

			if app.JSON {
				output.Print(output.Ok("task_scope_list", map[string]any{
					"locks": filtered,
				}))
			} else {
				if len(filtered) == 0 {
					output.PrintHuman("No scope locks found.")
				} else {
					output.PrintHuman("%-40s %-30s %-30s %s", "FILE", "TASK", "NODE", "ACQUIRED")
					for _, e := range filtered {
						output.PrintHuman("%-40s %-30s %-30s %s",
							e.File, e.Task, e.Node,
							e.AcquiredAt.Format(time.RFC3339))
					}
				}
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Filter by node address")
	cmd.Flags().String("task", "", "Filter by task address")
	return cmd
}

func newScopeReleaseCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release [<file>...]",
		Short: "Release scope locks held by a task",
		Long: `Releases file scope locks held by a task. Without file arguments, releases
all locks held by the specified task. With file arguments, releases only
those specific files. Releasing a lock not held by the task is a no-op.

If the lock table becomes empty after release, the scope-locks.json file
is deleted.

Examples:
  wolfcastle task scope release --node my-project/api-layer --task task-0001
  wolfcastle task scope release --node my-project/api-layer --task task-0001 file1.go file2.go`,
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeAddr, _ := cmd.Flags().GetString("node")
			taskID, _ := cmd.Flags().GetString("task")

			taskAddr := nodeAddr + "/" + taskID

			var released []string
			var tableEmpty bool

			if err := app.State.MutateScopeLocks(func(table *state.ScopeLockTable) error {
				if len(args) == 0 {
					// Release all locks held by this task.
					for file, lock := range table.Locks {
						if lock.Task == taskAddr {
							released = append(released, file)
							delete(table.Locks, file)
						}
					}
				} else {
					// Release only the specified files if held by this task.
					for _, file := range args {
						lock, ok := table.Locks[file]
						if ok && lock.Task == taskAddr {
							released = append(released, file)
							delete(table.Locks, file)
						}
					}
				}
				tableEmpty = len(table.Locks) == 0
				return nil
			}); err != nil {
				return err
			}

			sort.Strings(released)

			// If the table is now empty, remove the file entirely.
			if tableEmpty {
				path := app.State.ScopeLocksPath()
				if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("removing empty scope locks file: %w", err)
				}
			}

			if app.JSON {
				output.Print(output.Ok("task_scope_release", map[string]any{
					"released": released,
					"node":     nodeAddr,
					"task":     taskID,
				}))
			} else {
				if len(released) == 0 {
					output.PrintHuman("No locks released.")
				} else {
					output.PrintHuman("Released scope locks: %v", released)
				}
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Node address (required)")
	cmd.Flags().String("task", "", "Task ID (required)")
	_ = cmd.MarkFlagRequired("node")
	_ = cmd.MarkFlagRequired("task")
	return cmd
}
