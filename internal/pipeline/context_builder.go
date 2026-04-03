package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/invoke"
	"github.com/dorkusprime/wolfcastle/internal/knowledge"
	"github.com/dorkusprime/wolfcastle/internal/state"
)

// ContextBuilder composes iteration context strings by delegating section
// rendering to domain types (NodeState, Task, AuditState) and layering in
// class guidance and prompt-template-backed sections. It holds two repository
// dependencies and cached templates; safe for concurrent use after construction.
type ContextBuilder struct {
	prompts       *PromptRepository
	classes       *ClassRepository
	wolfcastleDir string

	// Cached parsed templates, resolved once at construction time.
	// nil when the corresponding prompt file is missing (fallback text is used).
	tmplSummary       *template.Template
	tmplFailHeader    *template.Template
	tmplDecomposition *template.Template
}

// NewContextBuilder creates a ContextBuilder. Both repositories are required;
// panics if either is nil. Templates are parsed eagerly and cached for the
// lifetime of the builder; missing prompt files are tolerated (fallback text
// is used at render time). wolfcastleDir is the root .wolfcastle directory,
// used to locate knowledge files; pass "" to disable knowledge injection.
func NewContextBuilder(prompts *PromptRepository, classes *ClassRepository, wolfcastleDir string) *ContextBuilder {
	if prompts == nil {
		panic("pipeline: NewContextBuilder requires a non-nil PromptRepository")
	}
	if classes == nil {
		panic("pipeline: NewContextBuilder requires a non-nil ClassRepository")
	}
	cb := &ContextBuilder{prompts: prompts, classes: classes, wolfcastleDir: wolfcastleDir}
	cb.tmplSummary = cb.cacheTemplate("summary-required")
	cb.tmplFailHeader = cb.cacheTemplate("context-headers")
	cb.tmplDecomposition = cb.cacheTemplate("decomposition")
	return cb
}

// cacheTemplate attempts to load and parse a prompt template by name.
// Returns nil when the prompt file is missing or fails to parse.
func (cb *ContextBuilder) cacheTemplate(name string) *template.Template {
	raw, err := cb.prompts.Resolve(name, nil)
	if err != nil {
		return nil
	}
	tmpl, err := template.New(name).Parse(raw)
	if err != nil {
		return nil
	}
	return tmpl
}

// Build assembles the complete iteration context for a single task within a
// node. The returned Markdown string is ready for inclusion in the
// execute-stage prompt. nodeDir is optional; when non-empty, per-task .md
// files are read from it. namespace identifies the engineer namespace for
// knowledge file lookup; pass "" to skip knowledge injection. cfg may be nil;
// failure context is skipped when nil.
// Returns an error when taskID does not match any task in the node.
func (cb *ContextBuilder) Build(nodeAddr string, nodeDir string, ns *state.NodeState, taskID string, namespace string, cfg *config.Config) (string, error) {
	var b strings.Builder

	// 1. Node address header
	fmt.Fprintf(&b, "**Node:** %s\n", nodeAddr)

	// 2. Node context (type, state, linked specs)
	b.WriteString(ns.RenderContext(taskID))

	// 3. Task context
	task, err := findTask(ns, taskID)
	if err != nil {
		return "", fmt.Errorf("context build for node %s: %w", nodeAddr, err)
	}
	{
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

		// 4a. Universal guidance (always injected)
		if universal, err := cb.prompts.ResolveRaw("prompts/classes", "universal.md"); err == nil {
			b.WriteString("\n## Universal Guidance\n\n")
			b.WriteString(universal)
			if !strings.HasSuffix(universal, "\n") {
				b.WriteString("\n")
			}
		}

		// 4b. Class guidance (always injected; falls back to coding/default.md)
		var classGuidance string
		if task.Class != "" {
			if resolved, err := cb.classes.Resolve(task.Class); err == nil {
				classGuidance = resolved
			}
		}
		if classGuidance == "" {
			classGuidance, _ = cb.prompts.ResolveRaw("prompts/classes", "coding/default.md")
		}
		if classGuidance != "" {
			b.WriteString("\n## Class Guidance\n\n")
			b.WriteString(classGuidance)
			if !strings.HasSuffix(classGuidance, "\n") {
				b.WriteString("\n")
			}
		}
	}

	// 4c. Codebase knowledge (read fresh every iteration, never cached)
	if cb.wolfcastleDir != "" && namespace != "" {
		if content, err := knowledge.Read(cb.wolfcastleDir, namespace); err == nil && strings.TrimSpace(content) != "" {
			b.WriteString("\n## Codebase Knowledge\n\n")
			b.WriteString(content)
			if !strings.HasSuffix(content, "\n") {
				b.WriteString("\n")
			}
		}
	}

	// 4d. Parallel execution scope acquisition instructions
	if cfg != nil && cfg.Daemon.Parallel.Enabled {
		b.WriteString("\n## Parallel Execution: Scope Acquisition Required\n\n")
		b.WriteString("Before writing any code, declare the files you intend to modify:\n\n")
		fmt.Fprintf(&b, "    wolfcastle task scope add --node %s file1.go file2.go pkg/foo/\n\n", nodeAddr)
		b.WriteString("List every file you plan to create or modify. Include directories (with trailing\n")
		b.WriteString("slash) when you expect to modify multiple files under a path. Be thorough: if you\n")
		b.WriteString("write to a file you did not scope, the daemon rejects your commit and the task fails.\n\n")
		b.WriteString("If the command fails with a scope conflict, another task is working on those files.\n")
		b.WriteString("Emit the following on its own line, replacing the address with the one from the error:\n\n")
		b.WriteString("    WOLFCASTLE_YIELD scope_conflict <conflicting-task-address>\n\n")
		b.WriteString("Do not attempt to work around the conflict.\n\n")
		b.WriteString("You may call wolfcastle task scope add again later if you discover additional files.\n")
		b.WriteString("If that second call also fails with a conflict, emit WOLFCASTLE_YIELD scope_conflict\n")
		b.WriteString("with the conflicting address.\n")
	}

	// 4e. Audit test verification policy
	if cfg != nil && cfg.Audit.RequireTests != "" {
		fmt.Fprintf(&b, "\n## Test Verification Policy\n\n")
		fmt.Fprintf(&b, "**require_tests:** `%s`\n", cfg.Audit.RequireTests)
	}

	// 5. Prior task AARs
	b.WriteString(state.RenderAARs(ns.AARs))

	// 6. Audit context (breadcrumbs, scope)
	b.WriteString(ns.Audit.RenderContext())

	// 7. Summary requirement
	if task != nil && cb.shouldIncludeSummary(ns, taskID) {
		summary := cb.renderSummaryRequired()
		b.WriteString("\n## Summary Required\n\n")
		b.WriteString(summary)
	}

	// 8. Failure context
	if task != nil && task.FailureCount > 0 && cfg != nil {
		failCtx := cb.renderFailureContext(nodeAddr, task, ns.DecompositionDepth, cfg)
		b.WriteString("\n" + failCtx)
	}

	return b.String(), nil
}

