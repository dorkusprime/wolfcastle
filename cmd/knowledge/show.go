package knowledge

import (
	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/knowledge"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

func newShowCmd(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Display the codebase knowledge file",
		Long: `Reads and prints the current namespace's knowledge file.
If no knowledge has been recorded yet, says so.

Examples:
  wolfcastle knowledge show
  wolfcastle knowledge show --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireIdentity(); err != nil {
				return err
			}

			namespace := app.Identity.Namespace
			wolfcastleDir := app.Config.Root()

			content, err := knowledge.Read(wolfcastleDir, namespace)
			if err != nil {
				return err
			}

			if app.JSON {
				output.Print(output.Ok("knowledge_show", map[string]any{
					"namespace":   namespace,
					"content":     content,
					"path":        knowledge.FilePath(wolfcastleDir, namespace),
					"token_count": knowledge.TokenCount(content),
				}))
			} else {
				if content == "" {
					output.PrintHuman("No codebase knowledge recorded yet for %s.", namespace)
				} else {
					output.PrintHuman("%s", content)
				}
			}
			return nil
		},
	}
}
