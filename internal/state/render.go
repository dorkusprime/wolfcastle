package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/invoke"
)

// RenderContext renders a task's context section as a formatted string suitable
// for inclusion in an iteration prompt. The output covers task metadata,
// deliverables, acceptance criteria, constraints, references (with inline spec
// content for small .md files), failure count, and last failure type. Failure
// headers and decomposition guidance are intentionally excluded; those belong
// to the ContextBuilder which has access to config thresholds.
//
// The task address line renders only the task ID; the full node-qualified
// address is provided by NodeState.RenderContext or ContextBuilder. Per-task
// .md file content is likewise the ContextBuilder's responsibility, since it
// requires filesystem access that a domain type should not perform.
func (t *Task) RenderContext() string {
	var b strings.Builder

	fmt.Fprintf(&b, "**Task:** %s\n", t.ID)
	fmt.Fprintf(&b, "**Description:** %s\n", t.Description)

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
		// Inline spec content when references point to readable .md files.
		// Reject paths containing ".." to prevent traversal outside the
		// project directory.
		for _, r := range t.References {
			if strings.HasSuffix(r, ".md") && !strings.Contains(filepath.Clean(r), "..") {
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
				fmt.Fprintf(&b, "The previous attempt did not emit a terminal marker (%s, %s, %s, or %s). Make sure to emit exactly one terminal marker when done.\n",
					invoke.MarkerStringComplete, invoke.MarkerStringSkip, invoke.MarkerStringBlocked, invoke.MarkerStringYield)
			case "no_progress":
				fmt.Fprintf(&b, "The previous attempt emitted %s but no git changes were detected. You must commit your changes before signaling completion.\n", invoke.MarkerStringComplete)
			default:
				fmt.Fprintf(&b, "The previous attempt failed with reason: %s\n", t.LastFailureType)
			}
		}
	}

	return b.String()
}

// RenderContext renders node-level context as a formatted string suitable for
// inclusion in an iteration prompt. It emits node metadata (type and state)
// and any linked specs. The full node address is not rendered here; the
// ContextBuilder provides it when composing the final context. The taskID
// parameter identifies the active task so the caller can pair this output
// with the corresponding Task.RenderContext and AuditState.RenderContext.
func (ns *NodeState) RenderContext(taskID string) string {
	var b strings.Builder

	fmt.Fprintf(&b, "**Node Type:** %s\n", ns.Type)
	fmt.Fprintf(&b, "**Node State:** %s\n", ns.State)

	if len(ns.Specs) > 0 {
		b.WriteString("\n## Linked Specs\n\n")
		for _, s := range ns.Specs {
			fmt.Fprintf(&b, "- %s\n", s)
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

// RenderAARs produces a formatted Markdown section summarizing prior After
// Action Reviews. The AARs are sorted by timestamp so the narrative reads
// chronologically. Returns empty string when there are no AARs.
func RenderAARs(aars map[string]AAR) string {
	if len(aars) == 0 {
		return ""
	}

	// Sort by timestamp for chronological ordering.
	sorted := make([]AAR, 0, len(aars))
	for _, aar := range aars {
		sorted = append(sorted, aar)
	}
	// Simple insertion sort; AAR counts are small.
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].Timestamp.Before(sorted[j-1].Timestamp); j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	var b strings.Builder
	b.WriteString("\n## Prior Task Reviews (AARs)\n\n")

	for _, aar := range sorted {
		fmt.Fprintf(&b, "### %s\n\n", aar.TaskID)
		fmt.Fprintf(&b, "**Objective:** %s\n", aar.Objective)
		fmt.Fprintf(&b, "**What happened:** %s\n", aar.WhatHappened)
		if len(aar.WentWell) > 0 {
			b.WriteString("**Went well:**\n")
			for _, item := range aar.WentWell {
				fmt.Fprintf(&b, "- %s\n", item)
			}
		}
		if len(aar.Improvements) > 0 {
			b.WriteString("**Improvements:**\n")
			for _, item := range aar.Improvements {
				fmt.Fprintf(&b, "- %s\n", item)
			}
		}
		if len(aar.ActionItems) > 0 {
			b.WriteString("**Action items:**\n")
			for _, item := range aar.ActionItems {
				fmt.Fprintf(&b, "- %s\n", item)
			}
		}
		b.WriteString("\n")
	}

	return b.String()
}
