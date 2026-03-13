package cmd

import (
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

var navigateCmd = &cobra.Command{
	Use:   "navigate",
	Short: "Find the next actionable task via depth-first traversal",
	Long:  "Returns the next task to work on. Does NOT claim the task.",
	RunE: func(cmd *cobra.Command, args []string) error {
		scopeNode, _ := cmd.Flags().GetString("node")

		idx, err := resolver.LoadRootIndex()
		if err != nil {
			return err
		}

		result, err := state.FindNextTask(idx, scopeNode, func(addr string) (*state.NodeState, error) {
			a := tree.MustParse(addr)
			return state.LoadNodeState(filepath.Join(resolver.ProjectsDir(), filepath.Join(a.Parts...), "state.json"))
		})
		if err != nil {
			return err
		}

		if jsonOutput {
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
