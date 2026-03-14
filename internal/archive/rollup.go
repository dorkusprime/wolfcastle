// Package archive generates timestamped Markdown archive entries from
// completed node state. Archive entries include breadcrumbs, audit results,
// and metadata, providing a permanent record of completed work (ADR-016).
package archive

import (
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/clock"
	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// Entry represents a generated archive entry.
type Entry struct {
	Filename string
	Content  string
}

// GenerateEntry creates an archive Markdown entry from a completed node's state.
// An optional clock may be provided; when omitted the real system clock is used.
func GenerateEntry(nodeAddr string, ns *state.NodeState, cfg *config.Config, branch string, summary string, clocks ...clock.Clock) *Entry {
	clk := resolveOptionalClock(clocks)
	now := clk.Now()
	slug := collapseHyphens(strings.ReplaceAll(nodeAddr, "/", "-"))
	if len(slug) > 80 {
		// Truncate at the last hyphen boundary before the limit
		cut := strings.LastIndex(slug[:80], "-")
		if cut > 0 {
			slug = slug[:cut]
		} else {
			slug = slug[:80]
		}
	}
	filename := fmt.Sprintf("%s-%s.md", now.Format("2006-01-02T15-04Z"), slug)

	var b strings.Builder

	fmt.Fprintf(&b, "# Archive: %s\n\n", nodeAddr)

	// Summary (if provided)
	if summary != "" {
		b.WriteString("## Summary\n\n")
		b.WriteString(summary)
		b.WriteString("\n\n")
	}

	// Breadcrumbs
	b.WriteString("## Breadcrumbs\n\n")
	if len(ns.Audit.Breadcrumbs) > 0 {
		for _, bc := range ns.Audit.Breadcrumbs {
			fmt.Fprintf(&b, "- **%s** [%s]: %s\n", bc.Task, bc.Timestamp.Format("2006-01-02T15:04Z"), bc.Text)
		}
	} else {
		b.WriteString("No breadcrumbs recorded.\n")
	}
	b.WriteString("\n")

	// Audit
	b.WriteString("## Audit\n\n")
	fmt.Fprintf(&b, "**Status:** %s\n\n", ns.Audit.Status)

	if ns.Audit.Scope != nil {
		b.WriteString("### Scope\n\n")
		b.WriteString(ns.Audit.Scope.Description + "\n\n")
		if len(ns.Audit.Scope.Criteria) > 0 {
			b.WriteString("**Criteria:**\n")
			for _, c := range ns.Audit.Scope.Criteria {
				fmt.Fprintf(&b, "- [x] %s\n", c)
			}
			b.WriteString("\n")
		}
	}

	if len(ns.Audit.Gaps) > 0 {
		b.WriteString("### Gaps\n\n")
		for _, g := range ns.Audit.Gaps {
			status := "OPEN"
			if g.Status == state.GapFixed {
				status = "FIXED"
			}
			fmt.Fprintf(&b, "- [%s] %s", status, g.Description)
			if g.FixedBy != "" {
				fmt.Fprintf(&b, " (fixed by %s)", g.FixedBy)
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(ns.Audit.Escalations) > 0 {
		b.WriteString("### Escalations\n\n")
		for _, e := range ns.Audit.Escalations {
			status := "OPEN"
			if e.Status == state.EscalationResolved {
				status = "RESOLVED"
			}
			fmt.Fprintf(&b, "- [%s] %s (from %s)\n", status, e.Description, e.SourceNode)
		}
		b.WriteString("\n")
	}

	if ns.Audit.ResultSummary != "" {
		b.WriteString("### Result\n\n")
		b.WriteString(ns.Audit.ResultSummary + "\n\n")
	}

	// Metadata
	b.WriteString("## Metadata\n\n")
	b.WriteString("| Field | Value |\n")
	b.WriteString("|-------|-------|\n")
	fmt.Fprintf(&b, "| Node | %s |\n", nodeAddr)
	completedAt := now.Format("2006-01-02T15:04Z")
	if ns.Audit.CompletedAt != nil {
		completedAt = ns.Audit.CompletedAt.Format("2006-01-02T15:04Z")
	}
	fmt.Fprintf(&b, "| Completed | %s |\n", completedAt)
	fmt.Fprintf(&b, "| Archived | %s |\n", now.Format("2006-01-02T15:04Z"))
	if cfg.Identity != nil {
		fmt.Fprintf(&b, "| Engineer | %s-%s |\n", cfg.Identity.User, cfg.Identity.Machine)
	}
	if branch != "" {
		fmt.Fprintf(&b, "| Branch | %s |\n", branch)
	}

	return &Entry{
		Filename: filename,
		Content:  b.String(),
	}
}

// resolveOptionalClock returns the first clock if provided, otherwise the real clock.
func resolveOptionalClock(clocks []clock.Clock) clock.Clock {
	if len(clocks) > 0 && clocks[0] != nil {
		return clocks[0]
	}
	return clock.New()
}

// collapseHyphens replaces consecutive hyphens with a single hyphen.
func collapseHyphens(s string) string {
	var b strings.Builder
	prev := byte(0)
	for i := 0; i < len(s); i++ {
		if s[i] == '-' && prev == '-' {
			continue
		}
		b.WriteByte(s[i])
		prev = s[i]
	}
	return b.String()
}
