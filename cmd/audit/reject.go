package audit

import (
	"fmt"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

func newRejectCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reject <finding-id | --all>",
		Short: "Dismiss a finding",
		Long: `Rejects a pending finding. No project created. Use --all to reject
everything remaining. When all findings are decided, the batch
archives to history.

Examples:
  wolfcastle audit reject finding-3
  wolfcastle audit reject --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			allFlag, _ := cmd.Flags().GetBool("all")
			if !allFlag && len(args) == 0 {
				return fmt.Errorf("provide a finding ID or use --all")
			}

			batchPath := filepath.Join(app.WolfcastleDir, "audit-state.json")
			batch, err := state.LoadBatch(batchPath)
			if err != nil {
				return err
			}
			if batch == nil {
				return fmt.Errorf("no pending batch. Run 'wolfcastle audit run' first")
			}

			now := app.Clock.Now()
			var rejected []state.Decision

			for i := range batch.Findings {
				f := &batch.Findings[i]
				if f.Status != state.FindingPending {
					continue
				}
				if !allFlag && (len(args) == 0 || args[0] != f.ID) {
					continue
				}

				f.Status = state.FindingRejected
				f.DecidedAt = &now

				rejected = append(rejected, state.Decision{
					FindingID: f.ID,
					Title:     f.Title,
					Action:    string(state.FindingRejected),
					Timestamp: now,
				})

				if !app.JSONOutput {
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
			if err := state.SaveBatch(batchPath, batch); err != nil {
				return err
			}

			// Check if batch is fully decided
			if err := finalizeBatchIfComplete(app, batch, batchPath); err != nil {
				return err
			}

			if app.JSONOutput {
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

	cmd.Flags().Bool("all", false, "Reject all pending findings")
	return cmd
}
