package project

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/project"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

func newCreateCmd(app *cmdutil.App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Designate a new target",
		Long: `Creates a project node in the work tree. Without --node, it lands at
the root. With --node, it nests under a parent. Leaf nodes hold tasks.
Orchestrators command their children.

Examples:
  wolfcastle project create "auth-system"
  wolfcastle project create --type orchestrator "auth-system"
  wolfcastle project create --node auth-system "login-flow"`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RequireResolver(); err != nil {
				return err
			}
			name := args[0]
			parentNode, _ := cmd.Flags().GetString("node")
			nodeType, _ := cmd.Flags().GetString("type")
			description, _ := cmd.Flags().GetString("description")
			scope, _ := cmd.Flags().GetString("scope")

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
				return fmt.Errorf("unknown node type %q: pick 'leaf' or 'orchestrator'", nodeType)
			}

			// All state mutations happen under a single lock hold to
			// prevent races with the daemon or other CLI commands.
			var addr string
			if err := app.Store.WithLock(func() error {
				// Load root index (raw, we hold the lock)
				idx, err := state.LoadRootIndex(app.Resolver.RootIndexPath())
				if err != nil {
					if !os.IsNotExist(err) {
						return fmt.Errorf("loading root index: %w", err)
					}
					idx = state.NewRootIndex()
				}

				// Validate parent exists if specified; auto-promote leaf → orchestrator
				if parentNode != "" {
					parentEntry, ok := idx.Nodes[parentNode]
					if !ok {
						return fmt.Errorf("parent node %q not found. Check your address", parentNode)
					}
					if parentEntry.Type == state.NodeLeaf {
						parentParsed, err := tree.ParseAddress(parentNode)
						if err != nil {
							return fmt.Errorf("invalid parent address: %w", err)
						}
						parentDir := filepath.Join(app.Resolver.ProjectsDir(), filepath.Join(parentParsed.Parts...))
						parentState, err := state.LoadNodeState(filepath.Join(parentDir, "state.json"))
						if err != nil {
							return fmt.Errorf("loading parent state for promotion: %w", err)
						}
						nonAuditTasks := 0
						for _, t := range parentState.Tasks {
							if !t.IsAudit {
								nonAuditTasks++
							}
						}
						if nonAuditTasks > 0 {
							return fmt.Errorf("cannot create child under leaf %q: it has %d existing task(s). Remove tasks before decomposing", parentNode, nonAuditTasks)
						}
						parentState.Type = state.NodeOrchestrator
						parentState.Tasks = nil
						if err := state.SaveNodeState(filepath.Join(parentDir, "state.json"), parentState); err != nil {
							return fmt.Errorf("saving promoted parent state: %w", err)
						}
						parentEntry.Type = state.NodeOrchestrator
						idx.Nodes[parentNode] = parentEntry
					}
				}

				// Create the project
				ns, createdAddr, err := project.CreateProject(idx, parentNode, slug, name, nt)
				if err != nil {
					return err
				}
				addr = createdAddr

				// Set scope and trigger planning if --scope provided
				if scope != "" && nt == state.NodeOrchestrator {
					ns.Scope = scope
					ns.NeedsPlanning = true
					ns.PlanningTrigger = "initial"
				}

				// Write node state (raw save, no nested lock)
				addrParsed, err := tree.ParseAddress(addr)
				if err != nil {
					return fmt.Errorf("invalid node address: %w", err)
				}
				nodeDir := filepath.Join(app.Resolver.ProjectsDir(), filepath.Join(addrParsed.Parts...))
				if err := os.MkdirAll(nodeDir, 0755); err != nil {
					return fmt.Errorf("creating node directory: %w", err)
				}
				if err := state.SaveNodeState(filepath.Join(nodeDir, "state.json"), ns); err != nil {
					return fmt.Errorf("saving node state: %w", err)
				}

				// Write audit.md for leaf nodes from embedded template
				if nt == state.NodeLeaf {
					project.WriteAuditTaskMD(nodeDir)
				}

				// Write project description Markdown
				descBody := description
				if descBody == "" {
					descBody = "Project description goes here."
				}
				descPath := filepath.Join(nodeDir, slug+".md")
				if err := os.WriteFile(descPath, []byte("# "+name+"\n\n"+descBody+"\n"), 0644); err != nil {
					return fmt.Errorf("writing project description: %w", err)
				}

				// Update parent node state to include child ref
				if parentNode != "" {
					parentParsed2, _ := tree.ParseAddress(parentNode)
					parentDir := filepath.Join(app.Resolver.ProjectsDir(), filepath.Join(parentParsed2.Parts...))
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
				}

				// Update root index metadata for the first root-level project
				if parentNode == "" && idx.RootID == "" {
					idx.RootID = slug
					idx.RootName = name
					idx.RootState = state.StatusNotStarted
				}

				return state.SaveRootIndex(app.Resolver.RootIndexPath(), idx)
			}); err != nil {
				return err
			}

			if app.JSONOutput {
				output.Print(output.Ok("project_create", map[string]string{
					"address": addr,
					"type":    string(nt),
					"name":    name,
				}))
			} else {
				output.PrintHuman("Created %s project: %s", nt, addr)
			}

			// Run overlap advisory if enabled (ADR-027)
			if app.Cfg != nil && app.Cfg.OverlapAdvisory.Enabled {
				overlapDesc := description
				if overlapDesc == "" {
					overlapDesc = name
				}
				app.CheckOverlap(name, "# "+name+"\n\n"+overlapDesc)
			}

			return nil
		},
	}

	cmd.Flags().String("node", "", "Parent node address (omit for root-level)")
	cmd.Flags().String("type", "leaf", "Node type: leaf or orchestrator")
	cmd.Flags().String("description", "", "Project description (written to the project .md file)")
	cmd.Flags().String("scope", "", "Planning scope (orchestrators only: sets scope and triggers planning)")

	return cmd
}
