package pipeline

import (
	"fmt"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// BuildIterationContext creates the iteration context section for the execute stage.
// cfg may be nil for backward compatibility (no failure policy context).
func BuildIterationContext(nodeAddr string, ns *state.NodeState, taskID string, cfgs ...*config.Config) string {
	var cfg *config.Config
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	var b strings.Builder

	b.WriteString(fmt.Sprintf("**Node:** %s\n", nodeAddr))
	b.WriteString(fmt.Sprintf("**Node Type:** %s\n", ns.Type))
	b.WriteString(fmt.Sprintf("**Node State:** %s\n\n", ns.State))

	// Find the target task and emit context (single pass)
	var taskFound bool
	for _, t := range ns.Tasks {
		if t.ID != taskID {
			continue
		}
		taskFound = true
		b.WriteString(fmt.Sprintf("**Task:** %s/%s\n", nodeAddr, t.ID))
		b.WriteString(fmt.Sprintf("**Description:** %s\n", t.Description))
		b.WriteString(fmt.Sprintf("**Task State:** %s\n", t.State))
		if t.FailureCount > 0 {
			b.WriteString(fmt.Sprintf("**Failure Count:** %d\n", t.FailureCount))
		}

		// Failure history and decomposition policy
		if t.FailureCount > 0 && cfg != nil {
			b.WriteString("\n## Failure History\n\n")
			b.WriteString(fmt.Sprintf("This task has failed %d times.\n", t.FailureCount))
			b.WriteString(fmt.Sprintf("- Decomposition threshold: %d\n", cfg.Failure.DecompositionThreshold))
			b.WriteString(fmt.Sprintf("- Max decomposition depth: %d (current: %d)\n", cfg.Failure.MaxDecompositionDepth, ns.DecompositionDepth))
			b.WriteString(fmt.Sprintf("- Hard failure cap: %d\n", cfg.Failure.HardCap))
			if t.NeedsDecomposition {
				b.WriteString("\n**Decomposition required.** This task has failed too many times to continue as-is.\n")
				b.WriteString("Break this leaf into smaller sub-tasks using the wolfcastle CLI:\n\n")
				b.WriteString(fmt.Sprintf("1. Create child nodes: `wolfcastle project create --node %s --type leaf \"<name>\"`\n", nodeAddr))
				b.WriteString(fmt.Sprintf("2. Add tasks to each child: `wolfcastle task add --node %s/<child-slug> \"<description>\"`\n", nodeAddr))
				b.WriteString("3. Emit WOLFCASTLE_YIELD when decomposition is complete.\n\n")
				b.WriteString("The parent node will automatically convert from leaf to orchestrator when the first child is created.\n")
			}
		}
		break
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

	// Summary guidance — when this is the last incomplete task in the node,
	// instruct the model to include a summary marker with WOLFCASTLE_COMPLETE.
	if taskFound && isLastIncompleteTask(ns, taskID) {
		b.WriteString("\n## Summary Required\n\n")
		b.WriteString("This is the last incomplete task in this node. When you complete it, ")
		b.WriteString("include a summary of all work done in this node using:\n\n")
		b.WriteString("`WOLFCASTLE_SUMMARY: <one-paragraph summary of what was accomplished>`\n\n")
		b.WriteString("Emit this on its own line before WOLFCASTLE_COMPLETE.\n")
	}

	return b.String()
}

// isLastIncompleteTask returns true if taskID is the only non-complete task
// remaining in the node (excluding itself). Returns false if taskID is not found.
func isLastIncompleteTask(ns *state.NodeState, taskID string) bool {
	found := false
	for _, t := range ns.Tasks {
		if t.ID == taskID {
			found = true
			continue
		}
		if t.State != state.StatusComplete {
			return false
		}
	}
	return found
}
