package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/inbox"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

var inboxClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear processed items from the inbox",
	Long:  "Removes items with status 'processed' or 'expanded'. Use --all to remove everything.",
	RunE: func(cmd *cobra.Command, args []string) error {
		clearAll, _ := cmd.Flags().GetBool("all")

		inboxPath := filepath.Join(resolver.ProjectsDir(), "inbox.json")
		inboxData, err := inbox.Load(inboxPath)
		if err != nil {
			return fmt.Errorf("reading inbox: %w", err)
		}

		originalCount := len(inboxData.Items)

		if clearAll {
			inboxData.Items = nil
		} else {
			var kept []inbox.Item
			for _, item := range inboxData.Items {
				if item.Status == "new" {
					kept = append(kept, item)
				}
			}
			inboxData.Items = kept
		}

		removedCount := originalCount - len(inboxData.Items)

		if err := inbox.Save(inboxPath, inboxData); err != nil {
			return err
		}

		if jsonOutput {
			output.Print(output.Ok("inbox_clear", map[string]any{
				"removed":   removedCount,
				"remaining": len(inboxData.Items),
			}))
		} else {
			output.PrintHuman("Cleared %d items from inbox (%d remaining)", removedCount, len(inboxData.Items))
		}
		return nil
	},
}

func init() {
	inboxClearCmd.Flags().Bool("all", false, "Clear all items, including unprocessed")
	inboxCmd.AddCommand(inboxClearCmd)
}
