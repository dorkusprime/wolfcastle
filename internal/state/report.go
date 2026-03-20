package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// GenerateAuditReport produces a markdown report summarizing the audit state
// for a given node. The nodeAddr identifies the node, and the timestamp
// determines the report filename. The report is written to the node's
// directory within the store.
func GenerateAuditReport(audit AuditState, nodeAddr string, nodeName string) string {
	var b strings.Builder

	// Header
	fmt.Fprintf(&b, "# Audit Report: %s\n\n", nodeName)
	fmt.Fprintf(&b, "**Node:** `%s`\n\n", nodeAddr)

	if audit.CompletedAt != nil {
		fmt.Fprintf(&b, "**Completed:** %s\n\n", audit.CompletedAt.Format(time.RFC3339))
	} else if audit.StartedAt != nil {
		fmt.Fprintf(&b, "**Started:** %s\n\n", audit.StartedAt.Format(time.RFC3339))
	}

	// Verdict
	fmt.Fprintf(&b, "**Verdict:** %s\n\n", verdictLabel(audit.Status))

	// Scope
	if audit.Scope != nil && audit.Scope.Description != "" {
		fmt.Fprintf(&b, "## Scope\n\n")
		fmt.Fprintf(&b, "%s\n\n", audit.Scope.Description)
		if len(audit.Scope.Files) > 0 {
			fmt.Fprintf(&b, "**Files:** %s\n\n", strings.Join(audit.Scope.Files, ", "))
		}
		if len(audit.Scope.Systems) > 0 {
			fmt.Fprintf(&b, "**Systems:** %s\n\n", strings.Join(audit.Scope.Systems, ", "))
		}
		if len(audit.Scope.Criteria) > 0 {
			fmt.Fprintf(&b, "**Criteria:**\n\n")
			for _, c := range audit.Scope.Criteria {
				fmt.Fprintf(&b, "- %s\n", c)
			}
			b.WriteString("\n")
		}
	}

	// Summary
	if audit.ResultSummary != "" {
		fmt.Fprintf(&b, "## Summary\n\n%s\n\n", audit.ResultSummary)
	}

	// Findings (gaps)
	if len(audit.Gaps) > 0 {
		open, fixed := countGaps(audit.Gaps)
		fmt.Fprintf(&b, "## Findings\n\n")
		fmt.Fprintf(&b, "%d total (%d remediated, %d open)\n\n", len(audit.Gaps), fixed, open)

		if open > 0 {
			fmt.Fprintf(&b, "### Open\n\n")
			for _, g := range audit.Gaps {
				if g.Status == GapOpen {
					fmt.Fprintf(&b, "- **%s** (%s): %s\n", g.ID, g.Source, g.Description)
				}
			}
			b.WriteString("\n")
		}

		if fixed > 0 {
			fmt.Fprintf(&b, "### Remediated\n\n")
			for _, g := range audit.Gaps {
				if g.Status == GapFixed {
					fixInfo := ""
					if g.FixedBy != "" {
						fixInfo = fmt.Sprintf(" (fixed by %s)", g.FixedBy)
					}
					fmt.Fprintf(&b, "- **%s**: %s%s\n", g.ID, g.Description, fixInfo)
				}
			}
			b.WriteString("\n")
		}
	}

	// Escalations
	if len(audit.Escalations) > 0 {
		fmt.Fprintf(&b, "## Escalations\n\n")
		for _, e := range audit.Escalations {
			status := "OPEN"
			if e.Status == EscalationResolved {
				status = "RESOLVED"
			}
			fmt.Fprintf(&b, "- **%s** [%s] from `%s`: %s\n", e.ID, status, e.SourceNode, e.Description)
		}
		b.WriteString("\n")
	}

	// Breadcrumbs (audit trail)
	if len(audit.Breadcrumbs) > 0 {
		fmt.Fprintf(&b, "## Audit Trail\n\n")
		for _, bc := range audit.Breadcrumbs {
			fmt.Fprintf(&b, "- `%s` [%s]: %s\n", bc.Task, bc.Timestamp.Format("2006-01-02 15:04"), bc.Text)
		}
		b.WriteString("\n")
	}

	return b.String()
}

// WriteAuditReport generates and writes an audit report to the node's
// directory. The filename uses the provided timestamp for uniqueness.
// Returns the path to the written report file.
func WriteAuditReport(storeDir string, nodeAddr string, audit AuditState, nodeName string, ts time.Time) (string, error) {
	parts := strings.Split(nodeAddr, "/")
	nodeDir := filepath.Join(storeDir, filepath.Join(parts...))

	if err := os.MkdirAll(nodeDir, 0755); err != nil {
		return "", fmt.Errorf("creating node directory: %w", err)
	}

	filename := fmt.Sprintf("audit-%s.md", ts.Format("2006-01-02T15-04"))
	reportPath := filepath.Join(nodeDir, filename)

	content := GenerateAuditReport(audit, nodeAddr, nodeName)

	if err := os.WriteFile(reportPath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("writing audit report: %w", err)
	}

	return reportPath, nil
}

// LatestAuditReport returns the path to the most recent audit report in
// a node's directory, or empty string if none exists.
func LatestAuditReport(storeDir string, nodeAddr string) string {
	parts := strings.Split(nodeAddr, "/")
	nodeDir := filepath.Join(storeDir, filepath.Join(parts...))

	entries, err := os.ReadDir(nodeDir)
	if err != nil {
		return ""
	}

	var latest string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "audit-") && strings.HasSuffix(name, ".md") {
			// Lexicographic comparison works because the timestamp format sorts correctly
			if name > latest {
				latest = name
			}
		}
	}

	if latest == "" {
		return ""
	}
	return filepath.Join(nodeDir, latest)
}

func verdictLabel(s AuditStatus) string {
	switch s {
	case AuditPassed:
		return "PASSED"
	case AuditFailed:
		return "FAILED"
	case AuditInProgress:
		return "IN PROGRESS"
	default:
		return "PENDING"
	}
}

func countGaps(gaps []Gap) (open, fixed int) {
	for _, g := range gaps {
		switch g.Status {
		case GapOpen:
			open++
		case GapFixed:
			fixed++
		}
	}
	return
}
