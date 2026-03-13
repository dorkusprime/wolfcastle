package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

type doctorIssue struct {
	Severity    string `json:"severity"`
	Category    string `json:"category"`
	Node        string `json:"node,omitempty"`
	Description string `json:"description"`
	CanAutoFix  bool   `json:"can_auto_fix"`
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Validate structural integrity of the project tree",
	Long:  "Checks for orphaned files, state inconsistencies, missing audit tasks, and other structural issues.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if resolver == nil {
			return fmt.Errorf("identity not configured — run 'wolfcastle init' first")
		}

		var issues []doctorIssue

		// Load root index
		idx, err := resolver.LoadRootIndex()
		if err != nil {
			issues = append(issues, doctorIssue{
				Severity:    "error",
				Category:    "root_index",
				Description: fmt.Sprintf("Cannot load root index: %v", err),
			})
			return reportIssues(issues)
		}

		// Check each node in the index
		for addr, entry := range idx.Nodes {
			a, parseErr := tree.ParseAddress(addr)
			if parseErr != nil {
				issues = append(issues, doctorIssue{
					Severity:    "error",
					Category:    "invalid_address",
					Node:        addr,
					Description: fmt.Sprintf("Invalid address in index: %v", parseErr),
				})
				continue
			}
			statePath := filepath.Join(resolver.ProjectsDir(), filepath.Join(a.Parts...), "state.json")

			// Check state file exists
			ns, err := state.LoadNodeState(statePath)
			if err != nil {
				issues = append(issues, doctorIssue{
					Severity:    "error",
					Category:    "orphan_index",
					Node:        addr,
					Description: fmt.Sprintf("Index references node but state file missing: %s", statePath),
					CanAutoFix:  true,
				})
				continue
			}

			// Check state consistency
			if ns.State != entry.State {
				issues = append(issues, doctorIssue{
					Severity:    "warning",
					Category:    "state_mismatch",
					Node:        addr,
					Description: fmt.Sprintf("Index says %s but node state says %s", entry.State, ns.State),
					CanAutoFix:  true,
				})
			}

			// Check type consistency
			if ns.Type != entry.Type {
				issues = append(issues, doctorIssue{
					Severity:    "error",
					Category:    "type_mismatch",
					Node:        addr,
					Description: fmt.Sprintf("Index says %s but node state says %s", entry.Type, ns.Type),
				})
			}

			// Check leaf has audit task
			if ns.Type == state.NodeLeaf {
				hasAudit := false
				auditLast := false
				for i, t := range ns.Tasks {
					if t.ID == "audit" {
						hasAudit = true
						auditLast = i == len(ns.Tasks)-1
					}
				}
				if !hasAudit {
					issues = append(issues, doctorIssue{
						Severity:    "error",
						Category:    "missing_audit",
						Node:        addr,
						Description: "Leaf node has no audit task",
						CanAutoFix:  true,
					})
				} else if !auditLast {
					issues = append(issues, doctorIssue{
						Severity:    "error",
						Category:    "audit_position",
						Node:        addr,
						Description: "Audit task is not the last task",
						CanAutoFix:  true,
					})
				}
			}

			// Check for invalid state values
			validStates := map[state.NodeStatus]bool{
				state.StatusNotStarted: true,
				state.StatusInProgress: true,
				state.StatusComplete:   true,
				state.StatusBlocked:    true,
			}
			if !validStates[ns.State] {
				issues = append(issues, doctorIssue{
					Severity:    "error",
					Category:    "invalid_state",
					Node:        addr,
					Description: fmt.Sprintf("Invalid state value: %q", ns.State),
					CanAutoFix:  false,
				})
			}

			// Check parent reference
			if entry.Parent != "" {
				if _, ok := idx.Nodes[entry.Parent]; !ok {
					issues = append(issues, doctorIssue{
						Severity:    "error",
						Category:    "orphan_parent",
						Node:        addr,
						Description: fmt.Sprintf("Parent %q not found in index", entry.Parent),
					})
				}
			}

			// Check children references
			for _, childAddr := range entry.Children {
				if _, ok := idx.Nodes[childAddr]; !ok {
					issues = append(issues, doctorIssue{
						Severity:    "error",
						Category:    "orphan_child",
						Node:        addr,
						Description: fmt.Sprintf("Child %q not found in index", childAddr),
					})
				}
			}
		}

		// Check for orphaned state files (on disk but not in index)
		err = checkOrphanedFiles(resolver.ProjectsDir(), idx, &issues)
		if err != nil {
			issues = append(issues, doctorIssue{
				Severity:    "warning",
				Category:    "filesystem",
				Description: fmt.Sprintf("Error scanning filesystem: %v", err),
			})
		}

		fix, _ := cmd.Flags().GetBool("fix")
		if !fix {
			return reportIssues(issues)
		}

		// Report issues first
		if err := reportIssues(issues); err != nil {
			return err
		}

		// Fix auto-fixable issues
		var fixed []string
		modifiedStates := map[string]*state.NodeState{}
		indexModified := false

		for _, issue := range issues {
			if !issue.CanAutoFix {
				continue
			}

			switch issue.Category {
			case "orphan_index":
				// Remove dangling entry from root index
				delete(idx.Nodes, issue.Node)
				// Also remove from parent's children list
				for addr, entry := range idx.Nodes {
					for i, child := range entry.Children {
						if child == issue.Node {
							entry.Children = append(entry.Children[:i], entry.Children[i+1:]...)
							idx.Nodes[addr] = entry
							break
						}
					}
				}
				indexModified = true
				fixed = append(fixed, fmt.Sprintf("orphan_index: removed %s from index", issue.Node))

			case "state_mismatch":
				// Node state is authoritative — update index to match
				a, aErr := tree.ParseAddress(issue.Node)
				if aErr != nil {
					continue
				}
				statePath := filepath.Join(resolver.ProjectsDir(), filepath.Join(a.Parts...), "state.json")
				ns, loadErr := state.LoadNodeState(statePath)
				if loadErr != nil {
					continue
				}
				if entry, ok := idx.Nodes[issue.Node]; ok {
					entry.State = ns.State
					idx.Nodes[issue.Node] = entry
					indexModified = true
				}
				fixed = append(fixed, fmt.Sprintf("state_mismatch: updated index for %s to %s", issue.Node, ns.State))

			case "missing_audit":
				// Append audit task to leaf's task list
				a, aErr := tree.ParseAddress(issue.Node)
				if aErr != nil {
					continue
				}
				statePath := filepath.Join(resolver.ProjectsDir(), filepath.Join(a.Parts...), "state.json")
				ns, ok := modifiedStates[statePath]
				if !ok {
					var loadErr error
					ns, loadErr = state.LoadNodeState(statePath)
					if loadErr != nil {
						continue
					}
				}
				ns.Tasks = append(ns.Tasks, state.Task{
					ID:          "audit",
					Description: "Audit task completion and verify acceptance criteria",
					State:       state.StatusNotStarted,
				})
				modifiedStates[statePath] = ns
				fixed = append(fixed, fmt.Sprintf("missing_audit: added audit task to %s", issue.Node))

			case "audit_position":
				// Move audit task to end of task list
				a, aErr := tree.ParseAddress(issue.Node)
				if aErr != nil {
					continue
				}
				statePath := filepath.Join(resolver.ProjectsDir(), filepath.Join(a.Parts...), "state.json")
				ns, ok := modifiedStates[statePath]
				if !ok {
					var loadErr error
					ns, loadErr = state.LoadNodeState(statePath)
					if loadErr != nil {
						continue
					}
				}
				var auditTask *state.Task
				var others []state.Task
				for i := range ns.Tasks {
					if ns.Tasks[i].ID == "audit" {
						t := ns.Tasks[i]
						auditTask = &t
					} else {
						others = append(others, ns.Tasks[i])
					}
				}
				if auditTask != nil {
					ns.Tasks = append(others, *auditTask)
					modifiedStates[statePath] = ns
				}
				fixed = append(fixed, fmt.Sprintf("audit_position: moved audit to end in %s", issue.Node))

			case "orphan_state":
				// Add orphaned node to root index, infer parent from path
				a, aErr := tree.ParseAddress(issue.Node)
				if aErr != nil {
					continue
				}
				statePath := filepath.Join(resolver.ProjectsDir(), filepath.Join(a.Parts...), "state.json")
				ns, loadErr := state.LoadNodeState(statePath)
				if loadErr != nil {
					continue
				}
				parentAddr := ""
				if len(a.Parts) > 1 {
					parentAddr = strings.Join(a.Parts[:len(a.Parts)-1], "/")
				}
				idx.Nodes[issue.Node] = state.IndexEntry{
					Name:               ns.Name,
					Type:               ns.Type,
					State:              ns.State,
					Address:            issue.Node,
					DecompositionDepth: ns.DecompositionDepth,
					Parent:             parentAddr,
				}
				// Add as child to parent if parent exists in index
				if parentAddr != "" {
					if parentEntry, ok := idx.Nodes[parentAddr]; ok {
						parentEntry.Children = append(parentEntry.Children, issue.Node)
						idx.Nodes[parentAddr] = parentEntry
					}
				}
				indexModified = true
				fixed = append(fixed, fmt.Sprintf("orphan_state: added %s to index", issue.Node))
			}
		}

		// Save modified state files
		for statePath, ns := range modifiedStates {
			if saveErr := state.SaveNodeState(statePath, ns); saveErr != nil {
				output.PrintHuman("Error saving %s: %v", statePath, saveErr)
			}
		}

		// Save root index if modified
		if indexModified {
			if saveErr := state.SaveRootIndex(resolver.RootIndexPath(), idx); saveErr != nil {
				output.PrintHuman("Error saving root index: %v", saveErr)
			}
		}

		// Report fixes
		if len(fixed) == 0 {
			output.PrintHuman("\nNo auto-fixable issues found")
		} else {
			output.PrintHuman("\nFixed %d issues:", len(fixed))
			for _, f := range fixed {
				output.PrintHuman("  FIXED %s", f)
			}
		}

		return nil
	},
}

