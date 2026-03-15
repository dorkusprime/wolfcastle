package inbox

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
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

			item := state.InboxItem{
				Timestamp: app.Clock.Now().Format("2006-01-02T15:04:05Z07:00"),
				Text:      text,
				Status:    "new",
			}

			if err := state.InboxAppend(inboxPath, item); err != nil {
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
