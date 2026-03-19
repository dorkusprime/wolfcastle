package cmd

import (
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

// navigateCmd finds the next actionable task via depth-first traversal.
var navigateCmd = &cobra.Command{
	Use:   "navigate",
	Short: "Acquire the next target",
	Long: `Searches the project tree depth-first and returns the next task to
eliminate. Does not claim it. Use 'wolfcastle task claim' for that.

Examples:
  wolfcastle navigate
  wolfcastle navigate --node auth-system
  wolfcastle navigate --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := app.RequireIdentity(); err != nil {
			return err
		}
		scopeNode, _ := cmd.Flags().GetString("node")

		idx, err := app.State.ReadIndex()
		if err != nil {
			return err
		}

		result, err := state.FindNextTask(idx, scopeNode, func(addr string) (*state.NodeState, error) {
			return app.State.ReadNode(addr)
		})
		if err != nil {
			return err
		}

		if app.JSONOutput {
			output.Print(output.Ok("navigate", result))
		} else {
			if result.Found {
				output.PrintHuman("Target acquired: %s/%s", result.NodeAddress, result.TaskID)
				output.PrintHuman("  %s", result.Description)
			} else {
				output.PrintHuman("No targets: %s", result.Reason)
			}
		}
		return nil
	},
}

func init() {
	navigateCmd.Flags().String("node", "", "Scope navigation to a subtree")
	rootCmd.AddCommand(navigateCmd)
}
