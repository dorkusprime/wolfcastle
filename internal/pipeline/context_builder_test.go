package pipeline_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/config"
	"github.com/dorkusprime/wolfcastle/internal/pipeline"
	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
)

func TestContextBuilder_IncludesNodeContext(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Do something", State: state.StatusInProgress},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("my-project/auth", "", ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "**Node:** my-project/auth") {
		t.Error("missing node address header")
	}
	if !strings.Contains(got, "**Node Type:** leaf") {
		t.Error("missing node type from RenderContext")
	}
	if !strings.Contains(got, "**Node State:** in_progress") {
		t.Error("missing node state from RenderContext")
	}
}

func TestContextBuilder_IncludesTaskContext(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{
				ID:                 "task-0001",
				Description:        "Implement auth",
				State:              state.StatusInProgress,
				Body:               "Detailed body text here",
				Deliverables:       []string{"auth.go", "auth_test.go"},
				AcceptanceCriteria: []string{"tests pass"},
			},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "**Task:** proj/task-0001") {
		t.Error("missing node-qualified task address")
	}
	if !strings.Contains(got, "**Description:** Implement auth") {
		t.Error("missing task description")
	}
	if !strings.Contains(got, "Detailed body text here") {
		t.Error("missing task body")
	}
	if !strings.Contains(got, "`auth.go`") {
		t.Error("missing deliverable")
	}
	if !strings.Contains(got, "tests pass") {
		t.Error("missing acceptance criteria")
	}
}

func TestContextBuilder_IncludesClassGuidance(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithClasses(map[string]config.ClassDef{"lang-go": {}}).
		WithPrompt("classes/lang-go.md", "Follow Go idioms and use gofmt.")

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Write code", State: state.StatusInProgress, Class: "lang-go"},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "## Class Guidance") {
		t.Error("missing class guidance header")
	}
	if !strings.Contains(got, "Follow Go idioms and use gofmt.") {
		t.Error("missing class guidance content")
	}
}

func TestContextBuilder_FallsBackToCodingDefault_WhenClassEmpty(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("classes/coding/default.md", "Default coding guidance.")

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "No class", State: state.StatusInProgress},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "## Class Guidance") {
		t.Error("class guidance should be present with coding/default.md fallback")
	}
	if !strings.Contains(got, "Default coding guidance.") {
		t.Error("should contain coding/default.md content")
	}
}

func TestContextBuilder_OmitsClassGuidance_WhenNoCodingDefault(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "No class", State: state.StatusInProgress},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(got, "## Class Guidance") {
		t.Error("class guidance should be absent when no class and no coding/default.md")
	}
}

func TestContextBuilder_IncludesUniversalGuidance(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("classes/universal.md", "Always apply these principles.")

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Work", State: state.StatusInProgress},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "## Universal Guidance") {
		t.Error("missing universal guidance header")
	}
	if !strings.Contains(got, "Always apply these principles.") {
		t.Error("missing universal guidance content")
	}
}

func TestContextBuilder_UniversalGuidance_OmittedWhenMissing(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Work", State: state.StatusInProgress},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(got, "## Universal Guidance") {
		t.Error("universal guidance should be absent when universal.md does not exist")
	}
}

func TestContextBuilder_IncludesAuditContext(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Work", State: state.StatusInProgress},
		},
		Audit: state.AuditState{
			Breadcrumbs: []state.Breadcrumb{
				{Timestamp: now, Task: "task-0001", Text: "Did something important"},
			},
			Scope: &state.AuditScope{
				Description: "Verify auth middleware",
			},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "## Recent Breadcrumbs") {
		t.Error("missing breadcrumbs section")
	}
	if !strings.Contains(got, "Did something important") {
		t.Error("missing breadcrumb text")
	}
	if !strings.Contains(got, "## Audit Scope") {
		t.Error("missing audit scope section")
	}
	if !strings.Contains(got, "Verify auth middleware") {
		t.Error("missing audit scope description")
	}
}

func TestContextBuilder_SummaryRequired_LastIncompleteTask(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Done", State: state.StatusComplete},
			{ID: "task-0002", Description: "Last one", State: state.StatusInProgress},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0002", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "## Summary Required") {
		t.Error("missing summary required section for last incomplete task")
	}
	if !strings.Contains(got, "last incomplete task") {
		t.Error("missing summary guidance text (fallback)")
	}
}