// findTask locates a task by ID within the node's task list. Returns an error
// when no matching task exists.
func findTask(ns *state.NodeState, taskID string) (*state.Task, error) {
	for i := range ns.Tasks {
		if ns.Tasks[i].ID == taskID {
			return &ns.Tasks[i], nil
		}
	}
	return nil, fmt.Errorf("task %q not found in node (have %d tasks)", taskID, len(ns.Tasks))
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

// renderSummaryRequired uses the cached summary template or falls back to
// hardcoded text when no template is available.
func (cb *ContextBuilder) renderSummaryRequired() string {
	if cb.tmplSummary != nil {
		var buf strings.Builder
		// summary-required template takes no context; execute with nil.
		if err := cb.tmplSummary.Execute(&buf, nil); err == nil {
			return buf.String()
		}
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
	if cb.tmplFailHeader != nil {
		var buf strings.Builder
		if err := cb.tmplFailHeader.Execute(&buf, headerCtx); err == nil {
			b.WriteString(buf.String())
		} else {
			cb.writeFailHeaderFallback(&b, headerCtx)
		}
	} else {
		cb.writeFailHeaderFallback(&b, headerCtx)
	}

	// Decomposition guidance
	if task.NeedsDecomposition {
		decompCtx := DecompositionContext{NodeAddr: nodeAddr}
		if cb.tmplDecomposition != nil {
			var buf strings.Builder
			if err := cb.tmplDecomposition.Execute(&buf, decompCtx); err == nil {
				b.WriteString("\n" + buf.String())
			} else {
				cb.writeDecompFallback(&b, nodeAddr)
			}
		} else {
			cb.writeDecompFallback(&b, nodeAddr)
		}
	}

	return b.String()
}

func (cb *ContextBuilder) writeFailHeaderFallback(b *strings.Builder, ctx FailureHeaderContext) {
	b.WriteString("## Failure History\n\n")
	fmt.Fprintf(b, "This task has failed %d times.\n", ctx.FailureCount)
	fmt.Fprintf(b, "- Decomposition threshold: %d\n", ctx.DecompThreshold)
	fmt.Fprintf(b, "- Max decomposition depth: %d (current: %d)\n", ctx.MaxDecompDepth, ctx.CurrentDepth)
	fmt.Fprintf(b, "- Hard failure cap: %d\n", ctx.HardCap)
}

func (cb *ContextBuilder) writeDecompFallback(b *strings.Builder, nodeAddr string) {
	b.WriteString("\n**Decomposition required.** This task has failed too many times to continue as-is.\n")
	b.WriteString("Break this leaf into smaller sub-tasks using the wolfcastle CLI:\n\n")
	fmt.Fprintf(b, "1. Create child nodes: `wolfcastle project create --node %s --type leaf \"<name>\"`\n", nodeAddr)
	fmt.Fprintf(b, "2. Add tasks to each child: `wolfcastle task add --node %s/<child-slug> \"<description>\"`\n", nodeAddr)
	fmt.Fprintf(b, "3. Emit %s when decomposition is complete.\n\n", invoke.MarkerStringYield)
	b.WriteString("The parent node will automatically convert from leaf to orchestrator when the first child is created.\n")
}
