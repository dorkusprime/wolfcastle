package audit

import (
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

func newAARCmd(app *cmdutil.App) *cobra.Command {
	var (
		taskID       string
		objective    string
		whatHappened string
		wentWell     []string
		improvements []string
		actionItems  []string
	)

	cmd := &cobra.Command{
		Use:   "aar",
		Short: "Record an After Action Review for a completed task",
		Long: `Records a structured After Action Review (AAR) for a task.
AARs capture what was attempted, what happened, what went well,
and what should change. They flow to subsequent tasks and into audits.

Examples:
  wolfcastle audit aar --node my-project --task task-0001 \
    --objective "Implement JWT validation" \
    --what-happened "Added JWT middleware with RS256 support" \
    --went-well "Clean separation of concerns" \
    --improvements "Error messages could be more specific" \
    --action-items "Add token refresh endpoint"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireIdentity(); err != nil {
				return err
			}
			nodeAddr, _ := cmd.Flags().GetString("node")
			if nodeAddr == "" {
				return fmt.Errorf("--node is required: specify the target node address")
			}
			if taskID == "" {
				return fmt.Errorf("--task is required: specify the task ID")
			}
			if strings.TrimSpace(objective) == "" {
				return fmt.Errorf("--objective is required: what did the task set out to do?")
			}
			if strings.TrimSpace(whatHappened) == "" {
				return fmt.Errorf("--what-happened is required: what actually happened?")
			}

			aar := state.AAR{
				TaskID:       taskID,
				Timestamp:    app.Clock.Now(),
				Objective:    objective,
				WhatHappened: whatHappened,
				WentWell:     wentWell,
				Improvements: improvements,
				ActionItems:  actionItems,
			}

			if err := app.State.MutateNode(nodeAddr, func(ns *state.NodeState) error {
				state.AddAAR(ns, aar)
				return nil
			}); err != nil {
				return err
			}

			if app.JSONOutput {
				output.Print(output.Ok("audit_aar", map[string]string{
					"node": nodeAddr,
					"task": taskID,
				}))
			} else {
				output.PrintHuman("AAR recorded for %s/%s", nodeAddr, taskID)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Target node address (required)")
	_ = cmd.MarkFlagRequired("node")
	cmd.Flags().StringVar(&taskID, "task", "", "Task ID (required)")
	cmd.Flags().StringVar(&objective, "objective", "", "What the task set out to do (required)")
	cmd.Flags().StringVar(&whatHappened, "what-happened", "", "What actually happened (required)")
	cmd.Flags().StringArrayVar(&wentWell, "went-well", nil, "Things that went well (repeatable)")
	cmd.Flags().StringArrayVar(&improvements, "improvements", nil, "Things that could be improved (repeatable)")
	cmd.Flags().StringArrayVar(&actionItems, "action-items", nil, "Follow-up items for next tasks (repeatable)")

	return cmd
}
