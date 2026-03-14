package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/project"
	"github.com/spf13/cobra"
)

// initCmd scaffolds the .wolfcastle directory in the current working directory.
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a Wolfcastle project in the current directory",
	Long: `Creates the .wolfcastle/ directory with default configuration, base prompts,
and engineer identity in the current working directory.

This is typically the first command you run in a new repository.

Examples:
  cd my-repo && wolfcastle init
  wolfcastle init --force`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		wcDir := filepath.Join(cwd, ".wolfcastle")

		force, _ := cmd.Flags().GetBool("force")

		// Check if already initialized
		if _, err := os.Stat(wcDir); err == nil {
			if !force {
				// Per spec: print message and exit 0
				if app.JSONOutput {
					output.Print(output.Ok("init", map[string]string{
						"path":    wcDir,
						"status":  "already_initialized",
						"message": "Wolfcastle already initialized in .wolfcastle/. Use --force to reinitialize.",
					}))
				} else {
					output.PrintHuman("Wolfcastle already initialized in %s. Use --force to reinitialize.", wcDir)
				}
				return nil
			}

			// Force mode: re-scaffold base/ and refresh identity
			if err := project.ReScaffold(wcDir); err != nil {
				return fmt.Errorf("re-scaffold failed: %w", err)
			}

			if app.JSONOutput {
				output.Print(output.Ok("init", map[string]string{
					"path":   wcDir,
					"status": "reinitialized",
				}))
			} else {
				output.PrintHuman("Reinitialized Wolfcastle project in %s", wcDir)
			}
			return nil
		}

		if err := project.Scaffold(wcDir); err != nil {
			return fmt.Errorf("scaffold failed: %w", err)
		}

		if app.JSONOutput {
			output.Print(output.Ok("init", map[string]string{
				"path": wcDir,
			}))
		} else {
			output.PrintHuman("Initialized Wolfcastle in %s", wcDir)
			output.PrintHuman("  config.json        team-shared configuration")
			output.PrintHuman("  config.local.json  your identity (gitignored)")
			output.PrintHuman("  base/              default prompts and rules")
			output.PrintHuman("")
			output.PrintHuman("Next: create a project with 'wolfcastle project create \"name\"'")
		}
		return nil
	},
}

func init() {
	initCmd.Flags().Bool("force", false, "Re-scaffold base/ templates and refresh identity")
	rootCmd.AddCommand(initCmd)
}
