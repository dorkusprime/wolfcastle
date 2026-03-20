package audit

import (
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

func newEscalateCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "escalate [gap description]",
		Short: "Escalate a gap to the parent node",
		Long: `Pushes a gap up to the parent orchestrator. The parent needs to
deal with it. Missing requirements, unclear scope, cross-cutting
concerns. Root-level nodes cannot escalate (no parent).

Examples:
  wolfcastle audit escalate --node auth/login "missing error handling spec"
  wolfcastle audit escalate --node api/endpoints "rate limiting not defined"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireIdentity(); err != nil {
				return err
			}
			description := args[0]
			if strings.TrimSpace(description) == "" {
				return fmt.Errorf("escalation description cannot be empty. Describe the problem")
			}
			nodeAddr, _ := cmd.Flags().GetString("node")
			if nodeAddr == "" {
				return fmt.Errorf("--node is required: specify the source node address")
			}

			addr, err := tree.ParseAddress(nodeAddr)
			if err != nil {
				return fmt.Errorf("invalid node address: %w", err)
			}
			parentAddr := addr.Parent()
			if parentAddr.IsRoot() {
				return fmt.Errorf("root-level node has no parent to escalate to")
			}

			if err := app.State.MutateNode(parentAddr.String(), func(parentState *state.NodeState) error {
				state.AddEscalation(parentState, nodeAddr, description, "", app.Clock)
				return nil
			}); err != nil {
				return err
			}

			if app.JSONOutput {
				output.Print(output.Ok("audit_escalate", map[string]string{
					"source": nodeAddr,
					"parent": parentAddr.String(),
					"gap":    description,
				}))
			} else {
				output.PrintHuman("Escalated to %s: %s", parentAddr.String(), description)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Source node address (required)")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}
