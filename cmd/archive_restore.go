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

// archiveRestoreCmd restores an archived node back to active state.
var archiveRestoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore an archived node to active state",
	Long: `Restores a previously archived node and its subtree back to active
state. The node must be a root-level archived entry (present in
archived_root).

Examples:
  wolfcastle archive restore --node my-project`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := app.RequireIdentity(); err != nil {
			return err
		}
		nodeAddr, _ := cmd.Flags().GetString("node")
		if nodeAddr == "" {
			return fmt.Errorf("--node is required: specify the archived node to restore")
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

		// Collect the subtree (node + all descendants).
		subtree := collectArchiveSubtree(idx, nodeAddr)

		// Move state directories from .archive/ back to active locations.
		storeDir := app.State.Dir()
		for _, addr := range subtree {
			parts := strings.Split(addr, "/")
			archiveDir := filepath.Join(storeDir, ".archive", filepath.Join(parts...))
			activeDir := filepath.Join(storeDir, filepath.Join(parts...))

			if _, statErr := os.Stat(archiveDir); statErr != nil {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(activeDir), 0o755); err != nil {
				return fmt.Errorf("creating active dir for %s: %w", addr, err)
			}
			if err := os.Rename(archiveDir, activeDir); err != nil {
				return fmt.Errorf("restoring %s from archive: %w", addr, err)
			}
		}

		// Update the RootIndex atomically.
		if err := app.State.MutateIndex(func(idx *state.RootIndex) error {
			var newArchivedRoot []string
			for _, r := range idx.ArchivedRoot {
				if r != nodeAddr {
					newArchivedRoot = append(newArchivedRoot, r)
				}
			}
			idx.ArchivedRoot = newArchivedRoot
			idx.Root = append(idx.Root, nodeAddr)

			for _, addr := range subtree {
				if e, ok := idx.Nodes[addr]; ok {
					e.Archived = false
					e.ArchivedAt = nil
					idx.Nodes[addr] = e
				}
			}
			return nil
		}); err != nil {
			return fmt.Errorf("updating index: %w", err)
		}

		if app.JSON {
			output.Print(output.Ok("archive_restore", map[string]string{
				"action": "restored",
				"node":   nodeAddr,
			}))
		} else {
			output.PrintHuman("Restored %s from archive", nodeAddr)
		}
		return nil
	},
}

// collectArchiveSubtree walks the index Children slices to gather a node
// and all its descendants.
func collectArchiveSubtree(idx *state.RootIndex, root string) []string {
	var result []string
	var walk func(addr string)
	walk = func(addr string) {
		result = append(result, addr)
		if e, ok := idx.Nodes[addr]; ok {
			for _, child := range e.Children {
				walk(child)
			}
		}
	}
	walk(root)
	return result
}

func init() {
	archiveRestoreCmd.Flags().String("node", "", "Archived node address to restore (required)")
	_ = archiveRestoreCmd.MarkFlagRequired("node")
	archiveCmd.AddCommand(archiveRestoreCmd)
}
