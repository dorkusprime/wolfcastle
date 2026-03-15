package audit

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

func newSummaryCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "summary [text]",
		Short: "Set the result summary on a node's audit record",
		Long: `Sets a one-paragraph result summary on a node's audit trail. This should
be called before signaling WOLFCASTLE_COMPLETE on the final task of a node.

Examples:
  wolfcastle audit summary --node my-project "Implemented JWT auth with full test coverage"
  wolfcastle audit summary --node auth/login "Refactored login flow to use OAuth2"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireResolver(); err != nil {
				return err
			}
			text := args[0]
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("summary text cannot be empty")
			}
			nodeAddr, _ := cmd.Flags().GetString("node")
			if nodeAddr == "" {
				return fmt.Errorf("--node is required: specify the target node address")
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

			ns.Audit.ResultSummary = text

			if err := state.SaveNodeState(statePath, ns); err != nil {
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
