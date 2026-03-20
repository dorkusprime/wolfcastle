package task

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

func newDeliverableCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deliverable [path]",
		Short: "Append a deliverable to an existing task",
		Long: `Declares a file the task is expected to produce. The daemon verifies
all deliverables exist before accepting WOLFCASTLE_COMPLETE. Missing
deliverables count as a failure and the model is re-invoked.

Examples:
  wolfcastle task deliverable "docs/pos-research.md" --node pizza-docs/task-0001
  wolfcastle task deliverable "src/api/handler.go" --node my-project/task-0002`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireIdentity(); err != nil {
				return err
			}
			delivPath := args[0]
			if strings.TrimSpace(delivPath) == "" {
				return fmt.Errorf("deliverable path cannot be empty")
			}
			if filepath.IsAbs(delivPath) {
				return fmt.Errorf("deliverable path must be relative, got absolute path %q", delivPath)
			}
			if strings.Contains(delivPath, "..") {
				return fmt.Errorf("deliverable path must not contain '..' traversal components")
			}
			nodeFlag, _ := cmd.Flags().GetString("node")
			if nodeFlag == "" {
				return fmt.Errorf("--node is required: specify the task address (e.g. my-project/task-1)")
			}

			nodeAddr, taskID, err := tree.SplitTaskAddress(nodeFlag)
			if err != nil {
				return fmt.Errorf("--node must be a task address: %w", err)
			}

			if err := app.State.MutateNode(nodeAddr, func(ns *state.NodeState) error {
				for i := range ns.Tasks {
					if ns.Tasks[i].ID == taskID {
						// Avoid duplicates
						for _, existing := range ns.Tasks[i].Deliverables {
							if existing == delivPath {
								return nil
							}
						}
						ns.Tasks[i].Deliverables = append(ns.Tasks[i].Deliverables, delivPath)
						return nil
					}
				}
				return fmt.Errorf("task %s not found in node %s", taskID, nodeAddr)
			}); err != nil {
				return err
			}

			// Warn if the declared path doesn't exist yet (may be a typo)
			repoDir := filepath.Dir(app.Config.Root())
			if !isGlobPath(delivPath) {
				fullPath := filepath.Join(repoDir, delivPath)
				if _, statErr := os.Stat(fullPath); os.IsNotExist(statErr) {
					base := filepath.Base(delivPath)
					matches := findFilesByName(repoDir, base)
					if len(matches) > 0 {
						output.PrintHuman("Warning: '%s' does not exist. Did you mean one of: %v", delivPath, matches)
					} else {
						output.PrintHuman("Warning: '%s' does not exist yet", delivPath)
					}
				}
			}

			if app.JSON {
				output.Print(output.Ok("task_deliverable", map[string]string{
					"address":     nodeFlag,
					"task_id":     taskID,
					"deliverable": delivPath,
				}))
			} else {
				output.PrintHuman("Added deliverable %s to %s", delivPath, nodeFlag)
			}
			return nil
		},
	}

	cmd.Flags().String("node", "", "Task address: node-path/task-id (required)")
	_ = cmd.MarkFlagRequired("node")
	return cmd
}

// isGlobPath reports whether a path contains glob metacharacters.
func isGlobPath(path string) bool {
	return strings.ContainsAny(path, "*?[")
}

// findFilesByName walks the repo looking for files with the given base name.
// Returns up to 5 relative paths.
func findFilesByName(repoDir, baseName string) []string {
	var matches []string
	_ = filepath.Walk(repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			// Skip .wolfcastle and .git directories
			if info != nil && info.IsDir() && (info.Name() == ".wolfcastle" || info.Name() == ".git") {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Name() == baseName {
			rel, _ := filepath.Rel(repoDir, path)
			if rel != "" {
				matches = append(matches, rel)
			}
		}
		if len(matches) >= 5 {
			return filepath.SkipAll
		}
		return nil
	})
	return matches
}
