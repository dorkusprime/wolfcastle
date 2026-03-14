package inbox

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/inbox"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

func newAddCmd(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "add [idea]",
		Short: "Add an item to the inbox",
		Long: `Adds a quick idea or work item to the inbox for later triage.

Examples:
  wolfcastle inbox add "refactor the auth middleware"
  wolfcastle inbox add "investigate flaky test in CI"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireResolver(); err != nil {
				return err
			}
			text := args[0]
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("inbox item text cannot be empty")
			}

			inboxPath := filepath.Join(app.Resolver.ProjectsDir(), "inbox.json")

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

			if app.JSONOutput {
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
}
