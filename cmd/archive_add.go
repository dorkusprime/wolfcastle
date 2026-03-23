package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/archive"
	"github.com/dorkusprime/wolfcastle/internal/git"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

// archiveAddCmd archives a completed root-level node, moving its state
// directories to .archive/ and updating the RootIndex.
var archiveAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Archive a completed node",
	Long: `Archives a completed root-level node. Generates a Markdown rollup,
moves state directories to .archive/, and updates the RootIndex
(moves the address to archived_root and flags entries as archived).

The node must be root-level, in the 'complete' state, and not
already archived.

Examples:
  wolfcastle archive add --node my-project`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := app.RequireIdentity(); err != nil {
			return err
		}
		nodeAddr, _ := cmd.Flags().GetString("node")
		if nodeAddr == "" {
			return fmt.Errorf("--node is required: specify the completed node to archive")
		}

		// Validate preconditions: node exists, is complete, is root-level,
		// and is not already archived.
		idx, err := app.State.ReadIndex()
		if err != nil {
			return fmt.Errorf("loading index: %w", err)
		}
		entry, ok := idx.Nodes[nodeAddr]
		if !ok {
			return fmt.Errorf("node %q does not exist. Check available nodes with 'wolfcastle status'", nodeAddr)
		}

		ns, err := app.State.ReadNode(nodeAddr)
		if err != nil {
			return fmt.Errorf("loading node state: %w", err)
		}
		if ns.State != state.StatusComplete {
			return fmt.Errorf("node %s is %s, not complete. Finish the job first", nodeAddr, ns.State)
		}
		if entry.Archived {
			return fmt.Errorf("node %q is already archived", nodeAddr)
		}
		isRoot := false
		for _, r := range idx.Root {
			if r == nodeAddr {
				isRoot = true
				break
			}
		}
		if !isRoot {
			return fmt.Errorf("node %q is not a root-level node", nodeAddr)
		}

		cfg, err := app.Config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Resolve current git branch for the rollup metadata.
		branch := ""
		gitSvc := git.NewService(filepath.Dir(app.Config.Root()))
		if b, err := gitSvc.CurrentBranch(); err == nil {
			branch = b
		}

		svc := &archive.Service{
			Store:         app.State,
			WolfcastleDir: app.Config.Root(),
			Clock:         app.Clock,
		}
		if err := svc.Archive(nodeAddr, cfg, branch); err != nil {
			return fmt.Errorf("archiving node: %w", err)
		}

		// Re-read index to get the archived_at timestamp set by the service.
		idx, err = app.State.ReadIndex()
		if err != nil {
			return fmt.Errorf("reading updated index: %w", err)
		}

		if app.JSON {
			result := map[string]string{
				"action": "archived",
				"node":   nodeAddr,
			}
			if e, ok := idx.Nodes[nodeAddr]; ok && e.ArchivedAt != nil {
				result["archived_at"] = e.ArchivedAt.Format("2006-01-02T15:04:05Z07:00")
			}
			output.Print(output.Ok("archive_add", result))
		} else {
			output.PrintHuman("Archived %s", nodeAddr)
		}
		return nil
	},
}

func init() {
	archiveAddCmd.Flags().String("node", "", "Node address to archive (required)")
	_ = archiveAddCmd.MarkFlagRequired("node")
	archiveCmd.AddCommand(archiveAddCmd)
}
