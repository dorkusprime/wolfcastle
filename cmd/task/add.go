package task

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

func newAddCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [title]",
		Short: "Add a task to a leaf node",
		Long: `Adds a new task to a leaf node's task list.

The positional argument is the task title (short name). Use --body or --stdin
to provide a detailed description. The task is created in the not_started state.
Tasks are executed in order by the daemon. Use 'wolfcastle navigate' to find the
next actionable task.

Examples:
  wolfcastle task add --node my-project "implement the API endpoint"
  wolfcastle task add --node auth/login "add rate limiting" --body "Details about implementation..."
  echo "body text" | wolfcastle task add --node my-project "title" --stdin`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireResolver(); err != nil {
				return err
			}
			title := args[0]
			if strings.TrimSpace(title) == "" {
				return fmt.Errorf("task title cannot be empty")
			}
			nodeAddr, _ := cmd.Flags().GetString("node")
			if nodeAddr == "" {
				return fmt.Errorf("--node is required: specify the target leaf node address")
			}
			body, _ := cmd.Flags().GetString("body")
			useStdin, _ := cmd.Flags().GetBool("stdin")

			if useStdin {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				body = string(data)
			}

			addr, err := tree.ParseAddress(nodeAddr)
			if err != nil {
				return fmt.Errorf("invalid node address: %w", err)
			}
			nsPath := filepath.Join(app.Resolver.ProjectsDir(), filepath.Join(addr.Parts...))

			var task *state.Task
			if err := app.Store.MutateNode(nodeAddr, func(ns *state.NodeState) error {
				var addErr error
				task, addErr = state.TaskAdd(ns, title)
				if addErr != nil {
					return addErr
				}
				task.Title = title
				for i := range ns.Tasks {
					if ns.Tasks[i].ID == task.ID {
						ns.Tasks[i].Title = title
						break
					}
				}
				return nil
			}); err != nil {
				return err
			}

			// Write task markdown file
			writeTaskMD(nsPath, task.ID, title, body)

			taskAddr := nodeAddr + "/" + task.ID
			if app.JSONOutput {
				output.Print(output.Ok("task_add", map[string]string{
					"address":     taskAddr,
					"task_id":     task.ID,
					"description": title,
					"state":       string(task.State),
				}))
			} else {
				output.PrintHuman("Added task %s: %s", taskAddr, title)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Target leaf node address (required)")
	cmd.Flags().String("body", "", "Detailed task description/body")
	cmd.Flags().Bool("stdin", false, "Read task body from stdin")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}

// writeTaskMD writes a {taskID}.md file in the node directory.
func writeTaskMD(nodeDir, taskID, title, body string) {
	var sb strings.Builder
	sb.WriteString("# " + title + "\n")
	if strings.TrimSpace(body) != "" {
		sb.WriteString("\n" + body + "\n")
	}
	// Best-effort write; errors here are non-fatal.
	_ = os.WriteFile(filepath.Join(nodeDir, taskID+".md"), []byte(sb.String()), 0644)
}
