package inbox

import (
	"fmt"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

func newListCmd(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show what's in the inbox",
		Long: `Lists every inbox item with its status and timestamp.

Examples:
  wolfcastle inbox list
  wolfcastle inbox list --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := resolveInstance(cmd, app); err != nil {
				return err
			}
			if err := app.RequireIdentity(); err != nil {
				return err
			}
			inboxPath := app.State.InboxPath()

			inboxData, err := state.LoadInbox(inboxPath)
			if err != nil {
				return fmt.Errorf("reading inbox: %w", err)
			}

			if app.JSON {
				output.Print(output.Ok("inbox_list", map[string]any{
					"items": inboxData.Items,
					"count": len(inboxData.Items),
				}))
			} else {
				if len(inboxData.Items) == 0 {
					output.PrintHuman("Inbox is empty. Nothing to triage.")
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
