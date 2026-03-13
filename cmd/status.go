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
	RunE: func(cmd *cobra.Command, args []string) error {
		showAll, _ := cmd.Flags().GetBool("all")
		scopeNode, _ := cmd.Flags().GetString("node")

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
		if scope != "" && entry.Address != scope && entry.Parent != scope {
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
		return fmt.Errorf("reading projects dir: %w", err)
	}

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
		output.PrintHuman("[%s] %d nodes: %d complete, %d in-progress, %d blocked",
			entry.Name(), len(idx.Nodes),
			counts[state.StatusComplete],
			counts[state.StatusInProgress],
			counts[state.StatusBlocked])
	}
	return nil
}

func init() {
	statusCmd.Flags().Bool("all", false, "Show status across all engineers")
	statusCmd.Flags().String("node", "", "Show status for a specific subtree")
	rootCmd.AddCommand(statusCmd)
}
