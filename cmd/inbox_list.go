package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

var inboxListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all items in the inbox",
	RunE: func(cmd *cobra.Command, args []string) error {
		inboxPath := filepath.Join(resolver.ProjectsDir(), "inbox.json")

		data, err := os.ReadFile(inboxPath)
		if err != nil {
			if os.IsNotExist(err) {
				if jsonOutput {
					output.Print(output.Ok("inbox_list", map[string]any{
						"items": []inboxItem{},
						"count": 0,
					}))
				} else {
					output.PrintHuman("Inbox is empty")
				}
				return nil
			}
			return fmt.Errorf("reading inbox: %w", err)
		}

		var inbox inboxFile
		if err := json.Unmarshal(data, &inbox); err != nil {
			return fmt.Errorf("parsing inbox: %w", err)
		}

		if jsonOutput {
			output.Print(output.Ok("inbox_list", map[string]any{
				"items": inbox.Items,
				"count": len(inbox.Items),
			}))
		} else {
			if len(inbox.Items) == 0 {
				output.PrintHuman("Inbox is empty")
			} else {
				for i, item := range inbox.Items {
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
