package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/project"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a Wolfcastle project in the current directory",
	Long: `Creates the .wolfcastle/ directory with default configuration, base prompts,
and engineer identity in the current working directory.

This is typically the first command you run in a new repository.

Examples:
  cd my-repo && wolfcastle init`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return err
		}
		wcDir := filepath.Join(cwd, ".wolfcastle")

		// Check if already initialized
		if _, err := os.Stat(wcDir); err == nil {
			return fmt.Errorf(".wolfcastle already exists — use 'wolfcastle update' to refresh base/")
		}

		if err := project.Scaffold(wcDir); err != nil {
			return fmt.Errorf("scaffold failed: %w", err)
		}

		if jsonOutput {
			output.Print(output.Ok("init", map[string]string{
				"path": wcDir,
			}))
		} else {
			output.PrintHuman("Initialized Wolfcastle in %s", wcDir)
			output.PrintHuman("  config.json        — team-shared configuration")
			output.PrintHuman("  config.local.json  — your identity (gitignored)")
			output.PrintHuman("  base/              — default prompts and rules")
			output.PrintHuman("")
			output.PrintHuman("Next: create a project with 'wolfcastle project create \"name\"'")
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
