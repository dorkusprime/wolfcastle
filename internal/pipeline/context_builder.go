package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ContextBuilder composes iteration context strings by delegating section
// rendering to domain types (NodeState, Task, AuditState) and layering in
// class guidance and prompt-template-backed sections. It holds two repository
// dependencies and no mutable state; safe for concurrent use.
type ContextBuilder struct {
	prompts *PromptRepository
	classes *ClassRepository
}

// NewContextBuilder creates a ContextBuilder. Both repositories are required;
// panics if either is nil.
func NewContextBuilder(prompts *PromptRepository, classes *ClassRepository) *ContextBuilder {
	if prompts == nil {
		panic("pipeline: NewContextBuilder requires a non-nil PromptRepository")
	}
	if classes == nil {
		panic("pipeline: NewContextBuilder requires a non-nil ClassRepository")
	}
	return &ContextBuilder{prompts: prompts, classes: classes}
}

// Build assembles the complete iteration context for a single task within a
// node. The returned Markdown string is ready for inclusion in the
// execute-stage prompt. nodeDir is optional; when non-empty, per-task .md
// files are read from it. cfg may be nil; failure context is skipped when nil.
func (cb *ContextBuilder) Build(nodeAddr string, nodeDir string, ns *state.NodeState, taskID string, cfg *config.Config) string {
	var b strings.Builder

	// 1. Node address header
	fmt.Fprintf(&b, "**Node:** %s\n", nodeAddr)

	// 2. Node context (type, state, linked specs)
	b.WriteString(ns.RenderContext(taskID))

	// 3. Task context
	task := findTask(ns, taskID)
	if task != nil {
		// Prefix the task ID with the full node address
		fmt.Fprintf(&b, "\n**Task:** %s/%s\n", nodeAddr, task.ID)
		// Task.RenderContext emits **Task:** with just the ID; we replace it
		// by writing the node-qualified line above and stripping the duplicate.
		taskCtx := task.RenderContext()
		if cut := "**Task:** " + task.ID + "\n"; strings.HasPrefix(taskCtx, cut) {
			taskCtx = taskCtx[len(cut):]
		}

		// Insert per-task .md file content before the rest of the task context.
		if nodeDir != "" {
			mdPath := filepath.Join(nodeDir, task.ID+".md")
			if mdContent, err := os.ReadFile(mdPath); err == nil {
				content := strings.TrimSpace(string(mdContent))
				if content != "" {
					// The task description line is already written; inject the
					// .md content before the remaining task metadata.
					descLine := "**Description:** " + task.Description + "\n"
					if idx := strings.Index(taskCtx, descLine); idx >= 0 {
						after := idx + len(descLine)
						b.WriteString(taskCtx[:after])
						b.WriteString("\n" + content + "\n\n")
						taskCtx = taskCtx[after:]
					}
				}
			}
		}
		b.WriteString(taskCtx)

		// 4. Class guidance
		if task.Class != "" {
			if guidance, err := cb.classes.Resolve(task.Class); err == nil {
				b.WriteString("\n## Class Guidance\n\n")
				b.WriteString(guidance)
				if !strings.HasSuffix(guidance, "\n") {
					b.WriteString("\n")
				}
			}
		}
	}

	// 5. Audit context (breadcrumbs, scope)
	b.WriteString(ns.Audit.RenderContext())

	// 6. Summary requirement
	if task != nil && cb.shouldIncludeSummary(ns, taskID) {
		summary := cb.renderSummaryRequired()
		b.WriteString("\n## Summary Required\n\n")
		b.WriteString(summary)
	}

	// 7. Failure context
	if task != nil && task.FailureCount > 0 && cfg != nil {
		failCtx := cb.renderFailureContext(nodeAddr, task, ns.DecompositionDepth, cfg)
		b.WriteString("\n" + failCtx)
	}

	return b.String()
}

// findTask locates a task by ID within the node's task list.
func findTask(ns *state.NodeState, taskID string) *state.Task {
	for i := range ns.Tasks {
		if ns.Tasks[i].ID == taskID {
			return &ns.Tasks[i]
		}
	}
	return nil
}

// shouldIncludeSummary returns true when taskID is the only non-complete task
// remaining in the node.
func (cb *ContextBuilder) shouldIncludeSummary(ns *state.NodeState, taskID string) bool {
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

// renderSummaryRequired loads summary-required.md via PromptRepository or
// falls back to hardcoded text.
func (cb *ContextBuilder) renderSummaryRequired() string {
	rendered, err := cb.prompts.Resolve("summary-required", nil)
	if err == nil {
		return rendered
	}
	var b strings.Builder
	b.WriteString("This is the last incomplete task in this node. When you complete it, ")
	b.WriteString("include a summary of all work done in this node:\n\n")
	b.WriteString("`wolfcastle audit summary --node <your-node> \"one-paragraph summary of what was accomplished\"`\n\n")
	fmt.Fprintf(&b, "Run this command before emitting %s.\n", invoke.MarkerStringComplete)
	return b.String()
}

// renderFailureContext produces the failure history header and optional
// decomposition guidance.
func (cb *ContextBuilder) renderFailureContext(nodeAddr string, task *state.Task, currentDepth int, cfg *config.Config) string {
	var b strings.Builder

	// Failure header
	headerCtx := FailureHeaderContext{
		FailureCount:    task.FailureCount,
		DecompThreshold: cfg.Failure.DecompositionThreshold,
		MaxDecompDepth:  cfg.Failure.MaxDecompositionDepth,
		CurrentDepth:    currentDepth,
		HardCap:         cfg.Failure.HardCap,
	}
	rendered, err := cb.prompts.Resolve("context-headers", headerCtx)
	if err == nil {
		b.WriteString(rendered)
	} else {
		b.WriteString("## Failure History\n\n")
		fmt.Fprintf(&b, "This task has failed %d times.\n", headerCtx.FailureCount)
		fmt.Fprintf(&b, "- Decomposition threshold: %d\n", headerCtx.DecompThreshold)
		fmt.Fprintf(&b, "- Max decomposition depth: %d (current: %d)\n", headerCtx.MaxDecompDepth, headerCtx.CurrentDepth)
		fmt.Fprintf(&b, "- Hard failure cap: %d\n", headerCtx.HardCap)
	}

	// Decomposition guidance
	if task.NeedsDecomposition {
		decompCtx := DecompositionContext{NodeAddr: nodeAddr}
		rendered, err := cb.prompts.Resolve("decomposition", decompCtx)
		if err == nil {
			b.WriteString("\n" + rendered)
		} else {
			b.WriteString("\n**Decomposition required.** This task has failed too many times to continue as-is.\n")
			b.WriteString("Break this leaf into smaller sub-tasks using the wolfcastle CLI:\n\n")
			fmt.Fprintf(&b, "1. Create child nodes: `wolfcastle project create --node %s --type leaf \"<name>\"`\n", nodeAddr)
			fmt.Fprintf(&b, "2. Add tasks to each child: `wolfcastle task add --node %s/<child-slug> \"<description>\"`\n", nodeAddr)
			fmt.Fprintf(&b, "3. Emit %s when decomposition is complete.\n\n", invoke.MarkerStringYield)
			b.WriteString("The parent node will automatically convert from leaf to orchestrator when the first child is created.\n")
		}
	}

	return b.String()
}
