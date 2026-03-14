package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

// navigateCmd finds the next actionable task via depth-first traversal.
var navigateCmd = &cobra.Command{
	Use:   "navigate",
	Short: "Find the next actionable task via depth-first traversal",
	Long: `Returns the next task to work on via depth-first traversal of the project tree.

Does NOT claim the task. Use 'wolfcastle task claim' to claim it.
Optionally scope navigation to a subtree with --node.

Examples:
  wolfcastle navigate
  wolfcastle navigate --node auth-system
  wolfcastle navigate --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := app.RequireResolver(); err != nil {
			return err
		}
		scopeNode, _ := cmd.Flags().GetString("node")

		idx, err := app.Resolver.LoadRootIndex()
		if err != nil {
			return err
		}

		result, err := state.FindNextTask(idx, scopeNode, func(addr string) (*state.NodeState, error) {
			a, err := tree.ParseAddress(addr)
			if err != nil {
				return nil, fmt.Errorf("parsing address %q: %w", addr, err)
			}
			return state.LoadNodeState(filepath.Join(app.Resolver.ProjectsDir(), filepath.Join(a.Parts...), "state.json"))
		})
		if err != nil {
			return err
		}

		if app.JSONOutput {
			output.Print(output.Ok("navigate", result))
		} else {
			if result.Found {
				output.PrintHuman("Next task: %s/%s", result.NodeAddress, result.TaskID)
				output.PrintHuman("  %s", result.Description)
			} else {
				output.PrintHuman("No actionable tasks: %s", result.Reason)
			}
		}
		return nil
	},
}

func init() {
	navigateCmd.Flags().String("node", "", "Scope navigation to a subtree")
	rootCmd.AddCommand(navigateCmd)
}
