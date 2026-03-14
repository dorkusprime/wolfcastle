package audit

import (
	"fmt"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

func newFixGapCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fix-gap [gap-id]",
		Short: "Mark an audit gap as fixed",
		Long: `Marks an open gap as fixed, recording who fixed it and when.

Examples:
  wolfcastle audit fix-gap --node my-project gap-my-project-1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireResolver(); err != nil {
				return err
			}
			gapID := args[0]
			nodeAddr, _ := cmd.Flags().GetString("node")
			if nodeAddr == "" {
				return fmt.Errorf("--node is required")
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

			found := false
			for i := range ns.Audit.Gaps {
				if ns.Audit.Gaps[i].ID == gapID {
					if ns.Audit.Gaps[i].Status == state.GapFixed {
						return fmt.Errorf("gap %s is already fixed", gapID)
					}
					ns.Audit.Gaps[i].Status = state.GapFixed
					ns.Audit.Gaps[i].FixedBy = nodeAddr
					now := app.Clock.Now()
					ns.Audit.Gaps[i].FixedAt = &now
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("gap %s not found in %s", gapID, nodeAddr)
			}

			if err := state.SaveNodeState(statePath, ns); err != nil {
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
	cmd.MarkFlagRequired("node")
	return cmd
}
