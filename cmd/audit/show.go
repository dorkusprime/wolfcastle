package audit

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

func newShowCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Display a node's full audit record",
		Long: `Shows scope, breadcrumbs, gaps, escalations, status, and result
summary for a single node.

Examples:
  wolfcastle audit show --node my-project
  wolfcastle audit show --node my-project --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireIdentity(); err != nil {
				return err
			}
			nodeAddr, _ := cmd.Flags().GetString("node")
			if nodeAddr == "" {
				return fmt.Errorf("--node is required: specify the node to inspect")
			}

			addr, err := tree.ParseAddress(nodeAddr)
			if err != nil {
				return fmt.Errorf("invalid node address: %w", err)
			}
			statePath := filepath.Join(app.State.Dir(), filepath.Join(addr.Parts...), "state.json")

			ns, err := state.LoadNodeState(statePath)
			if err != nil {
				return fmt.Errorf("loading node state: %w", err)
			}

			if app.JSON {
				output.Print(output.Ok("audit_show", ns.Audit))
				return nil
			}

			output.PrintHuman("Audit for %s", nodeAddr)
			output.PrintHuman("  Status: %s", ns.Audit.Status)

			if ns.Audit.Scope != nil {
				output.PrintHuman("  Scope: %s", ns.Audit.Scope.Description)
				if len(ns.Audit.Scope.Files) > 0 {
					data, _ := json.Marshal(ns.Audit.Scope.Files)
					output.PrintHuman("    Files: %s", string(data))
				}
				if len(ns.Audit.Scope.Systems) > 0 {
					data, _ := json.Marshal(ns.Audit.Scope.Systems)
					output.PrintHuman("    Systems: %s", string(data))
				}
				if len(ns.Audit.Scope.Criteria) > 0 {
					data, _ := json.Marshal(ns.Audit.Scope.Criteria)
					output.PrintHuman("    Criteria: %s", string(data))
				}
			}

			if len(ns.Audit.Breadcrumbs) > 0 {
				output.PrintHuman("  Breadcrumbs (%d):", len(ns.Audit.Breadcrumbs))
				for _, bc := range ns.Audit.Breadcrumbs {
					output.PrintHuman("    [%s] %s: %s", bc.Timestamp.Format("2006-01-02 15:04"), bc.Task, bc.Text)
				}
			}

			if len(ns.Audit.Gaps) > 0 {
				output.PrintHuman("  Gaps (%d):", len(ns.Audit.Gaps))
				for _, gap := range ns.Audit.Gaps {
					output.PrintHuman("    %s [%s]: %s", gap.ID, gap.Status, gap.Description)
				}
			}

			if len(ns.Audit.Escalations) > 0 {
				output.PrintHuman("  Escalations (%d):", len(ns.Audit.Escalations))
				for _, esc := range ns.Audit.Escalations {
					output.PrintHuman("    %s [%s] from %s: %s", esc.ID, esc.Status, esc.SourceNode, esc.Description)
				}
			}

			if ns.Audit.ResultSummary != "" {
				output.PrintHuman("  Result Summary: %s", ns.Audit.ResultSummary)
			}

			return nil
		},
	}

	cmd.Flags().String("node", "", "Target node address (required)")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}
