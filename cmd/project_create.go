package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/project"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

var projectCreateCmd = &cobra.Command{
	Use:   "create [name]",
	Short: "Create a new project or sub-project",
	Long: `Creates a new project node in the work tree.
Without --node, creates a root-level project.
With --node, creates a child under the specified parent.

Examples:
  wolfcastle project create "auth-system"
  wolfcastle project create --type orchestrator "auth-system"
  wolfcastle project create --node auth-system "login-flow"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := requireResolver(); err != nil {
			return err
		}
		name := args[0]
		parentNode, _ := cmd.Flags().GetString("node")
		nodeType, _ := cmd.Flags().GetString("type")

		// Validate slug from name
		slug := tree.ToSlug(name)
		if err := tree.ValidateSlug(slug); err != nil {
			return fmt.Errorf("invalid project name: %w", err)
		}

		// Determine node type
		var nt state.NodeType
		switch nodeType {
		case "leaf":
			nt = state.NodeLeaf
		case "orchestrator":
			nt = state.NodeOrchestrator
		default:
			return fmt.Errorf("invalid node type %q: must be 'leaf' or 'orchestrator'", nodeType)
		}

		// Load root index
		idx, err := resolver.LoadRootIndex()
		if err != nil {
			return fmt.Errorf("loading root index: %w", err)
		}

		// Validate parent exists if specified; auto-promote leaf → orchestrator
		if parentNode != "" {
			parentEntry, ok := idx.Nodes[parentNode]
			if !ok {
				return fmt.Errorf("parent node %q not found", parentNode)
			}
			if parentEntry.Type == state.NodeLeaf {
				// Auto-promote: convert leaf parent to orchestrator (decomposition)
				parentParsed, err := tree.ParseAddress(parentNode)
				if err != nil {
					return fmt.Errorf("invalid parent address: %w", err)
				}
				parentDir := filepath.Join(resolver.ProjectsDir(), filepath.Join(parentParsed.Parts...))
				parentState, err := state.LoadNodeState(filepath.Join(parentDir, "state.json"))
				if err != nil {
					return fmt.Errorf("loading parent state for promotion: %w", err)
				}
				// Only auto-promote if the leaf has no tasks (per tree-addressing spec)
				nonAuditTasks := 0
				for _, t := range parentState.Tasks {
					if !t.IsAudit {
						nonAuditTasks++
					}
				}
				if nonAuditTasks > 0 {
					return fmt.Errorf("cannot create child under leaf %q: it has %d existing task(s) — remove tasks before decomposing", parentNode, nonAuditTasks)
				}
				parentState.Type = state.NodeOrchestrator
				parentState.Tasks = nil // orchestrators don't have tasks
				if err := state.SaveNodeState(filepath.Join(parentDir, "state.json"), parentState); err != nil {
					return fmt.Errorf("saving promoted parent state: %w", err)
				}
				parentEntry.Type = state.NodeOrchestrator
				idx.Nodes[parentNode] = parentEntry
			}
		}

		// Create the project
		ns, addr, err := project.CreateProject(idx, parentNode, slug, name, nt)
		if err != nil {
			return err
		}

		// Write node state
		addrParsed, err := tree.ParseAddress(addr)
		if err != nil {
			return fmt.Errorf("invalid node address: %w", err)
		}
		nodeDir := filepath.Join(resolver.ProjectsDir(), filepath.Join(addrParsed.Parts...))
		if err := os.MkdirAll(nodeDir, 0755); err != nil {
			return fmt.Errorf("creating node directory: %w", err)
		}
		if err := state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns); err != nil {
			return fmt.Errorf("saving node state: %w", err)
		}

		// Write project description Markdown (in the node's own directory)
		if parentNode != "" {
			descPath := filepath.Join(nodeDir, slug+".md")
			if err := os.WriteFile(descPath, []byte("# "+name+"\n\nProject description goes here.\n"), 0644); err != nil {
				return fmt.Errorf("writing project description: %w", err)
			}

			// Update parent node state to include child ref
			parentParsed2, _ := tree.ParseAddress(parentNode)
			parentDir := filepath.Join(resolver.ProjectsDir(), filepath.Join(parentParsed2.Parts...))
			parentState, err := state.LoadNodeState(filepath.Join(parentDir, "state.json"))
			if err == nil {
				parentState.Children = append(parentState.Children, state.ChildRef{
					ID:      slug,
					Address: addr,
					State:   state.StatusNotStarted,
				})
				if err := state.SaveNodeState(filepath.Join(parentDir, "state.json"), parentState); err != nil {
					return fmt.Errorf("saving parent state: %w", err)
				}
			}
		} else {
			descPath := filepath.Join(nodeDir, slug+".md")
			if err := os.WriteFile(descPath, []byte("# "+name+"\n\nProject description goes here.\n"), 0644); err != nil {
				return fmt.Errorf("writing project description: %w", err)
			}
		}

		// Update root index metadata for the first root-level project
		if parentNode == "" && idx.RootID == "" {
			idx.RootID = slug
			idx.RootName = name
			idx.RootState = state.StatusNotStarted
		}

		// Save updated root index
		if err := state.SaveRootIndex(resolver.RootIndexPath(), idx); err != nil {
			return fmt.Errorf("saving root index: %w", err)
		}

		if jsonOutput {
			output.Print(output.Ok("project_create", map[string]string{
				"address": addr,
				"type":    string(nt),
				"name":    name,
			}))
		} else {
			output.PrintHuman("Created %s project: %s", nt, addr)
		}

		// Run overlap advisory if enabled (ADR-027)
		if cfg != nil && cfg.OverlapAdvisory.Enabled {
			checkOverlap(name, "# "+name+"\n\nProject description goes here.")
		}

		return nil
	},
}

func init() {
	projectCreateCmd.Flags().String("node", "", "Parent node address (omit for root-level)")
	projectCreateCmd.Flags().String("type", "leaf", "Node type: leaf or orchestrator")
	projectCmd.AddCommand(projectCreateCmd)
}
