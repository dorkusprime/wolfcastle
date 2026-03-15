package audit

import (
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

func newGapCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gap [description]",
		Short: "Flag a gap in the audit record",
		Long: `Records a gap on a node's audit. Gaps are open issues that need
resolution before the audit passes.

Examples:
  wolfcastle audit gap --node my-project "missing error handling in auth module"
  wolfcastle audit gap --node api/endpoints "no rate limiting tests"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireResolver(); err != nil {
				return err
			}
			description := args[0]
			if strings.TrimSpace(description) == "" {
				return fmt.Errorf("gap description cannot be empty. Describe the gap")
			}
			nodeAddr, _ := cmd.Flags().GetString("node")
			if nodeAddr == "" {
				return fmt.Errorf("--node is required")
			}

			var gapID string
			if err := app.Store.MutateNode(nodeAddr, func(ns *state.NodeState) error {
				gapID = fmt.Sprintf("gap-%s-%d", ns.ID, len(ns.Audit.Gaps)+1)
				ns.Audit.Gaps = append(ns.Audit.Gaps, state.Gap{
					ID:          gapID,
					Timestamp:   app.Clock.Now(),
					Description: description,
					Source:      nodeAddr,
					Status:      state.GapOpen,
				})
				return nil
			}); err != nil {
				return err
			}

			if app.JSONOutput {
				output.Print(output.Ok("audit_gap", map[string]string{
					"node":   nodeAddr,
					"gap_id": gapID,
				}))
			} else {
				output.PrintHuman("Gap %s recorded on %s", gapID, nodeAddr)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Target node address (required)")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}
