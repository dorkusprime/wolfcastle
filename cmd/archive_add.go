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
	"github.com/spf13/cobra"
)

// archiveAddCmd generates an archive entry for a completed node.
var archiveAddCmd = &cobra.Command{
	Use:   "add",
	Short: "Archive a completed node",
	Long: `Generates a Markdown archive entry for a completed node. The node
must be in the 'complete' state. Includes audit trail, results,
and metadata.

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

		ns, err := app.State.ReadNode(nodeAddr)
		if err != nil {
			return fmt.Errorf("loading node state: %w", err)
		}

		if ns.State != state.StatusComplete {
			return fmt.Errorf("node %s is %s, not complete. Finish the job first", nodeAddr, ns.State)
		}

		cfg, err := app.Config.Load()
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		root := app.Config.Root()

		// Get current branch
		branch := ""
		branchCmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		branchCmd.Dir = filepath.Dir(root)
		if out, err := branchCmd.Output(); err == nil {
			branch = strings.TrimSpace(string(out))
		}

		entry := archive.GenerateEntry(nodeAddr, ns, cfg, branch, ns.Audit.ResultSummary)

		archiveDir := filepath.Join(root, "archive")
		if err := os.MkdirAll(archiveDir, 0755); err != nil {
			return fmt.Errorf("creating archive directory: %w", err)
		}
		archivePath := filepath.Join(archiveDir, entry.Filename)

		if err := os.WriteFile(archivePath, []byte(entry.Content), 0644); err != nil {
			return fmt.Errorf("writing archive entry: %w", err)
		}

		if app.JSONOutput {
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
	_ = archiveAddCmd.MarkFlagRequired("node")
	archiveCmd.AddCommand(archiveAddCmd)
}
