package task

import (
	"fmt"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

func newAmendCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "amend",
		Short: "Modify an unstarted task's fields",
		Long: `Amends fields on a task that has not yet started. Tasks that are
in_progress or complete cannot be amended. Only the flags you provide
are applied; everything else stays untouched.

Examples:
  wolfcastle task amend --node my-project/task-0001 --body "updated description"
  wolfcastle task amend --node my-project/task-0001 --add-deliverable "docs/api.md"
  wolfcastle task amend --node my-project/task-0001 --type implementation --integration "feeds into auth module"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireIdentity(); err != nil {
				return err
			}
			nodeFlag, _ := cmd.Flags().GetString("node")
			if nodeFlag == "" {
				return fmt.Errorf("--node is required: specify the task address (e.g. my-project/task-0001)")
			}

			nodeAddr, taskID, err := tree.SplitTaskAddress(nodeFlag)
			if err != nil {
				return fmt.Errorf("--node must be a task address: %w", err)
			}

			body, _ := cmd.Flags().GetString("body")
			addDeliverable, _ := cmd.Flags().GetStringSlice("add-deliverable")
			addConstraint, _ := cmd.Flags().GetStringSlice("add-constraint")
			addAcceptance, _ := cmd.Flags().GetStringSlice("add-acceptance")
			addReference, _ := cmd.Flags().GetStringSlice("add-reference")
			taskType, _ := cmd.Flags().GetString("type")
			integration, _ := cmd.Flags().GetString("integration")

			// Validate task type if provided
			if taskType != "" {
				validTypes := map[string]bool{
					"discovery": true, "spec": true, "adr": true,
					"implementation": true, "integration": true, "cleanup": true,
				}
				if !validTypes[taskType] {
					return fmt.Errorf("invalid task type %q: must be one of discovery, spec, adr, implementation, integration, cleanup", taskType)
				}
			}

			if err := app.State.MutateNode(nodeAddr, func(ns *state.NodeState) error {
				for i := range ns.Tasks {
					if ns.Tasks[i].ID != taskID {
						continue
					}
					t := &ns.Tasks[i]
					if t.State == state.StatusInProgress || t.State == state.StatusComplete {
						return fmt.Errorf("cannot amend task %s: state is %s (must be not_started or blocked)", taskID, t.State)
					}
					if body != "" {
						t.Body = body
					}
					if taskType != "" {
						t.TaskType = taskType
					}
					if integration != "" {
						t.Integration = integration
					}
					t.Deliverables = appendUnique(t.Deliverables, addDeliverable)
					t.Constraints = appendUnique(t.Constraints, addConstraint)
					t.AcceptanceCriteria = appendUnique(t.AcceptanceCriteria, addAcceptance)
					t.References = appendUnique(t.References, addReference)
					return nil
				}
				return fmt.Errorf("task %s not found in node %s", taskID, nodeAddr)
			}); err != nil {
				return err
			}

			if app.JSON {
				output.Print(output.Ok("task_amend", map[string]string{
					"address": nodeFlag,
					"task_id": taskID,
				}))
			} else {
				output.PrintHuman("Amended task %s", nodeFlag)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Task address: node-path/task-id (required)")
	_ = cmd.MarkFlagRequired("node")
	cmd.Flags().String("body", "", "Replace task body/description")
	cmd.Flags().StringSlice("add-deliverable", nil, "Append deliverable (repeatable)")
	cmd.Flags().StringSlice("add-constraint", nil, "Append constraint (repeatable)")
	cmd.Flags().StringSlice("add-acceptance", nil, "Append acceptance criterion (repeatable)")
	cmd.Flags().StringSlice("add-reference", nil, "Append reference (repeatable)")
	cmd.Flags().String("type", "", "Task type: discovery, spec, adr, implementation, integration, cleanup")
	cmd.Flags().String("integration", "", "How this task connects to other work")
	return cmd
}

// appendUnique adds items from additions to base, skipping duplicates.
func appendUnique(base, additions []string) []string {
	if len(additions) == 0 {
		return base
	}
	seen := make(map[string]bool, len(base))
	for _, item := range base {
		seen[item] = true
	}
	for _, item := range additions {
		if !seen[item] {
			seen[item] = true
			base = append(base, item)
		}
	}
	return base
}
