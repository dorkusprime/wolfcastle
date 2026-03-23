package knowledge

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/knowledge"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

func newPruneCmd(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "prune",
		Short: "Review and consolidate the codebase knowledge file",
		Long: `Opens the knowledge file in $EDITOR for manual pruning. Remove stale entries,
consolidate related ones, and bring the file under its token budget.

After editing, reports the new token count relative to the configured budget.

When run by the daemon's maintenance task with --json, operates non-interactively
and just reports the current token count and budget status.

Examples:
  wolfcastle knowledge prune
  wolfcastle knowledge prune --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireIdentity(); err != nil {
				return err
			}

			cfg, err := app.Config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			namespace := app.Identity.Namespace
			wolfcastleDir := app.Config.Root()
			path := knowledge.FilePath(wolfcastleDir, namespace)
			maxTokens := cfg.Knowledge.MaxTokens

			// In JSON mode (daemon maintenance), report status without opening editor.
			if app.JSON {
				content, readErr := knowledge.Read(wolfcastleDir, namespace)
				if readErr != nil {
					return readErr
				}
				count := knowledge.TokenCount(content)
				output.Print(output.Ok("knowledge_prune", map[string]any{
					"namespace":   namespace,
					"path":        path,
					"token_count": count,
					"max_tokens":  maxTokens,
					"over_budget": count > maxTokens,
				}))
				return nil
			}

			if err := ensureKnowledgeFile(path); err != nil {
				return err
			}

			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}

			c := exec.Command(editor, path)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr

			if err := c.Run(); err != nil {
				return fmt.Errorf("editor exited with error: %w", err)
			}

			// Report token count after editing.
			content, err := knowledge.Read(wolfcastleDir, namespace)
			if err != nil {
				return err
			}
			count := knowledge.TokenCount(content)
			if count > maxTokens {
				output.PrintHuman("Token count: %d/%d (still over budget)", count, maxTokens)
			} else {
				output.PrintHuman("Token count: %d/%d", count, maxTokens)
			}
			return nil
		},
	}
}
