package audit

import (
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

func newBreadcrumbCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "breadcrumb [text]",
		Short: "Add a breadcrumb to a node's audit trail",
		Long: `Records a timestamped breadcrumb note on a node's audit trail.

Breadcrumbs provide a narrative log of progress, decisions, and observations
that persists across daemon iterations.

Examples:
  wolfcastle audit breadcrumb --node my-project "refactored auth module"
  wolfcastle audit breadcrumb --node auth/login "switched to JWT tokens"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireResolver(); err != nil {
				return err
			}
			text := args[0]
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("breadcrumb text cannot be empty")
			}
			nodeAddr, _ := cmd.Flags().GetString("node")
			if nodeAddr == "" {
				return fmt.Errorf("--node is required: specify the target node address")
			}

			if err := app.Store.MutateNode(nodeAddr, func(ns *state.NodeState) error {
				state.AddBreadcrumb(ns, nodeAddr, text, app.Clock)
				return nil
			}); err != nil {
				return err
			}

			if app.JSONOutput {
				output.Print(output.Ok("audit_breadcrumb", map[string]string{
					"node": nodeAddr,
					"text": text,
				}))
			} else {
				output.PrintHuman("Breadcrumb added to %s", nodeAddr)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Target node address (required)")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}
