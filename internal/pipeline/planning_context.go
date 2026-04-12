package pipeline

import (
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// BuildPlanningContext assembles the iteration context for an orchestrator
// planning pass. It includes the orchestrator's scope, pending scope items,
// children's state, success criteria, and planning history.
//
// Truncation priority when context exceeds budget:
// 1. Planning history beyond the last 3 passes
// 2. Children state detail (reduce to ID + state only)
// 3. Pending scope items beyond the first 5
// 4. Scope description is never truncated
func BuildPlanningContext(nodeAddr string, ns *state.NodeState, trigger string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "**Orchestrator:** %s\n", nodeAddr)
	fmt.Fprintf(&b, "**Planning Trigger:** %s\n", trigger)

	maxReplans := ns.MaxReplans
	if maxReplans == 0 {
		maxReplans = 3
	}
	if ns.TotalReplans > 0 {
		fmt.Fprintf(&b, "**Remediation Attempt:** %d of %d\n", ns.TotalReplans, maxReplans)
	}
	if ns.ReviewPass > 0 || trigger == "completion_review" {
		maxReview := ns.MaxReviewPasses
		if maxReview == 0 {
			maxReview = 3
		}
		fmt.Fprintf(&b, "**Review Pass:** %d of %d\n", ns.ReviewPass, maxReview)
	}
	b.WriteString("\n")

	// Scope (never truncated)
	if ns.Scope != "" {
		b.WriteString("## Scope\n\n")
		b.WriteString(ns.Scope + "\n\n")
	}

	// Pending scope items
	if len(ns.PendingScope) > 0 {
		b.WriteString("## Pending Scope\n\n")
		b.WriteString("New work items to integrate into the plan:\n\n")
		limit := len(ns.PendingScope)
		if limit > 5 {
			limit = 5
		}
		for i := 0; i < limit; i++ {
			fmt.Fprintf(&b, "- %s\n", ns.PendingScope[i])
		}
		if len(ns.PendingScope) > 5 {
			fmt.Fprintf(&b, "\n(%d additional items truncated)\n", len(ns.PendingScope)-5)
		}
		b.WriteString("\n")
	}

	// Success criteria
	if len(ns.SuccessCriteria) > 0 {
		b.WriteString("## Success Criteria\n\n")
		for _, c := range ns.SuccessCriteria {
			fmt.Fprintf(&b, "- %s\n", c)
		}
		b.WriteString("\n")
	}

	// Children state
	if len(ns.Children) > 0 {
		b.WriteString("## Children\n\n")
		for _, child := range ns.Children {
			fmt.Fprintf(&b, "- **%s** (%s): %s\n", child.ID, child.Address, child.State)
		}
		b.WriteString("\n")
	}

	// Task state (for leaves with tasks)
	if len(ns.Tasks) > 0 {
		b.WriteString("## Tasks\n\n")
		for _, t := range ns.Tasks {
			marker := ""
			switch t.State {
			case state.StatusComplete:
				marker = "✓"
			case state.StatusInProgress:
				marker = "→"
			case state.StatusBlocked:
				marker = "✖"
			default:
				marker = "○"
			}
			fmt.Fprintf(&b, "%s %s: %s", marker, t.ID, t.Description)
			if t.State == state.StatusBlocked && t.BlockedReason != "" {
				fmt.Fprintf(&b, " (blocked: %s)", t.BlockedReason)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Planning history (last 3)
	if len(ns.PlanningHistory) > 0 {
		b.WriteString("## Planning History\n\n")
		start := 0
		if len(ns.PlanningHistory) > 3 {
			start = len(ns.PlanningHistory) - 3
		}
		for _, pass := range ns.PlanningHistory[start:] {
			fmt.Fprintf(&b, "- [%s] %s: %s\n",
				pass.Timestamp.Format("2006-01-02T15:04Z"),
				pass.Trigger,
				pass.Summary)
		}
		b.WriteString("\n")
	}

	// Audit state (gaps, breadcrumbs)
	if len(ns.Audit.Gaps) > 0 {
		openGaps := 0
		for _, g := range ns.Audit.Gaps {
			if g.Status == state.GapOpen {
				openGaps++
			}
		}
		if openGaps > 0 {
			b.WriteString("## Open Audit Gaps\n\n")
			for _, g := range ns.Audit.Gaps {
				if g.Status == state.GapOpen {
					fmt.Fprintf(&b, "- %s: %s\n", g.ID, g.Description)
				}
			}
			b.WriteString("\n")
		}
	}

	// Linked specs
	if len(ns.Specs) > 0 {
		b.WriteString("## Linked Specs\n\n")
		for _, s := range ns.Specs {
			fmt.Fprintf(&b, "- %s\n", s)
		}
		b.WriteString("\n")
	}

	return b.String()
}
