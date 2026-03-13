package archive

import (
	"fmt"
	"strings"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// Entry represents a generated archive entry.
type Entry struct {
	Filename string
	Content  string
}

// GenerateEntry creates an archive Markdown entry from a completed node's state.
func GenerateEntry(nodeAddr string, ns *state.NodeState, cfg *config.Config, branch string, summary string) *Entry {
	now := time.Now().UTC()
	slug := strings.ReplaceAll(nodeAddr, "/", "-")
	filename := fmt.Sprintf("%s-%s-complete.md", now.Format("2006-01-02T15-04Z"), slug)

	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Archive: %s\n\n", nodeAddr))

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
			b.WriteString(fmt.Sprintf("- **%s** [%s]: %s\n", bc.Task, bc.Timestamp.Format("2006-01-02T15:04Z"), bc.Text))
		}
	} else {
		b.WriteString("No breadcrumbs recorded.\n")
	}
	b.WriteString("\n")

	// Audit
	b.WriteString("## Audit\n\n")
	b.WriteString(fmt.Sprintf("**Status:** %s\n\n", ns.Audit.Status))

	if ns.Audit.Scope != nil {
		b.WriteString("### Scope\n\n")
		b.WriteString(ns.Audit.Scope.Description + "\n\n")
		if len(ns.Audit.Scope.Criteria) > 0 {
			b.WriteString("**Criteria:**\n")
			for _, c := range ns.Audit.Scope.Criteria {
				b.WriteString(fmt.Sprintf("- [x] %s\n", c))
			}
			b.WriteString("\n")
		}
	}

	if len(ns.Audit.Gaps) > 0 {
		b.WriteString("### Gaps\n\n")
		for _, g := range ns.Audit.Gaps {
			status := "OPEN"
			if g.Status == "fixed" {
				status = "FIXED"
			}
			b.WriteString(fmt.Sprintf("- [%s] %s", status, g.Description))
			if g.FixedBy != "" {
				b.WriteString(fmt.Sprintf(" (fixed by %s)", g.FixedBy))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if len(ns.Audit.Escalations) > 0 {
		b.WriteString("### Escalations\n\n")
		for _, e := range ns.Audit.Escalations {
			status := "OPEN"
			if e.Status == "resolved" {
				status = "RESOLVED"
			}
			b.WriteString(fmt.Sprintf("- [%s] %s (from %s)\n", status, e.Description, e.SourceNode))
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
	b.WriteString(fmt.Sprintf("| Node | %s |\n", nodeAddr))
	b.WriteString(fmt.Sprintf("| Completed | %s |\n", now.Format("2006-01-02T15:04Z")))
	b.WriteString(fmt.Sprintf("| Archived | %s |\n", now.Format("2006-01-02T15:04Z")))
	if cfg.Identity != nil {
		b.WriteString(fmt.Sprintf("| Engineer | %s-%s |\n", cfg.Identity.User, cfg.Identity.Machine))
	}
	if branch != "" {
		b.WriteString(fmt.Sprintf("| Branch | %s |\n", branch))
	}

	return &Entry{
		Filename: filename,
		Content:  b.String(),
	}
}
