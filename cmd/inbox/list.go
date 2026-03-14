package inbox

import (
	"fmt"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/inbox"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

func newListCmd(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all items in the inbox",
		Long: `Lists all items in the inbox with their status and timestamp.

Examples:
  wolfcastle inbox list
  wolfcastle inbox list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireResolver(); err != nil {
				return err
			}
			inboxPath := filepath.Join(app.Resolver.ProjectsDir(), "inbox.json")

			inboxData, err := inbox.Load(inboxPath)
			if err != nil {
				return fmt.Errorf("reading inbox: %w", err)
			}

			if app.JSONOutput {
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
}
