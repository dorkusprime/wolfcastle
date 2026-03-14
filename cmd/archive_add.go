package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/archive"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

var archiveAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Generate an archive entry for a completed node",
	Long: `Generates a Markdown archive entry for a completed project node.

The node must be in the 'complete' state. The archive entry includes
the node's audit trail, result summary, and metadata.

Examples:
  wolfcastle archive add --node my-project`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireResolver(); err != nil {
			return err
		}
		nodeAddr, _ := cmd.Flags().GetString("node")
		if nodeAddr == "" {
			return fmt.Errorf("--node is required — specify the completed node to archive")
		}

		addr, err := tree.ParseAddress(nodeAddr)
		if err != nil {
			return fmt.Errorf("invalid node address: %w", err)
		}
		ns, err := resolver.LoadNodeState(addr)
		if err != nil {
			return fmt.Errorf("loading node state: %w", err)
		}

		if ns.State != state.StatusComplete {
			return fmt.Errorf("node %s is %s, must be complete to archive — finish all tasks first", nodeAddr, ns.State)
		}

		// Get current branch
		branch := ""
		branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		branchCmd.Dir = filepath.Dir(wolfcastleDir)
		if out, err := branchCmd.Output(); err == nil {
			branch = strings.TrimSpace(string(out))
		}

		entry := archive.GenerateEntry(nodeAddr, ns, cfg, branch, ns.Audit.ResultSummary)

		archiveDir := filepath.Join(wolfcastleDir, "archive")
		if err := os.MkdirAll(archiveDir, 0755); err != nil {
			return fmt.Errorf("creating archive directory: %w", err)
		}
		archivePath := filepath.Join(archiveDir, entry.Filename)

		if err := os.WriteFile(archivePath, []byte(entry.Content), 0644); err != nil {
			return fmt.Errorf("writing archive entry: %w", err)
		}

		if jsonOutput {
			output.Print(output.Ok("archive_add", map[string]string{
				"node":     nodeAddr,
				"filename": entry.Filename,
				"path":     archivePath,
			}))
		} else {
			output.PrintHuman("Archived %s → %s", nodeAddr, entry.Filename)
		}
		return nil
	},
}

func init() {
	archiveAddCmd.Flags().String("node", "", "Node address to archive (required)")
	archiveAddCmd.MarkFlagRequired("node")
	archiveCmd.AddCommand(archiveAddCmd)
}
