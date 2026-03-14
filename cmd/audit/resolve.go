package audit

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

func newResolveCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "resolve [escalation-id]",
		Short: "Resolve an audit escalation",
		Long: `Marks an open escalation as resolved, recording who resolved it and when.

Examples:
  wolfcastle audit resolve --node my-project escalation-my-project-1`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireResolver(); err != nil {
				return err
			}
			escalationID := args[0]
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
			for i := range ns.Audit.Escalations {
				if ns.Audit.Escalations[i].ID == escalationID {
					if ns.Audit.Escalations[i].Status == state.EscalationResolved {
						return fmt.Errorf("escalation %s is already resolved", escalationID)
					}
					ns.Audit.Escalations[i].Status = state.EscalationResolved
					ns.Audit.Escalations[i].ResolvedBy = nodeAddr
					now := time.Now().UTC()
					ns.Audit.Escalations[i].ResolvedAt = &now
					found = true
					break
				}
			}

			if !found {
				return fmt.Errorf("escalation %s not found in %s", escalationID, nodeAddr)
			}

			if err := state.SaveNodeState(statePath, ns); err != nil {
				return err
			}

			if app.JSONOutput {
				output.Print(output.Ok("audit_resolve", map[string]string{
					"node":          nodeAddr,
					"escalation_id": escalationID,
				}))
			} else {
				output.PrintHuman("Escalation %s resolved on %s", escalationID, nodeAddr)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Target node address (required)")
	cmd.MarkFlagRequired("node")
	return cmd
}
