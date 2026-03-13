package pipeline

import (
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func TestBuildIterationContext_IncludesNodeInfo(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.State = state.StatusInProgress

	result := BuildIterationContext("project/auth", ns, "task-1")

	if !strings.Contains(result, "**Node:** project/auth") {
		t.Error("expected node address")
	}
	if !strings.Contains(result, "**Node Type:** leaf") {
		t.Error("expected node type")
	}
	if !strings.Contains(result, "**Node State:** in_progress") {
		t.Error("expected node state")
	}
}

func TestBuildIterationContext_IncludesTaskDescription(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Tasks = []state.Task{
		{ID: "task-1", Description: "Implement JWT validation", State: state.StatusInProgress},
	}

	result := BuildIterationContext("project/auth", ns, "task-1")

	if !strings.Contains(result, "**Task:** project/auth/task-1") {
		t.Error("expected task address")
	}
	if !strings.Contains(result, "**Description:** Implement JWT validation") {
		t.Error("expected task description")
	}
}

func TestBuildIterationContext_BreadcrumbsLimitedTo10(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)

	// Add 15 breadcrumbs
	for i := 0; i < 15; i++ {
		ns.Audit.Breadcrumbs = append(ns.Audit.Breadcrumbs, state.Breadcrumb{
			Timestamp: time.Date(2026, 1, 1, i, 0, 0, 0, time.UTC),
			Task:      "task-1",
			Text:      strings.Repeat("x", 1) + string(rune('a'+i)),
		})
	}

	result := BuildIterationContext("project/auth", ns, "task-1")

	if !strings.Contains(result, "## Recent Breadcrumbs") {
		t.Error("expected breadcrumbs section")
	}

	// Should only contain the last 10 breadcrumbs (indices 5-14)
	lines := strings.Split(result, "\n")
	breadcrumbLines := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "- [") {
			breadcrumbLines++
		}
	}
	if breadcrumbLines != 10 {
		t.Errorf("expected 10 breadcrumb lines, got %d", breadcrumbLines)
	}
}

func TestBuildIterationContext_IncludesAuditScope(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Audit.Scope = &state.AuditScope{
		Description: "Verify all auth endpoints",
	}

	result := BuildIterationContext("project/auth", ns, "task-1")

	if !strings.Contains(result, "## Audit Scope") {
		t.Error("expected audit scope section")
	}
	if !strings.Contains(result, "Verify all auth endpoints") {
		t.Error("expected scope description")
	}
}

func TestBuildIterationContext_IncludesSpecs(t *testing.T) {
	t.Parallel()
	ns := state.NewNodeState("auth", "Auth Module", state.NodeLeaf)
	ns.Specs = []string{"spec-auth-flow.md", "spec-jwt-format.md"}

	result := BuildIterationContext("project/auth", ns, "task-1")

	if !strings.Contains(result, "## Linked Specs") {
		t.Error("expected specs section")
	}
	if !strings.Contains(result, "spec-auth-flow.md") {
		t.Error("expected first spec")
	}
	if !strings.Contains(result, "spec-jwt-format.md") {
		t.Error("expected second spec")
	}
}
