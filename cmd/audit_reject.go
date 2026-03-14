package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/review"
	"github.com/spf13/cobra"
)

var auditRejectCmd = &cobra.Command{
	Use:   "reject <finding-id | --all>",
	Short: "Reject an audit finding — no project will be created",
	Long: `Rejects a pending audit finding, recording the decision without
creating any project. Use --all to reject every remaining finding.

When all findings have been decided, the batch is archived to history
and the pending file is removed.

Examples:
  wolfcastle audit reject finding-3
  wolfcastle audit reject --all`,
	RunE: func(cmd *cobra.Command, args []string) error {
		allFlag, _ := cmd.Flags().GetBool("all")
		if !allFlag && len(args) == 0 {
			return fmt.Errorf("provide a finding ID or use --all")
		}

		batchPath := filepath.Join(wolfcastleDir, "audit-review.json")
		batch, err := review.LoadBatch(batchPath)
		if err != nil {
			return err
		}
		if batch == nil {
			return fmt.Errorf("no pending review batch — run 'wolfcastle audit run' first")
		}

		now := time.Now().UTC()
		var rejected []review.Decision

		for i := range batch.Findings {
			f := &batch.Findings[i]
			if f.Status != review.FindingPending {
				continue
			}
			if !allFlag && (len(args) == 0 || args[0] != f.ID) {
				continue
			}

			f.Status = review.FindingRejected
			f.DecidedAt = &now

			rejected = append(rejected, review.Decision{
				FindingID: f.ID,
				Title:     f.Title,
				Action:    string(review.FindingRejected),
				Timestamp: now,
			})

			if !jsonOutput {
				output.PrintHuman("  Rejected: %s (%s)", f.ID, f.Title)
			}
		}

		if len(rejected) == 0 {
			if allFlag {
				return fmt.Errorf("no pending findings to reject")
			}
			return fmt.Errorf("finding %q not found or already decided", args[0])
		}

		// Save updated batch
		if err := review.SaveBatch(batchPath, batch); err != nil {
			return err
		}

		// Check if batch is fully decided
		if err := finalizeBatchIfComplete(batch, batchPath); err != nil {
			return err
		}

		if jsonOutput {
			output.Print(output.Ok("audit_reject", map[string]any{
				"rejected":  len(rejected),
				"decisions": rejected,
			}))
		} else {
			output.PrintHuman("\nRejected %d finding(s).", len(rejected))
		}

		return nil
	},
}

func init() {
	auditRejectCmd.Flags().Bool("all", false, "Reject all pending findings")
	auditCmd.AddCommand(auditRejectCmd)
}
