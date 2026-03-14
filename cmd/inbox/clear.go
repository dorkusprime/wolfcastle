package inbox

import (
	"fmt"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

func newClearCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clear",
		Short: "Clear filed items from the inbox",
		Long: `Removes items with status 'filed' or 'expanded' from the inbox.
Items with status 'new' are kept unless --all is specified.

Examples:
  wolfcastle inbox clear
  wolfcastle inbox clear --all`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireResolver(); err != nil {
				return err
			}
			clearAll, _ := cmd.Flags().GetBool("all")

			inboxPath := filepath.Join(app.Resolver.ProjectsDir(), "inbox.json")
			inboxData, err := state.LoadInbox(inboxPath)
			if err != nil {
				return fmt.Errorf("reading inbox: %w", err)
			}

			originalCount := len(inboxData.Items)

			if clearAll {
				inboxData.Items = nil
			} else {
				var kept []state.InboxItem
				for _, item := range inboxData.Items {
					if item.Status == "new" {
						kept = append(kept, item)
					}
				}
				inboxData.Items = kept
			}

			removedCount := originalCount - len(inboxData.Items)

			if err := state.SaveInbox(inboxPath, inboxData); err != nil {
				return err
			}

			if app.JSONOutput {
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

	cmd.Flags().Bool("all", false, "Clear all items, including unprocessed")
	return cmd
}
