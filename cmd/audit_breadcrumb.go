package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

var auditBreadcrumbCmd = &cobra.Command{
	Use:   "breadcrumb [text]",
	Short: "Add a breadcrumb to a node's audit trail",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		text := args[0]
		nodeAddr, _ := cmd.Flags().GetString("node")
		if nodeAddr == "" {
			return fmt.Errorf("--node is required")
		}

		addr, err := tree.ParseAddress(nodeAddr)
		if err != nil {
			return fmt.Errorf("invalid node address: %w", err)
		}
		statePath := filepath.Join(resolver.ProjectsDir(), filepath.Join(addr.Parts...), "state.json")

		ns, err := state.LoadNodeState(statePath)
		if err != nil {
			return fmt.Errorf("loading node state: %w", err)
		}

		state.AddBreadcrumb(ns, nodeAddr, text)

		if err := state.SaveNodeState(statePath, ns); err != nil {
			return err
		}

		if jsonOutput {
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

func init() {
	auditBreadcrumbCmd.Flags().String("node", "", "Target node address (required)")
	auditBreadcrumbCmd.MarkFlagRequired("node")
	auditCmd.AddCommand(auditBreadcrumbCmd)
}
