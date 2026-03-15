package audit

import (
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

func newHistoryCmd(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "history",
		Short: "Review past audit decisions",
		Long: `Shows completed audit batches and their verdicts. Most recent first.

Examples:
  wolfcastle audit history
  wolfcastle audit history --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			historyPath := filepath.Join(app.WolfcastleDir, "audit-review-history.json")
			history, err := state.LoadHistory(historyPath)
			if err != nil {
				return err
			}

			if app.JSONOutput {
				output.Print(output.Ok("audit_history", map[string]any{
					"entries": history.Entries,
					"count":   len(history.Entries),
				}))
				return nil
			}

			if len(history.Entries) == 0 {
				output.PrintHuman("No audit history on record.")
				return nil
			}

			// Show most recent first
			for i := len(history.Entries) - 1; i >= 0; i-- {
				entry := history.Entries[i]
				output.PrintHuman("Batch %s (completed %s)", entry.BatchID, entry.CompletedAt.Format("2006-01-02 15:04"))
				output.PrintHuman("  Scopes: %v", entry.Scopes)

				approved := 0
				rejected := 0
				for _, d := range entry.Decisions {
					switch d.Action {
					case string(state.FindingApproved):
						approved++
					case string(state.FindingRejected):
						rejected++
					}
				}
				output.PrintHuman("  Decisions: %d approved, %d rejected", approved, rejected)

				for _, d := range entry.Decisions {
					marker := "+"
					extra := ""
					if d.Action == string(state.FindingRejected) {
						marker = "-"
					}
					if d.CreatedNode != "" {
						extra = " → " + d.CreatedNode
					}
					output.PrintHuman("    [%s] %s%s", marker, d.Title, extra)
				}
				output.PrintHuman("")
			}

			return nil
		},
	}
}
