package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/project"
	"github.com/spf13/cobra"
)

// initCmd scaffolds the .wolfcastle directory in the current working directory.
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Claim a directory for Wolfcastle",
	Long: `Creates .wolfcastle/ with default config, base prompts, and identity.
This is where it begins.

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
				if app.JSON {
					output.Print(output.Ok("init", map[string]string{
						"path":    wcDir,
						"status":  "already_initialized",
						"message": "Already initialized. Use --force to reinitialize.",
					}))
				} else {
					output.PrintHuman("Already initialized in %s. Use --force to reinitialize.", wcDir)
				}
				return nil
			}

			// Force mode: re-scaffold base/ and refresh identity
			svc := newScaffoldService(wcDir)
			if err := svc.Reinit(); err != nil {
				return fmt.Errorf("re-scaffold failed: %w", err)
			}

			if app.JSON {
				output.Print(output.Ok("init", map[string]string{
					"path":   wcDir,
					"status": "reinitialized",
				}))
			} else {
				output.PrintHuman("Reinitialized in %s", wcDir)
			}
			return nil
		}

		svc := newScaffoldService(wcDir)
		if err := svc.Init(config.DetectIdentity()); err != nil {
			return fmt.Errorf("scaffold failed: %w", err)
		}

		if app.JSON {
			output.Print(output.Ok("init", map[string]string{
				"path": wcDir,
			}))
		} else {
			output.PrintHuman("Wolfcastle deployed in %s", wcDir)
			output.PrintHuman("  system/base/config.json     defaults (regenerated on update)")
			output.PrintHuman("  system/custom/config.json   team overrides (committed)")
			output.PrintHuman("  system/local/config.json    your identity (gitignored)")
			output.PrintHuman("  system/base/                prompts and rules")
			output.PrintHuman("")
			output.PrintHuman("Next: wolfcastle project create \"target-name\"")
		}
		return nil
	},
}

func init() {
	initCmd.Flags().Bool("force", false, "Re-scaffold system/base/ templates and refresh identity")
	rootCmd.AddCommand(initCmd)
}

// newScaffoldService constructs a ScaffoldService with real repositories
// rooted at the given .wolfcastle directory.
func newScaffoldService(wcDir string) *project.ScaffoldService {
	cfgRepo := config.NewConfigRepository(wcDir)
	promptRepo := pipeline.NewPromptRepository(wcDir)
	return project.NewScaffoldService(cfgRepo, promptRepo, nil, wcDir)
}