func TestContextBuilder_SummaryOmitted_OtherTasksRemain(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Current", State: state.StatusInProgress},
			{ID: "task-0002", Description: "Still pending", State: state.StatusNotStarted},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(got, "## Summary Required") {
		t.Error("summary required should be absent when other tasks are incomplete")
	}
}

func TestContextBuilder_SummaryRequired_UsesPromptTemplate(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("summary-required.md", "Custom summary instructions from template.")

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Only task", State: state.StatusInProgress},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "Custom summary instructions from template.") {
		t.Error("should use summary-required.md prompt template when available")
	}
}

func TestContextBuilder_FailureHeader(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	cfg := config.Defaults()
	ns := &state.NodeState{
		Type:               state.NodeLeaf,
		State:              state.StatusInProgress,
		DecompositionDepth: 1,
		Tasks: []state.Task{
			{
				ID:           "task-0001",
				Description:  "Failing task",
				State:        state.StatusInProgress,
				FailureCount: 3,
			},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Falls back to hardcoded text since no context-headers.md template is seeded
	if !strings.Contains(got, "Failure History") {
		t.Error("missing failure history section")
	}
	if !strings.Contains(got, "failed 3 times") {
		t.Error("missing failure count in header")
	}
}

func TestContextBuilder_FailureHeader_UsesPromptTemplate(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("context-headers.md", "CUSTOM HEADER: {{.FailureCount}} failures, threshold {{.DecompThreshold}}")

	cfg := config.Defaults()
	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Failing", State: state.StatusInProgress, FailureCount: 5},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", cfg)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "CUSTOM HEADER: 5 failures, threshold 10") {
		t.Errorf("expected custom template output, got:\n%s", got)
	}
}

func TestContextBuilder_DecompositionGuidance(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	cfg := config.Defaults()
	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{
				ID:                 "task-0001",
				Description:        "Decompose me",
				State:              state.StatusInProgress,
				FailureCount:       12,
				NeedsDecomposition: true,
			},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj/deep", "", ns, "task-0001", "", cfg)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "Decomposition required") {
		t.Error("missing decomposition guidance")
	}
	if !strings.Contains(got, "proj/deep") {
		t.Error("decomposition guidance should include node address")
	}
}

func TestContextBuilder_DecompositionGuidance_UsesPromptTemplate(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("decomposition.md", "DECOMPOSE NOW at {{.NodeAddr}}")

	cfg := config.Defaults()
	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{
				ID:                 "task-0001",
				Description:        "Break it down",
				State:              state.StatusInProgress,
				FailureCount:       15,
				NeedsDecomposition: true,
			},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj/node", "", ns, "task-0001", "", cfg)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "DECOMPOSE NOW at proj/node") {
		t.Errorf("expected custom decomposition template, got:\n%s", got)
	}
}

func TestContextBuilder_FailureSkippedWithNilConfig(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Failing", State: state.StatusInProgress, FailureCount: 5},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(got, "Failure History") {
		t.Error("failure context should be skipped when cfg is nil")
	}
}

func TestContextBuilder_Build_ErrorsOnMissingTask(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusNotStarted,
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	_, err := cb.Build("proj/empty", "", ns, "nonexistent", "", nil)

	if err == nil {
		t.Fatal("expected error when task ID does not exist in node")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("error should mention the missing task ID, got: %s", err)
	}
	if !strings.Contains(err.Error(), "proj/empty") {
		t.Errorf("error should mention the node address, got: %s", err)
	}
}

func TestContextBuilder_IncludesLinkedSpecs(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Specs: []string{"2026-03-18-api-spec.md", "2026-03-18-auth-spec.md"},
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Work", State: state.StatusInProgress},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "## Linked Specs") {
		t.Error("missing linked specs section")
	}
	if !strings.Contains(got, "2026-03-18-api-spec.md") {
		t.Error("missing first spec")
	}
	if !strings.Contains(got, "2026-03-18-auth-spec.md") {
		t.Error("missing second spec")
	}
}

