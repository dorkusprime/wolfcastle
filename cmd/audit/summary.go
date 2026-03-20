package audit

import (
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

func newSummaryCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summary [text]",
		Short: "Record the final result summary",
		Long: `Sets a result summary on a node's audit record. Call this before
signaling WOLFCASTLE_COMPLETE on the final task.

Examples:
  wolfcastle audit summary --node my-project "Implemented JWT auth with full test coverage"
  wolfcastle audit summary --node auth/login "Refactored login flow to use OAuth2"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireIdentity(); err != nil {
				return err
			}
			text := args[0]
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("summary text cannot be empty. State the outcome")
			}
			nodeAddr, _ := cmd.Flags().GetString("node")
			if nodeAddr == "" {
				return fmt.Errorf("--node is required: specify the target node address")
			}

			if err := app.State.MutateNode(nodeAddr, func(ns *state.NodeState) error {
				ns.Audit.ResultSummary = text
				return nil
			}); err != nil {
				return err
			}

			if app.JSONOutput {
				output.Print(output.Ok("audit_summary", map[string]string{
					"node": nodeAddr,
					"text": text,
				}))
			} else {
				output.PrintHuman("Summary set on %s", nodeAddr)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Target node address (required)")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}
