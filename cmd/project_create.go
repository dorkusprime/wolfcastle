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
With --node, creates a child under the specified parent.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
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
			nt = state.NodeLeaf // default to leaf
		}

		// Load root index
		idx, err := resolver.LoadRootIndex()
		if err != nil {
			return fmt.Errorf("loading root index: %w", err)
		}

		// Validate parent exists if specified
		if parentNode != "" {
			if _, ok := idx.Nodes[parentNode]; !ok {
				return fmt.Errorf("parent node %q not found", parentNode)
			}
		}

		// Create the project
		ns, addr, err := project.CreateProject(idx, parentNode, slug, name, nt, nil)
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
			return err
		}
		if err := state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns); err != nil {
			return err
		}

		// Write project description Markdown
		if parentNode != "" {
			parentParsed, err := tree.ParseAddress(parentNode)
			if err != nil {
				return fmt.Errorf("invalid parent address: %w", err)
			}
			parentDir := filepath.Join(resolver.ProjectsDir(), filepath.Join(parentParsed.Parts...))
			descPath := filepath.Join(parentDir, slug+".md")
			os.WriteFile(descPath, []byte("# "+name+"\n\nProject description goes here.\n"), 0644)

			// Update parent node state to include child ref
			parentState, err := state.LoadNodeState(filepath.Join(parentDir, "state.json"))
			if err == nil {
				parentState.Children = append(parentState.Children, state.ChildRef{
					ID:      slug,
					Address: addr,
					State:   state.StatusNotStarted,
				})
				state.SaveNodeState(filepath.Join(parentDir, "state.json"), parentState)
			}
		} else {
			descPath := filepath.Join(resolver.ProjectsDir(), slug+".md")
			os.WriteFile(descPath, []byte("# "+name+"\n\nProject description goes here.\n"), 0644)
		}

		// Save updated root index
		if err := state.SaveRootIndex(resolver.RootIndexPath(), idx); err != nil {
			return err
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
