package task

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

func newAddCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [title]",
		Short: "Add a target to a leaf node",
		Long: `Creates a new task on a leaf node. The daemon will find it and destroy it
in order. Use --body or --stdin for a detailed description.

Examples:
  wolfcastle task add --node my-project "implement the API endpoint"
  wolfcastle task add --node auth/login "add rate limiting" --body "Details about implementation..."
  echo "body text" | wolfcastle task add --node my-project "title" --stdin`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("missing required argument: <title>")
			}
			if err := app.RequireIdentity(); err != nil {
				return err
			}
			title := args[0]
			if strings.TrimSpace(title) == "" {
				return fmt.Errorf("task title cannot be empty. Name the target")
			}
			nodeAddr, _ := cmd.Flags().GetString("node")
			if nodeAddr == "" {
				return fmt.Errorf("--node is required: specify the target leaf node address")
			}
			body, _ := cmd.Flags().GetString("body")
			useStdin, _ := cmd.Flags().GetBool("stdin")
			deliverables, _ := cmd.Flags().GetStringArray("deliverable")
			taskType, _ := cmd.Flags().GetString("type")
			taskClass, _ := cmd.Flags().GetString("class")
			constraints, _ := cmd.Flags().GetStringArray("constraint")
			acceptance, _ := cmd.Flags().GetStringArray("acceptance")
			references, _ := cmd.Flags().GetStringArray("reference")
			integration, _ := cmd.Flags().GetString("integration")

			if useStdin {
				data, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				body = string(data)
			}

			// Validate task class if provided
			if taskClass != "" {
				valid := app.Classes.List()
				if len(valid) == 0 {
					// Classes not loaded into repository; fall back to config keys.
					cfg, err := app.Config.Load()
					if err == nil && len(cfg.TaskClasses) > 0 {
						valid = make([]string, 0, len(cfg.TaskClasses))
						for k := range cfg.TaskClasses {
							valid = append(valid, k)
						}
						sort.Strings(valid)
					}
				}
				found := false
				for _, k := range valid {
					if k == taskClass {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("unknown task class %q; valid classes: %v", taskClass, valid)
				}
			}

			// Validate task type if provided
			if taskType != "" {
				validTypes := map[string]bool{
					"discovery": true, "spec": true, "adr": true,
					"implementation": true, "integration": true, "cleanup": true,
				}
				if !validTypes[taskType] {
					return fmt.Errorf("invalid task type %q: must be one of discovery, spec, adr, implementation, integration, cleanup", taskType)
				}
			}

			addr, err := tree.ParseAddress(nodeAddr)
			if err != nil {
				return fmt.Errorf("invalid node address: %w", err)
			}
			nsPath := filepath.Join(app.State.Dir(), filepath.Join(addr.Parts...))

			parentTask, _ := cmd.Flags().GetString("parent")

			var task *state.Task
			if err := app.State.MutateNode(nodeAddr, func(ns *state.NodeState) error {
				var addErr error
				if parentTask != "" {
					task, addErr = state.TaskAddChild(ns, parentTask, title)
				} else {
					task, addErr = state.TaskAdd(ns, title)
				}
				if addErr != nil {
					return addErr
				}
				task.Title = title
				for i := range ns.Tasks {
					if ns.Tasks[i].ID == task.ID {
						ns.Tasks[i].Title = title
						if len(deliverables) > 0 {
							ns.Tasks[i].Deliverables = deliverables
						}
						if body != "" {
							ns.Tasks[i].Body = body
						}
						if taskType != "" {
							ns.Tasks[i].TaskType = taskType
						}
						if taskClass != "" {
							ns.Tasks[i].Class = taskClass
						}
						if len(constraints) > 0 {
							ns.Tasks[i].Constraints = constraints
						}
						if len(acceptance) > 0 {
							ns.Tasks[i].AcceptanceCriteria = acceptance
						}
						if len(references) > 0 {
							ns.Tasks[i].References = references
						}
						if integration != "" {
							ns.Tasks[i].Integration = integration
						}
						break
					}
				}
				return nil
			}); err != nil {
				return err
			}

			// Write task markdown file
			writeTaskMD(app.Prompts, nsPath, task.ID, title, body)

			taskAddr := nodeAddr + "/" + task.ID
			if app.JSON {
				result := map[string]any{
					"address":     taskAddr,
					"task_id":     task.ID,
					"description": title,
					"state":       string(task.State),
				}
				if len(deliverables) > 0 {
					result["deliverables"] = deliverables
				}
				output.Print(output.Ok("task_add", result))
			} else {
				output.PrintHuman("Added task %s: %s", taskAddr, title)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Target leaf node address (required)")
	cmd.Flags().String("body", "", "Detailed task description/body")
	cmd.Flags().Bool("stdin", false, "Read task body from stdin")
	cmd.Flags().StringArray("deliverable", nil, "Expected output file (repeatable)")
	cmd.Flags().String("type", "", "Task type: discovery, spec, adr, implementation, integration, cleanup")
	cmd.Flags().String("class", "", "Task class override (e.g., coding/go)")
	cmd.Flags().StringArray("constraint", nil, "Constraint: what not to do (repeatable)")
	cmd.Flags().StringArray("acceptance", nil, "Acceptance criterion (repeatable)")
	cmd.Flags().StringArray("reference", nil, "Reference material path (repeatable)")
	cmd.Flags().String("integration", "", "How this task connects to other work")
	cmd.Flags().String("parent", "", "Parent task ID for hierarchical decomposition (e.g., task-0001)")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}

// writeTaskMD writes a {taskID}.md file in the node directory by resolving
// the task template through the three-tier system.
func writeTaskMD(prompts *pipeline.PromptRepository, nodeDir, taskID, title, body string) {
	// Whitespace-only bodies are treated as empty, matching the original
	// strings.Builder behavior.
	tmplBody := body
	if strings.TrimSpace(body) == "" {
		tmplBody = ""
	}
	// Best-effort write; errors here are non-fatal.
	_ = prompts.RenderToFile("artifacts/task.md", pipeline.TaskData{
		Title: title,
		Body:  tmplBody,
	}, filepath.Join(nodeDir, taskID+".md"))
}
