package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

// archiveDeleteCmd permanently removes an archived node and its subtree.
var archiveDeleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Permanently delete an archived node",
	Long: `Permanently removes an archived node and its subtree from the index
and archive store. This is irreversible. The --confirm flag is required.

Examples:
  wolfcastle archive delete --node my-project --confirm`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := app.RequireIdentity(); err != nil {
			return err
		}

		confirm, _ := cmd.Flags().GetBool("confirm")
		if !confirm {
			return fmt.Errorf("--confirm is required: this permanently deletes the archived node and cannot be undone")
		}

		nodeAddr, _ := cmd.Flags().GetString("node")
		if nodeAddr == "" {
			return fmt.Errorf("--node is required: specify the archived node to delete")
		}

		idx, err := app.State.ReadIndex()
		if err != nil {
			return fmt.Errorf("loading index: %w", err)
		}

		entry, ok := idx.Nodes[nodeAddr]
		if !ok {
			return fmt.Errorf("node %q not found in index", nodeAddr)
		}
		if !entry.Archived {
			return fmt.Errorf("node %q is not archived", nodeAddr)
		}

		inArchivedRoot := false
		for _, r := range idx.ArchivedRoot {
			if r == nodeAddr {
				inArchivedRoot = true
				break
			}
		}
		if !inArchivedRoot {
			return fmt.Errorf("node %q is not a root-level archived node", nodeAddr)
		}

		// Collect the subtree for index cleanup.
		subtree := collectArchiveSubtree(idx, nodeAddr)

		// Remove archived state directories permanently.
		storeDir := app.State.Dir()
		archiveRoot := filepath.Join(storeDir, ".archive", filepath.Join(strings.Split(nodeAddr, "/")...))
		if err := os.RemoveAll(archiveRoot); err != nil {
			return fmt.Errorf("removing archived directory for %s: %w", nodeAddr, err)
		}

		// Update the RootIndex atomically: remove from ArchivedRoot and
		// purge all subtree entries from the Nodes map.
		if err := app.State.MutateIndex(func(idx *state.RootIndex) error {
			var newArchivedRoot []string
			for _, r := range idx.ArchivedRoot {
				if r != nodeAddr {
					newArchivedRoot = append(newArchivedRoot, r)
				}
			}
			idx.ArchivedRoot = newArchivedRoot

			for _, addr := range subtree {
				delete(idx.Nodes, addr)
			}
			return nil
		}); err != nil {
			return fmt.Errorf("updating index: %w", err)
		}

		if app.JSON {
			output.Print(output.Ok("archive_delete", map[string]string{
				"action": "deleted",
				"node":   nodeAddr,
			}))
		} else {
			output.PrintHuman("Permanently deleted archived node %s", nodeAddr)
		}
		return nil
	},
}

func init() {
	archiveDeleteCmd.Flags().String("node", "", "Archived node address to delete (required)")
	_ = archiveDeleteCmd.MarkFlagRequired("node")
	archiveDeleteCmd.Flags().Bool("confirm", false, "Confirm permanent deletion (required)")
	archiveCmd.AddCommand(archiveDeleteCmd)
}