func TestContextBuilder_PanicsOnNilPrompts(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil PromptRepository")
		}
	}()
	pipeline.NewContextBuilder(nil, env.Classes, "")
}

func TestContextBuilder_PanicsOnNilClasses(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil ClassRepository")
		}
	}()
	pipeline.NewContextBuilder(env.Prompts, nil, "")
}

func TestContextBuilder_UniversalGuidance_AppearsWithClassifiedTask(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithClasses(map[string]config.ClassDef{"lang-go": {}}).
		WithPrompt("classes/lang-go.md", "Go idioms apply.").
		WithPrompt("classes/universal.md", "Universal principles for all tasks.")

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Classified work", State: state.StatusInProgress, Class: "lang-go"},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "## Universal Guidance") {
		t.Error("universal guidance should appear even when task has a class")
	}
	if !strings.Contains(got, "Universal principles for all tasks.") {
		t.Error("universal guidance content missing for classified task")
	}
	if !strings.Contains(got, "## Class Guidance") {
		t.Error("class guidance should also appear alongside universal")
	}
	if !strings.Contains(got, "Go idioms apply.") {
		t.Error("class-specific content missing")
	}
}

func TestContextBuilder_ClassResolved_DoesNotUseCodingDefault(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithClasses(map[string]config.ClassDef{"lang-go": {}}).
		WithPrompt("classes/lang-go.md", "Go-specific guidance.").
		WithPrompt("classes/coding/default.md", "Default coding fallback.")

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Typed task", State: state.StatusInProgress, Class: "lang-go"},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "Go-specific guidance.") {
		t.Error("resolved class guidance should appear")
	}
	if strings.Contains(got, "Default coding fallback.") {
		t.Error("coding/default.md should NOT appear when class resolves successfully")
	}
}

func TestContextBuilder_ClassResolveError_FallsToCodingDefault(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("classes/coding/default.md", "Fallback coding guidance.")
	// Class configured but no prompt file exists
	env.WithClasses(map[string]config.ClassDef{"missing-class": {}})

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Work", State: state.StatusInProgress, Class: "missing-class"},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "## Class Guidance") {
		t.Error("class guidance should fall back to coding/default.md when class resolve fails")
	}
	if !strings.Contains(got, "Fallback coding guidance.") {
		t.Error("should contain coding/default.md content as fallback")
	}
}

func TestContextBuilder_ClassResolveError_NoCodingDefault_SkipsGuidance(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	env.WithClasses(map[string]config.ClassDef{"missing-class": {}})

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Work", State: state.StatusInProgress, Class: "missing-class"},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(got, "## Class Guidance") {
		t.Error("class guidance should be absent when resolve fails and no coding/default.md")
	}
}

func TestContextBuilder_SectionOrdering(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithClasses(map[string]config.ClassDef{"lang-go": {}}).
		WithPrompt("classes/lang-go.md", "Go guidance here.").
		WithPrompt("classes/universal.md", "Universal principles.")

	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	cfg := config.Defaults()
	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Specs: []string{"spec.md"},
		Tasks: []state.Task{
			{
				ID:           "task-0001",
				Description:  "The task",
				State:        state.StatusInProgress,
				Class:        "lang-go",
				FailureCount: 2,
			},
		},
		Audit: state.AuditState{
			Breadcrumbs: []state.Breadcrumb{
				{Timestamp: now, Task: "task-0001", Text: "breadcrumb"},
			},
			Scope: &state.AuditScope{Description: "audit scope"},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Verify ordering: node addr < node context < task < universal < class < audit < failure
	nodeIdx := strings.Index(got, "**Node:** proj")
	typeIdx := strings.Index(got, "**Node Type:**")
	taskIdx := strings.Index(got, "**Task:** proj/task-0001")
	universalIdx := strings.Index(got, "## Universal Guidance")
	classIdx := strings.Index(got, "## Class Guidance")
	auditIdx := strings.Index(got, "## Recent Breadcrumbs")
	failIdx := strings.Index(got, "Failure History")

	if nodeIdx >= typeIdx {
		t.Error("node address should precede node type")
	}
	if typeIdx >= taskIdx {
		t.Error("node context should precede task context")
	}
	if taskIdx >= universalIdx {
		t.Error("task context should precede universal guidance")
	}
	if universalIdx >= classIdx {
		t.Error("universal guidance should precede class guidance")
	}
	if classIdx >= auditIdx {
		t.Error("class guidance should precede audit context")
	}
	if auditIdx >= failIdx {
		t.Error("audit context should precede failure context")
	}
}

func TestContextBuilder_IncludesKnowledgeContent(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	// Write a knowledge file for the test namespace.
	knowledgeDir := env.Root + "/docs/knowledge"
	if err := os.MkdirAll(knowledgeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(knowledgeDir+"/test-ns.md", []byte("- The tests require Docker\n- Go 1.26 loop semantics\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Work", State: state.StatusInProgress},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, env.Root)
	got, err := cb.Build("proj", "", ns, "task-0001", "test-ns", nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "## Codebase Knowledge") {
		t.Error("missing codebase knowledge header")
	}
	if !strings.Contains(got, "The tests require Docker") {
		t.Error("missing knowledge content")
	}
}

func TestContextBuilder_OmitsKnowledgeWhenEmpty(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Work", State: state.StatusInProgress},
		},
	}

	// No knowledge file exists; section should be omitted.
	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, env.Root)
	got, err := cb.Build("proj", "", ns, "task-0001", "nonexistent-ns", nil)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(got, "## Codebase Knowledge") {
		t.Error("codebase knowledge should be absent when no knowledge file exists")
	}
}

