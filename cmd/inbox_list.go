package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/inbox"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

var inboxListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all items in the inbox",
	RunE: func(cmd *cobra.Command, args []string) error {
		inboxPath := filepath.Join(resolver.ProjectsDir(), "inbox.json")

		inboxData, err := inbox.Load(inboxPath)
		if err != nil {
			return fmt.Errorf("reading inbox: %w", err)
		}

		if jsonOutput {
			output.Print(output.Ok("inbox_list", map[string]any{
				"items": inboxData.Items,
				"count": len(inboxData.Items),
			}))
		} else {
			if len(inboxData.Items) == 0 {
				output.PrintHuman("Inbox is empty")
			} else {
				for i, item := range inboxData.Items {
					output.PrintHuman("  %d. [%s] %s (%s)", i+1, item.Status, item.Text, item.Timestamp)
				}
			}
		}
		return nil
	},
}

func init() {
	inboxCmd.AddCommand(inboxListCmd)
}
