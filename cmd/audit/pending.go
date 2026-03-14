package audit

import (
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

func newPendingCmd(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "pending",
		Short: "Show pending audit findings awaiting review",
		Long: `Displays the current batch of audit findings that have not yet been
approved or rejected. If no pending batch exists, reports that.

Examples:
  wolfcastle audit pending
  wolfcastle audit pending --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			batchPath := filepath.Join(app.WolfcastleDir, "audit-state.json")
			batch, err := state.LoadBatch(batchPath)
			if err != nil {
				return err
			}
			if batch == nil {
				if app.JSONOutput {
					output.Print(output.Ok("audit_pending", map[string]any{
						"pending": 0,
					}))
				} else {
					output.PrintHuman("No pending audit review batch.")
				}
				return nil
			}

			var pending []state.Finding
			for _, f := range batch.Findings {
				if f.Status == state.FindingPending {
					pending = append(pending, f)
				}
			}

			if app.JSONOutput {
				output.Print(output.Ok("audit_pending", map[string]any{
					"batch_id": batch.ID,
					"scopes":   batch.Scopes,
					"pending":  len(pending),
					"total":    len(batch.Findings),
					"findings": pending,
				}))
			} else {
				if len(pending) == 0 {
					output.PrintHuman("All findings in batch %s have been reviewed.", batch.ID)
					output.PrintHuman("Run 'audit approve' or 'audit reject' on the last finding to archive the batch.")
					return nil
				}
				output.PrintHuman("Pending audit findings (batch %s, %d scope(s)):\n", batch.ID, len(batch.Scopes))
				for _, f := range pending {
					output.PrintHuman("  [%s] %s", f.ID, f.Title)
					if f.Description != "" {
						// Show first line of description
						desc := f.Description
						if idx := strings.IndexByte(desc, '\n'); idx > 0 {
							desc = desc[:idx]
						}
						if len(desc) > 80 {
							desc = desc[:77] + "..."
						}
						output.PrintHuman("         %s", desc)
					}
				}
				output.PrintHuman("\n  Approve: wolfcastle audit approve <id>")
				output.PrintHuman("  Reject:  wolfcastle audit reject <id>")
				output.PrintHuman("  Detail:  wolfcastle audit pending --json | jq '.data.findings[] | select(.id==\"<id>\")'")
			}
			return nil
		},
	}
}