func TestContextBuilder_OmitsKnowledgeWhenNamespaceEmpty(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Work", State: state.StatusInProgress},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, env.Root)
	got, err := cb.Build("proj", "", ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(got, "## Codebase Knowledge") {
		t.Error("codebase knowledge should be absent when namespace is empty")
	}
}

func TestContextBuilder_KnowledgeBetweenClassAndAARs(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithClasses(map[string]config.ClassDef{"lang-go": {}}).
		WithPrompt("classes/lang-go.md", "Go guidance.")

	knowledgeDir := env.Root + "/docs/knowledge"
	if err := os.MkdirAll(knowledgeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(knowledgeDir+"/test-ns.md", []byte("- Knowledge entry\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 3, 22, 12, 0, 0, 0, time.UTC)
	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0002", Description: "Work", State: state.StatusInProgress, Class: "lang-go"},
		},
		AARs: map[string]state.AAR{
			"task-0001": {
				TaskID:       "task-0001",
				Timestamp:    now,
				Objective:    "Prior work",
				WhatHappened: "Done",
			},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, env.Root)
	got, err := cb.Build("proj", "", ns, "task-0002", "test-ns", nil)
	if err != nil {
		t.Fatal(err)
	}

	classIdx := strings.Index(got, "## Class Guidance")
	knowledgeIdx := strings.Index(got, "## Codebase Knowledge")
	aarIdx := strings.Index(got, "## Prior Task Reviews (AARs)")

	if classIdx < 0 {
		t.Fatal("missing class guidance section")
	}
	if knowledgeIdx < 0 {
		t.Fatal("missing codebase knowledge section")
	}
	if aarIdx < 0 {
		t.Fatal("missing AARs section")
	}
	if classIdx >= knowledgeIdx {
		t.Error("class guidance should precede codebase knowledge")
	}
	if knowledgeIdx >= aarIdx {
		t.Error("codebase knowledge should precede AARs")
	}
}

func TestContextBuilder_TemplateCaching(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithPrompt("context-headers.md", "HEADER: {{.FailureCount}} failures")

	cfg := config.Defaults()
	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Fail", State: state.StatusInProgress, FailureCount: 3},
		},
	}

	// Build the context builder once. Templates are cached at construction.
	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")

	got1, err := cb.Build("proj", "", ns, "task-0001", "", cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Mutate the failure count and build again with the same builder.
	ns.Tasks[0].FailureCount = 7
	got2, err := cb.Build("proj", "", ns, "task-0001", "", cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Both calls should use the cached template but produce different output
	// because the data context changed.
	if !strings.Contains(got1, "HEADER: 3 failures") {
		t.Errorf("first call should render with count 3, got:\n%s", got1)
	}
	if !strings.Contains(got2, "HEADER: 7 failures") {
		t.Errorf("second call should render with count 7, got:\n%s", got2)
	}
}

func TestContextBuilder_ParallelEnabled_InjectsScopeSection(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	cfg := config.Defaults()
	cfg.Daemon.Parallel.Enabled = true

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Work", State: state.StatusInProgress},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("my-project/auth", "", ns, "task-0001", "", cfg)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "## Parallel Execution: Scope Acquisition Required") {
		t.Error("missing scope acquisition section when parallel is enabled")
	}
	if !strings.Contains(got, "wolfcastle task scope add --node my-project/auth") {
		t.Error("scope section should contain the node address")
	}
	if !strings.Contains(got, "WOLFCASTLE_YIELD scope_conflict") {
		t.Error("scope section should contain yield instruction for conflicts")
	}
}

func TestContextBuilder_ParallelDisabled_OmitsScopeSection(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	cfg := config.Defaults()
	// Parallel.Enabled defaults to false; be explicit.
	cfg.Daemon.Parallel.Enabled = false

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Work", State: state.StatusInProgress},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", cfg)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(got, "Scope Acquisition Required") {
		t.Error("scope acquisition section should be absent when parallel is disabled")
	}
}

func TestContextBuilder_ParallelNilConfig_OmitsScopeSection(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Work", State: state.StatusInProgress},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(got, "Scope Acquisition Required") {
		t.Error("scope acquisition section should be absent when config is nil")
	}
}

