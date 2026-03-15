package task

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

func newCompleteCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "complete",
		Short: "Mark a task as destroyed",
		Long: `Transitions a task from in_progress to complete. Validation commands
run first if configured. When every task in a leaf is done, the
node falls and the victory propagates upward.

Examples:
  wolfcastle task complete --node my-project/task-1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireResolver(); err != nil {
				return err
			}
			nodeFlag, _ := cmd.Flags().GetString("node")
			if nodeFlag == "" {
				return fmt.Errorf("--node is required: specify the task address (e.g. my-project/task-1)")
			}

			nodeAddr, taskID, err := tree.SplitTaskAddress(nodeFlag)
			if err != nil {
				return fmt.Errorf("--node must be a task address: %w", err)
			}

			addr, err := tree.ParseAddress(nodeAddr)
			if err != nil {
				return fmt.Errorf("invalid node address: %w", err)
			}
			statePath := filepath.Join(app.Resolver.ProjectsDir(), filepath.Join(addr.Parts...), "state.json")

			ns, err := state.LoadNodeState(statePath)
			if err != nil {
				return fmt.Errorf("loading node state: %w", err)
			}

			if err := state.TaskComplete(ns, taskID); err != nil {
				return err
			}

			// Run configured validation commands before saving.
			// If a validation command fails, the error is returned before
			// SaveNodeState is called, so the in-memory mutation is discarded
			// and the on-disk state remains unchanged.
			if app.Cfg != nil {
				for _, vc := range app.Cfg.Validation.Commands {
					timeout := 30 * time.Second
					if vc.TimeoutSeconds > 0 {
						timeout = time.Duration(vc.TimeoutSeconds) * time.Second
					}
					ctx, cancel := context.WithTimeout(context.Background(), timeout)
					out, err := exec.CommandContext(ctx, "sh", "-c", vc.Run).CombinedOutput()
					cancel()
					if err != nil {
						return fmt.Errorf("validation command %q failed (completion not saved): %v\n%s", vc.Name, err, string(out))
					}
				}
			}

			if err := state.SaveNodeState(statePath, ns); err != nil {
				return err
			}

			// Propagate state up through parent orchestrators and root index
			if err := app.PropagateState(nodeAddr, ns.State); err != nil {
				return fmt.Errorf("propagating state: %w", err)
			}

			if app.JSONOutput {
				output.Print(output.Ok("task_complete", map[string]any{
					"address":    nodeFlag,
					"task_id":    taskID,
					"state":      string(state.StatusComplete),
					"node_state": string(ns.State),
				}))
			} else {
				output.PrintHuman("Destroyed %s", nodeFlag)
				if ns.State == state.StatusComplete {
					output.PrintHuman("Node %s eliminated", nodeAddr)
				}
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Task address: node-path/task-id (required)")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}
