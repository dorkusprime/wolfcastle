package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

type inboxItem struct {
	Timestamp string `json:"timestamp"`
	Text      string `json:"text"`
	Status    string `json:"status"`
}

type inboxFile struct {
	Items []inboxItem `json:"items"`
}

var inboxAddCmd = &cobra.Command{
	Use:   "add [idea]",
	Short: "Add an item to the inbox",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		text := args[0]

		inboxPath := filepath.Join(resolver.ProjectsDir(), "inbox.json")

		var inbox inboxFile
		data, err := os.ReadFile(inboxPath)
		if err == nil {
			json.Unmarshal(data, &inbox)
		}

		item := inboxItem{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Text:      text,
			Status:    "new",
		}
		inbox.Items = append(inbox.Items, item)

		out, _ := json.MarshalIndent(inbox, "", "  ")
		out = append(out, '\n')
		if err := os.WriteFile(inboxPath, out, 0644); err != nil {
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
