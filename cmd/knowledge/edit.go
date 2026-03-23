package knowledge

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/knowledge"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/spf13/cobra"
)

func newEditCmd(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open the codebase knowledge file in your editor",
		Long: `Opens the current namespace's knowledge file in $EDITOR (falls back to vi).
Creates the file with an empty template if it doesn't exist yet.

Examples:
  wolfcastle knowledge edit
  EDITOR=nano wolfcastle knowledge edit`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireIdentity(); err != nil {
				return err
			}

			path := knowledge.FilePath(app.Config.Root(), app.Identity.Namespace)

			if err := ensureKnowledgeFile(path); err != nil {
				return err
			}

			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}

			if app.JSON {
				output.Print(output.Ok("knowledge_edit", map[string]string{
					"path":   path,
					"editor": editor,
				}))
				return nil
			}

			c := exec.Command(editor, path)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr

			if err := c.Run(); err != nil {
				return fmt.Errorf("editor exited with error: %w", err)
			}
			return nil
		},
	}
}

// ensureKnowledgeFile creates the knowledge file with an empty template
// if it doesn't already exist.
func ensureKnowledgeFile(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating knowledge directory: %w", err)
	}
	template := "# Codebase Knowledge\n\n"
	if err := os.WriteFile(path, []byte(template), 0o644); err != nil {
		return fmt.Errorf("creating knowledge file: %w", err)
	}
	return nil
}
