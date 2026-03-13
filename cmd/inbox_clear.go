package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
		data, err := os.ReadFile(inboxPath)
		if err != nil {
			if os.IsNotExist(err) {
				output.PrintHuman("Inbox is empty")
				return nil
			}
			return fmt.Errorf("reading inbox: %w", err)
		}

		var inbox inboxFile
		if err := json.Unmarshal(data, &inbox); err != nil {
			return fmt.Errorf("parsing inbox: %w", err)
		}

		originalCount := len(inbox.Items)

		if clearAll {
			inbox.Items = nil
		} else {
			var kept []inboxItem
			for _, item := range inbox.Items {
				if item.Status == "new" {
					kept = append(kept, item)
				}
			}
			inbox.Items = kept
		}

		removedCount := originalCount - len(inbox.Items)

		out, err := json.MarshalIndent(inbox, "", "  ")
		if err != nil {
			return err
		}
		out = append(out, '\n')
		if err := os.WriteFile(inboxPath, out, 0644); err != nil {
			return err
		}

		if jsonOutput {
			output.Print(output.Ok("inbox_clear", map[string]any{
				"removed":   removedCount,
				"remaining": len(inbox.Items),
			}))
		} else {
			output.PrintHuman("Cleared %d items from inbox (%d remaining)", removedCount, len(inbox.Items))
		}
		return nil
	},
}

func init() {
	inboxClearCmd.Flags().Bool("all", false, "Clear all items, including unprocessed")
	inboxCmd.AddCommand(inboxClearCmd)
}
