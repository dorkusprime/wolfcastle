package knowledge

import (
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/knowledge"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

func newAddCmd(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "add [entry]",
		Short: "Append an entry to the codebase knowledge file",
		Long: `Adds a knowledge entry to the current namespace's knowledge file.
The entry is checked against the configured token budget before writing.
If the entry would push the file over budget, the command fails with
a clear error. Nothing is silently truncated.

Examples:
  wolfcastle knowledge add "the integration tests require docker compose up before running"
  wolfcastle knowledge add "state.Store serializes mutations through a file lock"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("missing required argument: <entry>")
			}
			if err := app.RequireIdentity(); err != nil {
				return err
			}
			entry := args[0]
			if strings.TrimSpace(entry) == "" {
				return fmt.Errorf("empty entry. Knowledge worth recording shouldn't be blank")
			}

			cfg, err := app.Config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			wolfcastleDir := app.Config.Root()
			namespace := app.Identity.Namespace
			maxTokens := cfg.Knowledge.MaxTokens

			if err := knowledge.CheckBudget(wolfcastleDir, namespace, maxTokens, entry); err != nil {
				return err
			}

			if err := knowledge.Append(wolfcastleDir, namespace, entry); err != nil {
				return err
			}

			path := knowledge.FilePath(wolfcastleDir, namespace)

			if app.JSON {
				output.Print(output.Ok("knowledge_add", map[string]string{
					"entry":     entry,
					"namespace": namespace,
					"path":      path,
				}))
			} else {
				output.PrintHuman("Knowledge: %s", entry)
			}
			return nil
		},
	}
}
