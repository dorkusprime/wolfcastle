package task

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

func newScopeCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scope",
		Short: "Claim and release territory for parallel operations",
	}

	cmd.AddCommand(newScopeAddCmd(app))
	cmd.AddCommand(newScopeListCmd(app))
	cmd.AddCommand(newScopeReleaseCmd(app))
	return cmd
}

func newScopeAddCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <file> [<file>...]",
		Short: "Claim exclusive territory over files",
		Long: `Claims exclusive territory over one or more files for the current task.
Two tasks cannot hold the same ground. If any requested file conflicts
with another task's claim, the entire request is rejected. No partial
victories. Reclaiming territory you already hold is accepted without
complaint.

Examples:
  wolfcastle task scope add --node my-project/api-layer internal/daemon/iteration.go
  wolfcastle task scope add --node my-project/api-layer --task task-0001 internal/daemon/
  wolfcastle task scope add --node my-project/api-layer file1.go file2.go file3.go`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeAddr, _ := cmd.Flags().GetString("node")
			taskID, _ := cmd.Flags().GetString("task")

			if taskID == "" {
				return fmt.Errorf("scope add requires --task when not running inside a daemon iteration")
			}

			// Validate all requested paths before touching the lock table.
			var invalid []string
			for _, p := range args {
				if !state.ValidateScopePath(p) {
					invalid = append(invalid, p)
				}
			}
			if len(invalid) > 0 {
				return fmt.Errorf("invalid scope path(s): %s (paths must be relative, non-empty, and contain no \"..\" segments)", strings.Join(invalid, ", "))
			}

			taskAddr := nodeAddr + "/" + taskID

			var conflicts []state.ScopeConflict

			errConflict := fmt.Errorf("scope conflict")
			err := app.State.MutateScopeLocks(func(table *state.ScopeLockTable) error {
				conflicts = state.FindConflicts(args, table, taskAddr)
				if len(conflicts) > 0 {
					return errConflict // abort the write entirely (all-or-nothing)
				}

				now := time.Now().UTC()
				pid := os.Getpid()
				for _, file := range args {
					if existing, held := table.Locks[file]; held && existing.Task == taskAddr {
						continue // Already ours; preserve original AcquiredAt and PID.
					}
					table.Locks[file] = state.ScopeLock{
						Task:       taskAddr,
						Node:       nodeAddr,
						AcquiredAt: now,
						PID:        pid,
					}
				}
				return nil
			})
			if err != nil && err != errConflict {
				return err
			}

			if len(conflicts) > 0 {
				parts := make([]string, len(conflicts))
				for i, c := range conflicts {
					parts[i] = fmt.Sprintf("%s (held by %s on node %s)", c.File, c.HeldByTask, c.HeldByNode)
				}
				errMsg := "scope conflict: " + strings.Join(parts, ", ")

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
				output.PrintHuman("Territory claimed: %v", args)
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
		Short: "Show who holds what territory",
		Long: `Shows all claimed territory. Without flags, the full map. With --node
or --task, only that sector.

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
					output.PrintHuman("No territory claimed. The field is empty.")
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
		Short: "Surrender claimed territory",
		Long: `Gives up territory held by a task. Without file arguments, surrenders
everything. With file arguments, only those positions. Releasing
territory you never held is a no-op. When no claims remain, the lock
file is removed entirely.

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
					output.PrintHuman("Nothing to surrender. Task held no territory.")
				} else {
					output.PrintHuman("Territory surrendered: %v", released)
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
