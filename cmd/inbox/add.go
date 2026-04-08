package inbox

import (
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

func newAddCmd(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "add [idea]",
		Short: "Throw an idea at Wolfcastle",
		Long: `Drops a raw idea into the inbox. Triage happens later.

Examples:
  wolfcastle inbox add "refactor the auth middleware"
  wolfcastle inbox add "investigate flaky test in CI"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := resolveInstance(cmd, app); err != nil {
				return err
			}
			if len(args) < 1 {
				return fmt.Errorf("missing required argument: <idea>")
			}
			if err := app.RequireIdentity(); err != nil {
				return err
			}
			text := args[0]
			if strings.TrimSpace(text) == "" {
				return fmt.Errorf("empty text. Give Wolfcastle something to work with")
			}

			inboxPath := app.State.InboxPath()

			item := state.InboxItem{
				Timestamp: app.Clock.Now().Format("2006-01-02T15:04:05Z07:00"),
				Text:      text,
				Status:    state.InboxNew,
			}

			if err := state.InboxAppend(inboxPath, item); err != nil {
				return fmt.Errorf("writing inbox: %w", err)
			}

			if app.JSON {
				output.Print(output.Ok("inbox_add", map[string]string{
					"text":      text,
					"timestamp": item.Timestamp,
				}))
			} else {
				output.PrintHuman("Inbox: %s", text)
			}
			return nil
		},
	}
}
