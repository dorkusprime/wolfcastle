package pipeline

import (
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

// BuildIterationContext creates the iteration context section for the execute stage.
func BuildIterationContext(nodeAddr string, ns *state.NodeState, taskID string) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("**Node:** %s\n", nodeAddr))
	b.WriteString(fmt.Sprintf("**Node Type:** %s\n", ns.Type))
	b.WriteString(fmt.Sprintf("**Node State:** %s\n\n", ns.State))

	// Find the target task
	for _, t := range ns.Tasks {
		if t.ID == taskID {
			b.WriteString(fmt.Sprintf("**Task:** %s/%s\n", nodeAddr, t.ID))
			b.WriteString(fmt.Sprintf("**Description:** %s\n", t.Description))
			b.WriteString(fmt.Sprintf("**Task State:** %s\n", t.State))
			if t.FailureCount > 0 {
				b.WriteString(fmt.Sprintf("**Failure Count:** %d\n", t.FailureCount))
			}
			break
		}
	}

	// Audit breadcrumbs (recent)
	if len(ns.Audit.Breadcrumbs) > 0 {
		b.WriteString("\n## Recent Breadcrumbs\n\n")
		start := 0
		if len(ns.Audit.Breadcrumbs) > 10 {
			start = len(ns.Audit.Breadcrumbs) - 10
		}
		for _, bc := range ns.Audit.Breadcrumbs[start:] {
			b.WriteString(fmt.Sprintf("- [%s] %s: %s\n", bc.Timestamp.Format("2006-01-02T15:04Z"), bc.Task, bc.Text))
		}
	}

	// Audit scope
	if ns.Audit.Scope != nil {
		b.WriteString("\n## Audit Scope\n\n")
		b.WriteString(ns.Audit.Scope.Description + "\n")
	}

	// Specs
	if len(ns.Specs) > 0 {
		b.WriteString("\n## Linked Specs\n\n")
		for _, s := range ns.Specs {
			b.WriteString(fmt.Sprintf("- %s\n", s))
		}
	}

	return b.String()
}
