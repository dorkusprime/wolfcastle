// Package knowledge implements the knowledge command group for managing
// codebase knowledge files: the living, growing "what you need to know"
// documents that accumulate across tasks.
package knowledge

import (
	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/spf13/cobra"
)

// Register creates the knowledge command tree and attaches it to rootCmd.
func Register(app *cmdutil.App, rootCmd *cobra.Command) {
	knowledgeCmd := &cobra.Command{
		Use:   "knowledge",
		Short: "Manage codebase knowledge files",
		Long: `Knowledge files capture the informal, accumulating wisdom that
developers build by working in a codebase: build quirks, undocumented
conventions, things that look wrong but are intentional, test patterns
that work well, dependencies between modules that aren't obvious.

Examples:
  wolfcastle knowledge add "make test runs with -short; use make test-integration for full suite"
  wolfcastle knowledge show
  wolfcastle knowledge edit`,
	}

	knowledgeCmd.AddCommand(newAddCmd(app))
	knowledgeCmd.AddCommand(newShowCmd(app))
	knowledgeCmd.AddCommand(newEditCmd(app))
	knowledgeCmd.AddCommand(newPruneCmd(app))

	knowledgeCmd.GroupID = "docs"
	rootCmd.AddCommand(knowledgeCmd)
}
