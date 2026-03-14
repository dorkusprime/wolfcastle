package cmd

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

var auditGapCmd = &cobra.Command{
	Use:   "gap [description]",
	Short: "Record an audit gap on a node",
	Long: `Appends a new gap to the node's audit record. Gaps represent issues
found during audit that need resolution before the audit can pass.

Examples:
  wolfcastle audit gap --node my-project "missing error handling in auth module"
  wolfcastle audit gap --node api/endpoints "no rate limiting tests"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireResolver(); err != nil {
			return err
		}
		description := args[0]
		if strings.TrimSpace(description) == "" {
			return fmt.Errorf("gap description cannot be empty")
		}
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

		gapID := fmt.Sprintf("gap-%s-%d", ns.ID, len(ns.Audit.Gaps)+1)
		ns.Audit.Gaps = append(ns.Audit.Gaps, state.Gap{
			ID:          gapID,
			Timestamp:   time.Now().UTC(),
			Description: description,
			Source:       nodeAddr,
			Status:       "open",
		})

		if err := state.SaveNodeState(statePath, ns); err != nil {
			return err
		}

		if jsonOutput {
			output.Print(output.Ok("audit_gap", map[string]string{
				"node":   nodeAddr,
				"gap_id": gapID,
			}))
		} else {
			output.PrintHuman("Gap %s recorded on %s", gapID, nodeAddr)
		}
		return nil
	},
}

func init() {
	auditGapCmd.Flags().String("node", "", "Target node address (required)")
	auditGapCmd.MarkFlagRequired("node")
	auditCmd.AddCommand(auditGapCmd)
}
