package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RenderContext renders a task's context section as a formatted string suitable
// for inclusion in an iteration prompt. nodeAddr is the slash-delimited address
// of the parent node (e.g. "project/auth"); nodeDir, when non-empty, is the
// filesystem path to the node directory for reading per-task .md files.
//
// The output covers task metadata, deliverables, acceptance criteria,
// constraints, references (with inline spec content for small .md files),
// failure count, and last failure type. Failure headers and decomposition
// guidance are intentionally excluded; those belong to the ContextBuilder
// which has access to config thresholds.
func (t *Task) RenderContext(nodeAddr string, nodeDir string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "**Task:** %s/%s\n", nodeAddr, t.ID)
	fmt.Fprintf(&b, "**Description:** %s\n", t.Description)

	// Include task .md content if available
	if nodeDir != "" {
		mdPath := filepath.Join(nodeDir, t.ID+".md")
		if mdContent, err := os.ReadFile(mdPath); err == nil {
			content := strings.TrimSpace(string(mdContent))
			if content != "" {
				b.WriteString("\n" + content + "\n\n")
			}
		}
	}

	if t.TaskType != "" {
		fmt.Fprintf(&b, "**Task Type:** %s\n", t.TaskType)
	}
	if t.Body != "" {
		b.WriteString("\n## Task Details\n\n")
		b.WriteString(t.Body + "\n")
	}
	if t.Integration != "" {
		b.WriteString("\n## Integration\n\n")
		b.WriteString(t.Integration + "\n")
	}
	if len(t.Deliverables) > 0 {
		fmt.Fprintf(&b, "\n**Deliverables:**\n")
		for _, d := range t.Deliverables {
			fmt.Fprintf(&b, "- `%s`\n", d)
		}
	}
	if len(t.AcceptanceCriteria) > 0 {
		fmt.Fprintf(&b, "\n**Acceptance Criteria:**\n")
		for _, ac := range t.AcceptanceCriteria {
			fmt.Fprintf(&b, "- %s\n", ac)
		}
	}
	if len(t.Constraints) > 0 {
		fmt.Fprintf(&b, "\n**Constraints:**\n")
		for _, c := range t.Constraints {
			fmt.Fprintf(&b, "- %s\n", c)
		}
	}
	if len(t.References) > 0 {
		fmt.Fprintf(&b, "\n**Reference Material:**\n")
		for _, r := range t.References {
			fmt.Fprintf(&b, "- `%s`\n", r)
		}
		// Inline spec content when references point to readable files
		for _, r := range t.References {
			if strings.HasSuffix(r, ".md") {
				if content, err := os.ReadFile(r); err == nil {
					trimmed := strings.TrimSpace(string(content))
					if len(trimmed) > 0 && len(trimmed) < 8000 {
						fmt.Fprintf(&b, "\n### Reference: %s\n\n%s\n", r, trimmed)
					}
				}
			}
		}
	}
	fmt.Fprintf(&b, "\n**Task State:** %s\n", t.State)
	if t.FailureCount > 0 {
		fmt.Fprintf(&b, "**Failure Count:** %d\n", t.FailureCount)
		if t.LastFailureType != "" {
			fmt.Fprintf(&b, "\n## Previous Attempt Failed\n\n")
			switch t.LastFailureType {
			case "no_terminal_marker":
				fmt.Fprintf(&b, "The previous attempt did not emit a terminal marker (WOLFCASTLE_COMPLETE, WOLFCASTLE_SKIP, WOLFCASTLE_BLOCKED, or WOLFCASTLE_YIELD). Make sure to emit exactly one terminal marker when done.\n")
			case "no_progress":
				fmt.Fprintf(&b, "The previous attempt emitted WOLFCASTLE_COMPLETE but no git changes were detected. You must commit your changes before signaling completion.\n")
			default:
				fmt.Fprintf(&b, "The previous attempt failed with reason: %s\n", t.LastFailureType)
			}
		}
	}

	return b.String()
}

// RenderContext renders the audit state (breadcrumbs and scope) as a formatted
// string for inclusion in an iteration prompt. Returns empty string when there
// are no breadcrumbs and no scope.
func (a *AuditState) RenderContext() string {
	hasBreadcrumbs := len(a.Breadcrumbs) > 0
	hasScope := a.Scope != nil

	if !hasBreadcrumbs && !hasScope {
		return ""
	}

	var b strings.Builder

	if hasBreadcrumbs {
		b.WriteString("\n## Recent Breadcrumbs\n\n")
		start := 0
		if len(a.Breadcrumbs) > 10 {
			start = len(a.Breadcrumbs) - 10
		}
		for _, bc := range a.Breadcrumbs[start:] {
			fmt.Fprintf(&b, "- [%s] %s: %s\n", bc.Timestamp.Format("2006-01-02T15:04Z"), bc.Task, bc.Text)
		}
	}

	if hasScope {
		b.WriteString("\n## Audit Scope\n\n")
		b.WriteString(a.Scope.Description + "\n")
	}

	return b.String()
}
