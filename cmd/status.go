package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current state of the project tree",
	Long: `Displays a summary of node states in the project tree.

Use --node to scope the status to a specific subtree.
Use --all to show status across all engineers' namespaces.

Examples:
  wolfcastle status
  wolfcastle status --node auth-system
  wolfcastle status --all
  wolfcastle status --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		showAll, _ := cmd.Flags().GetBool("all")
		scopeNode, _ := cmd.Flags().GetString("node")

		if !showAll {
			if err := requireResolver(); err != nil {
				return err
			}
		}

		if showAll {
			return showAllStatus()
		}

		idx, err := resolver.LoadRootIndex()
		if err != nil {
			return err
		}

		return showTreeStatus(idx, scopeNode)
	},
}

func showTreeStatus(idx *state.RootIndex, scope string) error {
	counts := map[state.NodeStatus]int{}
	for _, entry := range idx.Nodes {
		if scope != "" && !isInSubtree(idx, entry.Address, scope) {
			continue
		}
		counts[entry.State]++
	}

	total := len(idx.Nodes)
	if scope != "" {
		total = counts[state.StatusNotStarted] + counts[state.StatusInProgress] + counts[state.StatusComplete] + counts[state.StatusBlocked]
	}

	if jsonOutput {
		output.Print(output.Ok("status", map[string]any{
			"total":       total,
			"not_started": counts[state.StatusNotStarted],
			"in_progress": counts[state.StatusInProgress],
			"complete":    counts[state.StatusComplete],
			"blocked":     counts[state.StatusBlocked],
		}))
	} else {
		output.PrintHuman("Wolfcastle Status")
		output.PrintHuman("  Total nodes:  %d", total)
		output.PrintHuman("  Not started:  %d", counts[state.StatusNotStarted])
		output.PrintHuman("  In progress:  %d", counts[state.StatusInProgress])
		output.PrintHuman("  Complete:     %d", counts[state.StatusComplete])
		output.PrintHuman("  Blocked:      %d", counts[state.StatusBlocked])
	}
	return nil
}

func showAllStatus() error {
	projectsDir := filepath.Join(wolfcastleDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return fmt.Errorf("reading projects dir: %w — is this a valid Wolfcastle workspace?", err)
	}

	type namespaceSummary struct {
		Namespace   string `json:"namespace"`
		Total       int    `json:"total"`
		Complete    int    `json:"complete"`
		InProgress  int    `json:"in_progress"`
		Blocked     int    `json:"blocked"`
		NotStarted  int    `json:"not_started"`
	}

	var summaries []namespaceSummary

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		idxPath := filepath.Join(projectsDir, entry.Name(), "state.json")
		idx, err := state.LoadRootIndex(idxPath)
		if err != nil {
			continue
		}
		counts := map[state.NodeStatus]int{}
		for _, e := range idx.Nodes {
			counts[e.State]++
		}
		summaries = append(summaries, namespaceSummary{
			Namespace:  entry.Name(),
			Total:      len(idx.Nodes),
			Complete:   counts[state.StatusComplete],
			InProgress: counts[state.StatusInProgress],
			Blocked:    counts[state.StatusBlocked],
			NotStarted: counts[state.StatusNotStarted],
		})
	}

	if jsonOutput {
		output.Print(output.Ok("status_all", map[string]any{
			"namespaces": summaries,
			"count":      len(summaries),
		}))
	} else {
		if len(summaries) == 0 {
			output.PrintHuman("No engineer namespaces found in projects/")
		} else {
			for _, s := range summaries {
				output.PrintHuman("[%s] %d nodes: %d complete, %d in-progress, %d blocked",
					s.Namespace, s.Total, s.Complete, s.InProgress, s.Blocked)
			}
		}
	}
	return nil
}

// isInSubtree checks whether addr is the scope node or a descendant of it.
func isInSubtree(idx *state.RootIndex, addr string, scope string) bool {
	current := addr
	for current != "" {
		if current == scope {
			return true
		}
		entry, ok := idx.Nodes[current]
		if !ok {
			return false
		}
		current = entry.Parent
	}
	return false
}

func init() {
	statusCmd.Flags().Bool("all", false, "Show status across all engineers")
	statusCmd.Flags().String("node", "", "Show status for a specific subtree")
	rootCmd.AddCommand(statusCmd)
}
