package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

var auditEscalateCmd = &cobra.Command{
	Use:   "escalate [gap description]",
	Short: "Escalate a gap to the parent node's audit",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		description := args[0]
		nodeAddr, _ := cmd.Flags().GetString("node")
		if nodeAddr == "" {
			return fmt.Errorf("--node is required")
		}

		addr, err := tree.ParseAddress(nodeAddr)
		if err != nil {
			return fmt.Errorf("invalid node address: %w", err)
		}
		parentAddr := addr.Parent()
		if parentAddr.IsRoot() {
			return fmt.Errorf("cannot escalate from a root-level node (no parent)")
		}

		parentStatePath := filepath.Join(resolver.ProjectsDir(), filepath.Join(parentAddr.Parts...), "state.json")
		parentState, err := state.LoadNodeState(parentStatePath)
		if err != nil {
			return fmt.Errorf("loading parent state: %w", err)
		}

		state.AddEscalation(parentState, nodeAddr, description, "")

		if err := state.SaveNodeState(parentStatePath, parentState); err != nil {
			return err
		}

		if jsonOutput {
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

func init() {
	auditEscalateCmd.Flags().String("node", "", "Source node address (required)")
	auditEscalateCmd.MarkFlagRequired("node")
	auditCmd.AddCommand(auditEscalateCmd)
}
