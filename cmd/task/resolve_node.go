package task

import (
	"fmt"

	"github.com/spf13/cobra"
)

// resolveNode extracts the task address from either a positional argument
// (args[idx]) or the --node flag. If both are provided, it returns an error.
// If neither is provided, it returns an error prompting the user.
//
// This enables two equivalent invocation styles:
//
//	wolfcastle task claim my-project/task-0001          # positional
//	wolfcastle task claim --node my-project/task-0001   # flag
func resolveNode(cmd *cobra.Command, args []string, argIdx int) (string, error) {
	nodeFlag, _ := cmd.Flags().GetString("node")
	flagChanged := cmd.Flags().Changed("node")

	var positional string
	if argIdx < len(args) {
		positional = args[argIdx]
	}

	if flagChanged && positional != "" {
		return "", fmt.Errorf("specify the task address as a positional argument or with --node, not both")
	}
	if flagChanged {
		if nodeFlag == "" {
			return "", fmt.Errorf("--node value cannot be empty: specify the task address (e.g. my-project/task-1)")
		}
		return nodeFlag, nil
	}
	if positional != "" {
		return positional, nil
	}
	return "", fmt.Errorf("task address required: provide as argument or with --node (e.g. my-project/task-1)")
}
