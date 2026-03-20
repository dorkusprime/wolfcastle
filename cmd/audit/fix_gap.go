package audit

import (
	"fmt"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

func newFixGapCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix-gap [gap-id]",
		Short: "Close an audit gap",
		Long: `Marks an open gap as fixed. Records who and when.

Examples:
  wolfcastle audit fix-gap --node my-project gap-my-project-1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireIdentity(); err != nil {
				return err
			}
			gapID := args[0]
			nodeAddr, _ := cmd.Flags().GetString("node")
			if nodeAddr == "" {
				return fmt.Errorf("--node is required: specify the target node address")
			}

			if err := app.State.MutateNode(nodeAddr, func(ns *state.NodeState) error {
				for i := range ns.Audit.Gaps {
					if ns.Audit.Gaps[i].ID == gapID {
						if ns.Audit.Gaps[i].Status == state.GapFixed {
							return fmt.Errorf("gap %s is already fixed", gapID)
						}
						ns.Audit.Gaps[i].Status = state.GapFixed
						ns.Audit.Gaps[i].FixedBy = nodeAddr
						now := app.Clock.Now()
						ns.Audit.Gaps[i].FixedAt = &now
						return nil
					}
				}
				return fmt.Errorf("gap %s not found in %s", gapID, nodeAddr)
			}); err != nil {
				return err
			}

			if app.JSONOutput {
				output.Print(output.Ok("audit_fix_gap", map[string]string{
					"node":   nodeAddr,
					"gap_id": gapID,
				}))
			} else {
				output.PrintHuman("Gap %s marked as fixed on %s", gapID, nodeAddr)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Target node address (required)")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}
