package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/cmd/cmdutil"
	dmn "github.com/dorkusprime/wolfcastle/internal/daemon"
	"github.com/dorkusprime/wolfcastle/internal/output"
	"github.com/dorkusprime/wolfcastle/internal/signals"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/spf13/cobra"
)

func newStatusCmd(app *cmdutil.App) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Survey the battlefield",
		Long: `Shows node states across the project tree. How many targets remain.
How many have fallen. Use --node to scope to a subtree, --all to
see every engineer's namespace. Use --watch to refresh on an interval.
Use --detail to see task bodies, failure reasons, deliverables, and
breadcrumbs for in-progress work.

Examples:
  wolfcastle status
  wolfcastle status --node auth-system
  wolfcastle status --watch
  wolfcastle status -w --interval 2
  wolfcastle status --all
  wolfcastle status --detail
  wolfcastle status --expand --detail
  wolfcastle status --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			showAll, _ := cmd.Flags().GetBool("all")
			scopeNode, _ := cmd.Flags().GetString("node")
			watch, _ := cmd.Flags().GetBool("watch")
			interval, _ := cmd.Flags().GetFloat64("interval")
			width, _ := cmd.Flags().GetInt("width")

			expand, _ := cmd.Flags().GetBool("expand")
			detail, _ := cmd.Flags().GetBool("detail")
			archived, _ := cmd.Flags().GetBool("archived")

			opts := treeOpts{
				Expand:   expand,
				Detail:   detail,
				Archived: archived,
				Width:    width,
			}

			if !showAll {
				if err := app.RequireIdentity(); err != nil {
					return err
				}
			}

			if watch {
				parent := cmd.Context()
				if parent == nil {
					parent = context.Background()
				}
				ctx, stop := signal.NotifyContext(parent, signals.Shutdown...)
				defer stop()
				return watchStatus(ctx, app, scopeNode, showAll, interval, opts)
			}

			if showAll {
				return showAllStatus(app)
			}

			idx, err := app.State.ReadIndex()
			if err != nil {
				return err
			}

			return showTreeStatus(app, idx, scopeNode, opts)
		},
	}
}

// treeOpts controls how the status tree is rendered.
type treeOpts struct {
	Expand   bool
	Detail   bool
	Archived bool
	Width    int // truncation width for text fields
}

// nodeDetail holds the index entry and optionally the full node state
// for rendering the tree view.
type nodeDetail struct {
	entry state.IndexEntry
	ns    *state.NodeState // nil for orchestrators or load failures
}

func showTreeStatus(app *cmdutil.App, idx *state.RootIndex, scope string, opts treeOpts) error {
	showArchived := opts.Archived

	// Load scope locks and build a task -> locked paths index.
	taskScopeLocks := map[string][]string{}
	if table, err := app.State.ReadScopeLocks(); err == nil {
		for path, lock := range table.Locks {
			taskScopeLocks[lock.Task] = append(taskScopeLocks[lock.Task], path)
		}
		for task := range taskScopeLocks {
			sort.Strings(taskScopeLocks[task])
		}
	}

	counts := map[state.NodeStatus]int{}
	auditCounts := map[state.AuditStatus]int{}
	openGaps := 0
	openEscalations := 0

	details := map[string]*nodeDetail{}
	archivedProjects := 0

	for addr, entry := range idx.Nodes {
		if scope != "" && !isInSubtree(idx, entry.Address, scope) {
			continue
		}
		if entry.Archived {
			if entry.Parent == "" {
				archivedProjects++
			}
			if !showArchived {
				continue
			}
		} else if showArchived {
			// --archived: only show archived nodes
			continue
		}
		counts[entry.State]++
		nd := &nodeDetail{entry: entry}
		details[addr] = nd

		ns, err := app.State.ReadNode(addr)
		if err == nil {
			nd.ns = ns
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

	total := len(details)

	daemonStatus := getDaemonStatus(app.Daemon)

	if app.JSON {
		// Build per-node detail for JSON consumers.
		nodeDetails := make(map[string]any, len(details))
		for addr, nd := range details {
			info := map[string]any{
				"name":  nd.entry.Name,
				"type":  nd.entry.Type,
				"state": nd.entry.State,
			}
			if nd.ns != nil {
				if len(nd.ns.Tasks) > 0 {
					taskList := make([]map[string]any, 0, len(nd.ns.Tasks))
					for _, t := range nd.ns.Tasks {
						td := map[string]any{
							"id":            t.ID,
							"state":         t.State,
							"description":   t.Description,
							"failure_count": t.FailureCount,
						}
						if t.Title != "" {
							td["title"] = t.Title
						}
						if t.Body != "" {
							td["body"] = t.Body
						}
						if t.LastFailureType != "" {
							td["last_failure_type"] = t.LastFailureType
						}
						if len(t.Deliverables) > 0 {
							td["deliverables"] = t.Deliverables
						}
						if t.BlockedReason != "" {
							td["block_reason"] = t.BlockedReason
						}
						taskAddr := addr + "/" + t.ID
						if locks, ok := taskScopeLocks[taskAddr]; ok && len(locks) > 0 {
							td["scope_locks"] = locks
						}
						taskList = append(taskList, td)
					}
					info["tasks"] = taskList
				}
				if len(nd.ns.Audit.Breadcrumbs) > 0 {
					info["breadcrumbs"] = nd.ns.Audit.Breadcrumbs
				}
			}
			nodeDetails[addr] = info
		}

		statusData := map[string]any{
			"total":             total,
			"not_started":       counts[state.StatusNotStarted],
			"in_progress":       counts[state.StatusInProgress],
			"complete":          counts[state.StatusComplete],
			"blocked":           counts[state.StatusBlocked],
			"archived":          archivedProjects,
			"daemon":            daemonStatus,
			"audit_pending":     auditCounts[state.AuditPending],
			"audit_in_progress": auditCounts[state.AuditInProgress],
			"audit_passed":      auditCounts[state.AuditPassed],
			"audit_failed":      auditCounts[state.AuditFailed],
			"open_gaps":         openGaps,
			"open_escalations":  openEscalations,
			"nodes":             nodeDetails,
		}
		if ps := dmn.LoadParallelStatus(app.Config.Root()); ps != nil {
			statusData["parallel"] = ps
		}
		output.Print(output.Ok("status", statusData))
		return nil
	}

	// Human output: header summary + tree view
	if showArchived {
		output.PrintHuman("Wolfcastle Archived")
	} else {
		output.PrintHuman("Wolfcastle Status")
	}
	output.PrintHuman("")

	// Summary line
	var parts []string
	if c := counts[state.StatusComplete]; c > 0 {
		parts = append(parts, fmt.Sprintf("%d complete", c))
	}
	if c := counts[state.StatusInProgress]; c > 0 {
		parts = append(parts, fmt.Sprintf("%d in progress", c))
	}
	if c := counts[state.StatusBlocked]; c > 0 {
		parts = append(parts, fmt.Sprintf("%d blocked", c))
	}
	if c := counts[state.StatusNotStarted]; c > 0 {
		parts = append(parts, fmt.Sprintf("%d not started", c))
	}
	if archivedProjects > 0 {
		parts = append(parts, fmt.Sprintf("%d archived", archivedProjects))
	}
	if total == 0 && archivedProjects == 0 {
		output.PrintHuman("  No targets. Feed the inbox.")
	} else {
		output.PrintHuman("  %d nodes (%s)", total+archivedProjects, strings.Join(parts, ", "))
	}
	output.PrintHuman("")

	// Tree view: walk root nodes in order. When showing archived,
	// walk ArchivedRoot instead of Root.
	roots := idx.Root
	if showArchived {
		roots = idx.ArchivedRoot
	}
	for _, rootAddr := range roots {
		if scope != "" && !isInSubtree(idx, rootAddr, scope) {
			continue
		}
		if _, ok := idx.Nodes[rootAddr]; !ok {
			continue
		}
		printNodeTree(app, idx, details, rootAddr, "  ", opts, taskScopeLocks)
	}

	if !showArchived && archivedProjects > 0 {
		output.PrintHuman("")
		output.PrintHuman("  %s archived (use --archived to view)", output.Plural(archivedProjects, "project", "projects"))
	}

	// Inbox count
	if inboxData, err := app.State.ReadInbox(); err == nil {
		newCount, filedCount := 0, 0
		for _, item := range inboxData.Items {
			switch item.Status {
			case state.InboxNew:
				newCount++
			case state.InboxFiled:
				filedCount++
			}
		}
		if newCount > 0 || filedCount > 0 {
			output.PrintHuman("")
			output.PrintHuman("  Inbox: %d new, %d filed", newCount, filedCount)
		}
	}

	// Planning queue: orchestrators that still need planning (no children,
	// not complete). These get planned when the daemon has no tasks to execute.
	var planQueue []string
	for addr, entry := range idx.Nodes {
		if entry.Type != state.NodeOrchestrator {
			continue
		}
		if len(entry.Children) == 0 && entry.State != state.StatusComplete {
			planQueue = append(planQueue, addr)
		}
	}
	if len(planQueue) > 0 {
		sort.Strings(planQueue)
		output.PrintHuman("  Planning queue: %s", strings.Join(planQueue, ", "))
	}

	output.PrintHuman("  Daemon: %s", daemonStatus)

	// Parallel worker pool status (only shown when a snapshot file exists).
	if ps := dmn.LoadParallelStatus(app.Config.Root()); ps != nil {
		printParallelStatus(ps)
	}

	return nil
}

// printNodeTree recursively prints a node and its children/tasks.
// The optional detailFlag parameter controls whether extra detail
// (task body, failure type, deliverables, breadcrumbs) is shown.
func printNodeTree(app *cmdutil.App, idx *state.RootIndex, details map[string]*nodeDetail, addr string, indent string, opts treeOpts, taskScopeLocks map[string][]string) {
	expand := opts.Expand
	detail := opts.Detail

	nd, ok := details[addr]
	if !ok {
		return
	}

	tp := nodeTypePrefix(nd.entry)

	// Collapse completed and not_started nodes unless --expand is set.
	if (nd.entry.State == state.StatusComplete || nd.entry.State == state.StatusNotStarted) && !expand {
		glyph := nodeGlyph(nd.entry.State)
		childCount := countDescendants(idx, addr)
		if childCount > 0 {
			output.PrintHuman("%s%s %s: %s  (%s, %d nodes)", indent, glyph, tp, nd.entry.Name, addr, childCount+1)
			return
		}
		if nd.ns != nil && len(nd.ns.Tasks) > 0 {
			output.PrintHuman("%s%s %s: %s  (%s, %d tasks)", indent, glyph, tp, nd.entry.Name, addr, len(nd.ns.Tasks))
			return
		}
	}

	glyph := nodeGlyph(nd.entry.State)
	output.PrintHuman("%s%s %s: %s  (%s)", indent, glyph, tp, nd.entry.Name, addr)

	// Show most recent breadcrumb for in_progress nodes when --detail is set.
	if detail && nd.ns != nil && nd.entry.State == state.StatusInProgress && len(nd.ns.Audit.Breadcrumbs) > 0 {
		bc := nd.ns.Audit.Breadcrumbs[len(nd.ns.Audit.Breadcrumbs)-1]
		text := truncate(bc.Text, opts.Width)
		output.PrintHuman("%s  breadcrumb: %s", indent, text)
	}

	// For orchestrators, print children as a timeline: completed work
	// at top (past), active work in middle (present), pending at bottom
	// (future). Within each group, creation order is preserved.
	if nd.entry.Type == state.NodeOrchestrator {
		for _, childAddr := range sortChildrenTimeline(nd.entry.Children, idx) {
			printNodeTree(app, idx, details, childAddr, indent+"  ", opts, taskScopeLocks)
		}
		if nd.ns != nil {
			for _, t := range nd.ns.Tasks {
				if t.IsAudit && (t.State == state.StatusInProgress || t.State == state.StatusBlocked) {
					tGlyph := taskGlyph(t.State)
					output.PrintHuman("%s  %s %s  %s", indent, tGlyph, t.ID, t.Description)
				}
			}
		}
		return
	}

	// For leaves, print tasks
	if nd.ns == nil {
		return
	}
	// Build a set of task IDs that should be skipped because their
	// parent task is collapsed (completed with all children complete).
	skipChildren := map[string]bool{}
	if !expand {
		for _, t := range nd.ns.Tasks {
			if t.State != state.StatusComplete {
				continue
			}
			prefix := t.ID + "."
			childCount := 0
			allChildrenDone := true
			for _, c := range nd.ns.Tasks {
				if !strings.HasPrefix(c.ID, prefix) {
					continue
				}
				// Only immediate children
				rest := c.ID[len(prefix):]
				if strings.Contains(rest, ".") {
					continue
				}
				childCount++
				if c.State != state.StatusComplete {
					allChildrenDone = false
				}
			}
			if childCount > 0 && allChildrenDone {
				// Mark all descendants for skipping
				for _, c := range nd.ns.Tasks {
					if strings.HasPrefix(c.ID, prefix) {
						skipChildren[c.ID] = true
					}
				}
			}
		}
	}

	for _, t := range nd.ns.Tasks {
		if skipChildren[t.ID] {
			continue
		}

		tGlyph := taskGlyph(t.State)
		label := t.Title
		if label == "" {
			label = t.Description
		}

		// Indent subtasks by depth. task-0001.0002 gets one extra
		// level, task-0001.0002.0003 gets two, etc.
		taskIndent := indent + "  "
		depth := strings.Count(t.ID, ".")
		for i := 0; i < depth; i++ {
			taskIndent += "  "
		}

		// Collapsed parent task: show child count instead of listing them
		if !expand && t.State == state.StatusComplete {
			prefix := t.ID + "."
			childCount := 0
			for _, c := range nd.ns.Tasks {
				if strings.HasPrefix(c.ID, prefix) {
					childCount++
				}
			}
			if childCount > 0 {
				output.PrintHuman("%s%s %s  %s  (%d subtasks)", taskIndent, tGlyph, t.ID, label, childCount)
				continue
			}
		}

		extra := ""
		if t.State == state.StatusBlocked && t.BlockedReason != "" {
			extra = "\n" + taskIndent + "       " + t.BlockedReason
		}
		if t.FailureCount > 0 && t.State != state.StatusComplete {
			if t.LastFailureType != "" && detail {
				extra += fmt.Sprintf("  (%d failures, last: %s)", t.FailureCount, t.LastFailureType)
			} else {
				extra += fmt.Sprintf("  (%d failures)", t.FailureCount)
			}
		}
		// Show description detail for completed tasks when a title is
		// the primary label and the description adds information.
		if t.State == state.StatusComplete && t.Title != "" && t.Description != "" && t.Description != t.Title {
			extra += "\n" + taskIndent + "       " + t.Description
		}

		classTag := ""
		if t.Class != "" {
			classTag = " [" + t.Class + "]"
		}
		output.PrintHuman("%s%s %s  %s%s%s", taskIndent, tGlyph, t.ID, label, classTag, extra)

		// Detail-only lines: task body
		if detail {
			if t.Body != "" {
				output.PrintHuman("%s       %s", taskIndent, truncate(t.Body, opts.Width))
			}
		}

		// Scope lock display for in_progress tasks
		if t.State == state.StatusInProgress {
			taskAddr := addr + "/" + t.ID
			if locks, ok := taskScopeLocks[taskAddr]; ok && len(locks) > 0 {
				output.PrintHuman("%s    scope: %s", taskIndent, strings.Join(locks, ", "))
			}
		}
	}

	// Gaps
	for _, g := range nd.ns.Audit.Gaps {
		if g.Status == state.GapOpen {
			if output.IsTerminal() {
				output.PrintHuman("%s    %s⚠ %s: %s%s", indent, colorYellow, g.ID, g.Description, colorReset)
			} else {
				output.PrintHuman("%s    ⚠ %s: %s", indent, g.ID, g.Description)
			}
		}
	}

	// Audit report path (shown in expanded view)
	if expand {
		if reportPath := state.LatestAuditReport(app.State.Dir(), addr); reportPath != "" {
			output.PrintHuman("%s    report: %s", indent, reportPath)
		}
	}
}

func truncate(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// ANSI color codes matching the TUI spec (section 2.9).
const (
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorDim    = "\033[2m"
	colorReset  = "\033[0m"
)

// nodeTypePrefix returns "Proj" for root orchestrators, "Orch" for child
// orchestrators, and "Leaf" for leaf nodes.
func nodeTypePrefix(entry state.IndexEntry) string {
	if entry.Type == state.NodeOrchestrator {
		if entry.Parent == "" {
			return "Proj"
		}
		return "Orch"
	}
	return "Leaf"
}

// nodeGlyph returns the TUI-consistent colored status glyph for a node.
func nodeGlyph(s state.NodeStatus) string {
	if !output.IsTerminal() {
		switch s {
		case state.StatusComplete:
			return "●"
		case state.StatusInProgress:
			return "◐"
		case state.StatusBlocked:
			return "☢"
		default:
			return "◯"
		}
	}
	switch s {
	case state.StatusComplete:
		return colorGreen + "●" + colorReset
	case state.StatusInProgress:
		return colorYellow + "◐" + colorReset
	case state.StatusBlocked:
		return colorRed + "☢" + colorReset
	default:
		return colorDim + "◯" + colorReset
	}
}

// taskGlyph returns the colored status glyph for a task.
func taskGlyph(s state.NodeStatus) string {
	if !output.IsTerminal() {
		switch s {
		case state.StatusComplete:
			return "✓"
		case state.StatusInProgress:
			return "→"
		case state.StatusBlocked:
			return "✖"
		default:
			return "○"
		}
	}
	switch s {
	case state.StatusComplete:
		return colorGreen + "✓" + colorReset
	case state.StatusInProgress:
		return colorYellow + "→" + colorReset
	case state.StatusBlocked:
		return colorRed + "✖" + colorReset
	default:
		return colorDim + "○" + colorReset
	}
}

// getDaemonStatus checks the PID file and reports daemon status.
func getDaemonStatus(repo *dmn.DaemonRepository) string {
	pid, err := repo.ReadPID()
	if err != nil {
		return "stopped"
	}
	if !dmn.IsProcessRunning(pid) {
		return fmt.Sprintf("stopped (stale PID %d)", pid)
	}
	return fmt.Sprintf("running (PID %d)", pid)
}

// printParallelStatus renders the worker pool section of the status display.
func printParallelStatus(ps *dmn.ParallelStatus) {
	activeCount := len(ps.Active)
	output.PrintHuman("")
	output.PrintHuman("  Workers: %d/%d active", activeCount, ps.MaxWorkers)

	for _, w := range ps.Active {
		output.PrintHuman("")
		output.PrintHuman("    %s [%s]", w.Task, state.StatusInProgress)
		if len(w.Scope) > 0 {
			output.PrintHuman("      scope: %s", strings.Join(w.Scope, ", "))
		}
	}

	if len(ps.Yielded) > 0 {
		output.PrintHuman("")
		output.PrintHuman("  Yielded (waiting on scope):")
		for _, y := range ps.Yielded {
			suffix := ""
			if y.YieldCount > 1 {
				suffix = fmt.Sprintf(" (%d yields, %ds)", y.YieldCount, y.BlockedForSecs)
			}
			output.PrintHuman("    %s -> blocked by %s%s", y.Task, y.Blocker, suffix)
		}
	}
}

func showAllStatus(app *cmdutil.App) error {
	projectsDir := filepath.Join(app.Config.Root(), "system", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return fmt.Errorf("reading projects dir: %w", err)
	}

	type namespaceSummary struct {
		Namespace  string `json:"namespace"`
		Total      int    `json:"total"`
		Complete   int    `json:"complete"`
		InProgress int    `json:"in_progress"`
		Blocked    int    `json:"blocked"`
		NotStarted int    `json:"not_started"`
		Archived   int    `json:"archived"`
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
		archivedN := 0
		for _, e := range idx.Nodes {
			if e.Archived {
				archivedN++
				continue
			}
			counts[e.State]++
		}
		summaries = append(summaries, namespaceSummary{
			Namespace:  entry.Name(),
			Total:      len(idx.Nodes) - archivedN,
			Complete:   counts[state.StatusComplete],
			InProgress: counts[state.StatusInProgress],
			Blocked:    counts[state.StatusBlocked],
			NotStarted: counts[state.StatusNotStarted],
			Archived:   archivedN,
		})
	}

	if app.JSON {
		output.Print(output.Ok("status_all", map[string]any{
			"namespaces": summaries,
			"count":      len(summaries),
		}))
	} else {
		if len(summaries) == 0 {
			output.PrintHuman("No namespaces found. The battlefield is empty.")
		} else {
			for _, s := range summaries {
				line := fmt.Sprintf("[%s] %d nodes: %d complete, %d in-progress, %d blocked",
					s.Namespace, s.Total, s.Complete, s.InProgress, s.Blocked)
				if s.Archived > 0 {
					line += fmt.Sprintf(", %d archived", s.Archived)
				}
				output.PrintHuman("%s", line)
			}
		}
	}
	return nil
}

// watchStatus refreshes the status display on an interval. Uses the
// alternate screen buffer and cursor repositioning for flicker-free
// updates (no clear-then-redraw flash).
func watchStatus(ctx context.Context, app *cmdutil.App, scope string, showAll bool, intervalSec float64, opts treeOpts) error {
	if intervalSec < 0.1 {
		intervalSec = 0.1
	}
	d := time.Duration(intervalSec * float64(time.Second))

	// Enter alternate screen buffer
	if output.IsTerminal() {
		_, _ = fmt.Fprint(os.Stdout, "\033[?1049h")
		defer func() { _, _ = fmt.Fprint(os.Stdout, "\033[?1049l") }()
	}

	for {
		// Home + clear. Inside the alternate screen buffer this is
		// effectively instantaneous, no visible flash. Cursor-home
		// alone left stale text when lines shrank between frames.
		_, _ = fmt.Fprint(os.Stdout, "\033[H\033[2J")

		// Show interval header
		if output.IsTerminal() {
			output.PrintHuman("%sEvery %.1fs: wolfcastle status%s", colorDim, intervalSec, colorReset)
			output.PrintHuman("")
		}

		if showAll {
			if err := showAllStatus(app); err != nil {
				output.PrintError("%v", err)
			}
		} else {
			idx, err := app.State.ReadIndex()
			if err != nil {
				output.PrintError("%v", err)
			} else {
				if err := showTreeStatus(app, idx, scope, opts); err != nil {
					output.PrintError("%v", err)
				}
			}
		}

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(d):
		}
	}
}

// countDescendants returns the total number of descendant nodes under addr.
func countDescendants(idx *state.RootIndex, addr string) int {
	entry, ok := idx.Nodes[addr]
	if !ok {
		return 0
	}
	count := 0
	for _, child := range entry.Children {
		count++ // the child itself
		count += countDescendants(idx, child)
	}
	return count
}

// sortChildrenTimeline reorders children so completed nodes appear first
// (they finished in the past), then in-progress/blocked (happening now),
// then not-started (future). Creation order preserved within each group.
func sortChildrenTimeline(children []string, idx *state.RootIndex) []string {
	sorted := make([]string, len(children))
	copy(sorted, children)
	sort.SliceStable(sorted, func(i, j int) bool {
		return timelinePriority(idx.Nodes[sorted[i]].State) < timelinePriority(idx.Nodes[sorted[j]].State)
	})
	return sorted
}

func timelinePriority(s state.NodeStatus) int {
	switch s {
	case state.StatusComplete:
		return 0
	case state.StatusInProgress:
		return 1
	case state.StatusBlocked:
		return 2
	case state.StatusNotStarted:
		return 3
	default:
		return 4
	}
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