func TestContextBuilder_RequireTests_IncludesPolicy(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	cfg := config.Defaults()
	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Work", State: state.StatusInProgress},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", cfg)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "## Test Verification Policy") {
		t.Error("missing test verification policy section")
	}
	if !strings.Contains(got, "**require_tests:** `block`") {
		t.Error("missing require_tests value in context")
	}
}

func TestContextBuilder_RequireTests_Warn(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	cfg := config.Defaults()
	cfg.Audit.RequireTests = "warn"
	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Work", State: state.StatusInProgress},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", cfg)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(got, "**require_tests:** `warn`") {
		t.Error("expected require_tests=warn in context")
	}
}

func TestContextBuilder_RequireTests_OmittedWhenEmpty(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	cfg := config.Defaults()
	cfg.Audit.RequireTests = ""
	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Work", State: state.StatusInProgress},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", cfg)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(got, "## Test Verification Policy") {
		t.Error("test verification policy should be absent when require_tests is empty")
	}
}

func TestContextBuilder_RequireTests_OmittedWhenNilConfig(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Work", State: state.StatusInProgress},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0001", "", nil)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(got, "## Test Verification Policy") {
		t.Error("test verification policy should be absent when config is nil")
	}
}

func TestContextBuilder_ParallelScopeSection_Ordering(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithClasses(map[string]config.ClassDef{"lang-go": {}}).
		WithPrompt("classes/lang-go.md", "Go guidance.")

	cfg := config.Defaults()
	cfg.Daemon.Parallel.Enabled = true

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0002", Description: "Work", State: state.StatusInProgress, Class: "lang-go"},
		},
		AARs: map[string]state.AAR{
			"task-0001": {
				TaskID:       "task-0001",
				Timestamp:    time.Date(2026, 3, 24, 12, 0, 0, 0, time.UTC),
				Objective:    "Prior work",
				WhatHappened: "Done",
			},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes, "")
	got, err := cb.Build("proj", "", ns, "task-0002", "", cfg)
	if err != nil {
		t.Fatal(err)
	}

	classIdx := strings.Index(got, "## Class Guidance")
	scopeIdx := strings.Index(got, "## Parallel Execution: Scope Acquisition Required")
	aarIdx := strings.Index(got, "## Prior Task Reviews (AARs)")

	if classIdx < 0 {
		t.Fatal("missing class guidance section")
	}
	if scopeIdx < 0 {
		t.Fatal("missing scope acquisition section")
	}
	if aarIdx < 0 {
		t.Fatal("missing AARs section")
	}
	if classIdx >= scopeIdx {
		t.Error("class guidance should precede scope acquisition")
	}
	if scopeIdx >= aarIdx {
		t.Error("scope acquisition should precede AARs")
	}
}
