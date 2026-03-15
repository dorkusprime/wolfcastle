package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// FailureHeaderContext holds template variables for context-headers.md.
type FailureHeaderContext struct {
	FailureCount    int
	DecompThreshold int
	MaxDecompDepth  int
	CurrentDepth    int
	HardCap         int
}

// DecompositionContext holds template variables for decomposition.md.
type DecompositionContext struct {
	NodeAddr string
}

// BuildIterationContext creates the iteration context section for the execute stage.
// cfg may be nil for backward compatibility (no failure policy context).
// wolfcastleDir is optional — when provided, instructional text is loaded from
// externalized prompt templates via the three-tier resolution system.
func BuildIterationContext(nodeAddr string, ns *state.NodeState, taskID string, cfgs ...*config.Config) string {
	return BuildIterationContextWithDir("", nodeAddr, ns, taskID, cfgs...)
}

// BuildIterationContextWithDir is like BuildIterationContext but accepts a
// wolfcastleDir for loading externalized prompt templates and an optional
// nodeDir for reading task markdown files.
func BuildIterationContextWithDir(wolfcastleDir string, nodeAddr string, ns *state.NodeState, taskID string, cfgs ...*config.Config) string {
	return buildIterationContext(wolfcastleDir, "", nodeAddr, ns, taskID, cfgs...)
}

// BuildIterationContextFull is like BuildIterationContextWithDir but also
// accepts a nodeDir for reading per-task .md files.
func BuildIterationContextFull(wolfcastleDir string, nodeDir string, nodeAddr string, ns *state.NodeState, taskID string, cfgs ...*config.Config) string {
	return buildIterationContext(wolfcastleDir, nodeDir, nodeAddr, ns, taskID, cfgs...)
}

func buildIterationContext(wolfcastleDir string, nodeDir string, nodeAddr string, ns *state.NodeState, taskID string, cfgs ...*config.Config) string {
	var cfg *config.Config
	if len(cfgs) > 0 {
		cfg = cfgs[0]
	}
	var b strings.Builder

	fmt.Fprintf(&b, "**Node:** %s\n", nodeAddr)
	fmt.Fprintf(&b, "**Node Type:** %s\n", ns.Type)
	fmt.Fprintf(&b, "**Node State:** %s\n\n", ns.State)

	// Find the target task and emit context (single pass)
	var taskFound bool
	for _, t := range ns.Tasks {
		if t.ID != taskID {
			continue
		}
		taskFound = true
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

		if len(t.Deliverables) > 0 {
			fmt.Fprintf(&b, "**Deliverables:**\n")
			for _, d := range t.Deliverables {
				fmt.Fprintf(&b, "- `%s`\n", d)
			}
		}
		fmt.Fprintf(&b, "**Task State:** %s\n", t.State)
		if t.FailureCount > 0 {
			fmt.Fprintf(&b, "**Failure Count:** %d\n", t.FailureCount)
		}

		// Failure history and decomposition policy
		if t.FailureCount > 0 && cfg != nil {
			headerCtx := FailureHeaderContext{
				FailureCount:    t.FailureCount,
				DecompThreshold: cfg.Failure.DecompositionThreshold,
				MaxDecompDepth:  cfg.Failure.MaxDecompositionDepth,
				CurrentDepth:    ns.DecompositionDepth,
				HardCap:         cfg.Failure.HardCap,
			}
			header := renderFailureHeader(wolfcastleDir, headerCtx)
			b.WriteString("\n" + header)

			if t.NeedsDecomposition {
				decompCtx := DecompositionContext{NodeAddr: nodeAddr}
				decomp := renderDecomposition(wolfcastleDir, decompCtx)
				b.WriteString("\n" + decomp)
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
			fmt.Fprintf(&b, "- [%s] %s: %s\n", bc.Timestamp.Format("2006-01-02T15:04Z"), bc.Task, bc.Text)
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
			fmt.Fprintf(&b, "- %s\n", s)
		}
	}

	// Summary guidance — when this is the last incomplete task in the node,
	// instruct the model to include a summary marker with WOLFCASTLE_COMPLETE.
	if taskFound && isLastIncompleteTask(ns, taskID) {
		summary := renderSummaryRequired(wolfcastleDir)
		b.WriteString("\n## Summary Required\n\n")
		b.WriteString(summary)
	}

	return b.String()
}

// renderFailureHeader loads the context-headers.md template or falls back to
// a hardcoded default when wolfcastleDir is empty or loading fails.
func renderFailureHeader(wolfcastleDir string, ctx FailureHeaderContext) string {
	if wolfcastleDir != "" {
		rendered, err := ResolvePromptTemplate(wolfcastleDir, "context-headers.md", ctx)
		if err == nil {
			return rendered
		}
	}
	// Fallback
	var b strings.Builder
	b.WriteString("## Failure History\n\n")
	fmt.Fprintf(&b, "This task has failed %d times.\n", ctx.FailureCount)
	fmt.Fprintf(&b, "- Decomposition threshold: %d\n", ctx.DecompThreshold)
	fmt.Fprintf(&b, "- Max decomposition depth: %d (current: %d)\n", ctx.MaxDecompDepth, ctx.CurrentDepth)
	fmt.Fprintf(&b, "- Hard failure cap: %d\n", ctx.HardCap)
	return b.String()
}

// renderDecomposition loads decomposition.md or falls back to hardcoded text.
func renderDecomposition(wolfcastleDir string, ctx DecompositionContext) string {
	if wolfcastleDir != "" {
		rendered, err := ResolvePromptTemplate(wolfcastleDir, "decomposition.md", ctx)
		if err == nil {
			return rendered
		}
	}
	// Fallback
	var b strings.Builder
	b.WriteString("**Decomposition required.** This task has failed too many times to continue as-is.\n")
	b.WriteString("Break this leaf into smaller sub-tasks using the wolfcastle CLI:\n\n")
	fmt.Fprintf(&b, "1. Create child nodes: `wolfcastle project create --node %s --type leaf \"<name>\"`\n", ctx.NodeAddr)
	fmt.Fprintf(&b, "2. Add tasks to each child: `wolfcastle task add --node %s/<child-slug> \"<description>\"`\n", ctx.NodeAddr)
	b.WriteString("3. Emit WOLFCASTLE_YIELD when decomposition is complete.\n\n")
	b.WriteString("The parent node will automatically convert from leaf to orchestrator when the first child is created.\n")
	return b.String()
}

// renderSummaryRequired loads summary-required.md or falls back to hardcoded text.
func renderSummaryRequired(wolfcastleDir string) string {
	if wolfcastleDir != "" {
		rendered, err := ResolvePromptTemplate(wolfcastleDir, "summary-required.md", nil)
		if err == nil {
			return rendered
		}
	}
	// Fallback
	var b strings.Builder
	b.WriteString("This is the last incomplete task in this node. When you complete it, ")
	b.WriteString("include a summary of all work done in this node:\n\n")
	b.WriteString("`wolfcastle audit summary --node <your-node> \"one-paragraph summary of what was accomplished\"`\n\n")
	b.WriteString("Run this command before emitting WOLFCASTLE_COMPLETE.\n")
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
