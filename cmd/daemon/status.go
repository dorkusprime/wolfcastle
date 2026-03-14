package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/tree"
	"github.com/spf13/cobra"
)

func newStatusCmd(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
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
				if err := app.RequireResolver(); err != nil {
					return err
				}
			}

			if showAll {
				return showAllStatus(app)
			}

			idx, err := app.Resolver.LoadRootIndex()
			if err != nil {
				return err
			}

			return showTreeStatus(app, idx, scopeNode)
		},
	}
}

func showTreeStatus(app *cmdutil.App, idx *state.RootIndex, scope string) error {
	counts := map[state.NodeStatus]int{}
	auditCounts := map[state.AuditStatus]int{}
	openGaps := 0
	openEscalations := 0

	for addr, entry := range idx.Nodes {
		if scope != "" && !isInSubtree(idx, entry.Address, scope) {
			continue
		}
		counts[entry.State]++

		// Load leaf nodes for audit stats
		if entry.Type == state.NodeLeaf {
			a, err := tree.ParseAddress(addr)
			if err != nil {
				continue
			}
			ns, err := state.LoadNodeState(app.Resolver.NodeStatePath(a))
			if err != nil {
				continue
			}
			auditCounts[ns.Audit.Status]++
			for _, g := range ns.Audit.Gaps {
				if g.Status == state.GapOpen {
					openGaps++
				}
			}
			for _, e := range ns.Audit.Escalations {
				if e.Status == state.EscalationOpen {
					openEscalations++
				}
			}
		}
	}

	total := len(idx.Nodes)
	if scope != "" {
		total = counts[state.StatusNotStarted] + counts[state.StatusInProgress] + counts[state.StatusComplete] + counts[state.StatusBlocked]
	}

	daemonStatus := getDaemonStatus(app.WolfcastleDir)

	if app.JSONOutput {
		output.Print(output.Ok("status", map[string]any{
			"total":             total,
			"not_started":       counts[state.StatusNotStarted],
			"in_progress":       counts[state.StatusInProgress],
			"complete":          counts[state.StatusComplete],
			"blocked":           counts[state.StatusBlocked],
			"daemon":            daemonStatus,
			"audit_pending":     auditCounts[state.AuditPending],
			"audit_in_progress": auditCounts[state.AuditInProgress],
			"audit_passed":      auditCounts[state.AuditPassed],
			"audit_failed":      auditCounts[state.AuditFailed],
			"open_gaps":         openGaps,
			"open_escalations":  openEscalations,
		}))
	} else {
		output.PrintHuman("Wolfcastle Status")
		output.PrintHuman("")
		output.PrintHuman("  Nodes")
		output.PrintHuman("    Total:        %d", total)
		output.PrintHuman("    Not started:  %d", counts[state.StatusNotStarted])
		output.PrintHuman("    In progress:  %d", counts[state.StatusInProgress])
		output.PrintHuman("    Complete:     %d", counts[state.StatusComplete])
		output.PrintHuman("    Blocked:      %d", counts[state.StatusBlocked])
		output.PrintHuman("")
		output.PrintHuman("  Audit")
		output.PrintHuman("    Pending:      %d", auditCounts[state.AuditPending])
		output.PrintHuman("    In progress:  %d", auditCounts[state.AuditInProgress])
		output.PrintHuman("    Passed:       %d", auditCounts[state.AuditPassed])
		output.PrintHuman("    Failed:       %d", auditCounts[state.AuditFailed])
		if openGaps > 0 {
			output.PrintHuman("    Open gaps:    %d", openGaps)
		}
		if openEscalations > 0 {
			output.PrintHuman("    Open escalations: %d", openEscalations)
		}
		output.PrintHuman("")
		output.PrintHuman("  Daemon: %s", daemonStatus)
	}
	return nil
}

// getDaemonStatus checks the PID file and reports daemon status.
func getDaemonStatus(wolfcastleDir string) string {
	pidPath := filepath.Join(wolfcastleDir, "wolfcastle.pid")
	data, err := os.ReadFile(pidPath)
	if err != nil {
		return "stopped"
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return "unknown (malformed PID file)"
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Sprintf("stopped (stale PID %d)", pid)
	}
	if err := proc.Signal(syscall.Signal(0)); err != nil {
		return fmt.Sprintf("stopped (stale PID %d)", pid)
	}
	return fmt.Sprintf("running (PID %d)", pid)
}

func showAllStatus(app *cmdutil.App) error {
	projectsDir := filepath.Join(app.WolfcastleDir, "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return fmt.Errorf("reading projects dir: %w — is this a valid Wolfcastle workspace?", err)
	}

	type namespaceSummary struct {
		Namespace  string `json:"namespace"`
		Total      int    `json:"total"`
		Complete   int    `json:"complete"`
		InProgress int    `json:"in_progress"`
		Blocked    int    `json:"blocked"`
		NotStarted int    `json:"not_started"`
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

	if app.JSONOutput {
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