func checkOrphanedFiles(projectsDir string, idx *state.RootIndex, issues *[]doctorIssue) error {
	return filepath.Walk(projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.Name() != "state.json" || path == filepath.Join(projectsDir, "state.json") {
			return nil
		}
		// Derive address from path
		rel, err := filepath.Rel(projectsDir, filepath.Dir(path))
		if err != nil {
			return nil
		}
		addr := filepath.ToSlash(rel)
		if _, ok := idx.Nodes[addr]; !ok {
			*issues = append(*issues, doctorIssue{
				Severity:    "warning",
				Category:    "orphan_state",
				Node:        addr,
				Description: "State file exists on disk but node not in index",
				CanAutoFix:  true,
			})
		}
		return nil
	})
}

func reportIssues(issues []doctorIssue) error {
	if jsonOutput {
		output.Print(output.Ok("doctor", map[string]any{
			"issues": issues,
			"count":  len(issues),
		}))
		return nil
	}

	if len(issues) == 0 {
		output.PrintHuman("No issues found — project tree is healthy")
		return nil
	}

	errors := 0
	warnings := 0
	for _, issue := range issues {
		prefix := "  "
		switch issue.Severity {
		case "error":
			prefix = "  ERROR"
			errors++
		case "warning":
			prefix = "  WARN "
			warnings++
		case "info":
			prefix = "  INFO "
		}
		if issue.Node != "" {
			output.PrintHuman("%s [%s] %s: %s", prefix, issue.Category, issue.Node, issue.Description)
		} else {
			output.PrintHuman("%s [%s] %s", prefix, issue.Category, issue.Description)
		}
	}
	output.PrintHuman("")
	output.PrintHuman("Found %d issues (%d errors, %d warnings)", len(issues), errors, warnings)
	return nil
}

func init() {
	doctorCmd.Flags().Bool("fix", false, "Attempt to fix deterministic issues")
	rootCmd.AddCommand(doctorCmd)
}
