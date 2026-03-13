package cmd

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/inbox"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

var inboxAddCmd = &cobra.Command{
	Use:   "add [idea]",
	Short: "Add an item to the inbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		text := args[0]

		inboxPath := filepath.Join(resolver.ProjectsDir(), "inbox.json")

		inboxData, err := inbox.Load(inboxPath)
		if err != nil {
			return err
		}

		item := inbox.Item{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Text:      text,
			Status:    "new",
		}
		inboxData.Items = append(inboxData.Items, item)

		if err := inbox.Save(inboxPath, inboxData); err != nil {
			return fmt.Errorf("writing inbox: %w", err)
		}

		if jsonOutput {
			output.Print(output.Ok("inbox_add", map[string]string{
				"text":      text,
				"timestamp": item.Timestamp,
			}))
		} else {
			output.PrintHuman("Added to inbox: %s", text)
		}
		return nil
	},
}

func init() {
	inboxCmd.AddCommand(inboxAddCmd)
}
