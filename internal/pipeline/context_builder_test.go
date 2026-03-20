package pipeline_test

import (
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

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes)
	got, err := cb.Build("my-project/auth", "", ns, "task-0001", nil)
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

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes)
	got, err := cb.Build("proj", "", ns, "task-0001", nil)
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

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes)
	got, err := cb.Build("proj", "", ns, "task-0001", nil)
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

func TestContextBuilder_OmitsClassWhenEmpty(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "No class", State: state.StatusInProgress},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes)
	got, err := cb.Build("proj", "", ns, "task-0001", nil)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(got, "## Class Guidance") {
		t.Error("class guidance should be absent when task has no class")
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

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes)
	got, err := cb.Build("proj", "", ns, "task-0001", nil)
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

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes)
	got, err := cb.Build("proj", "", ns, "task-0002", nil)
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

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes)
	got, err := cb.Build("proj", "", ns, "task-0001", nil)
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

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes)
	got, err := cb.Build("proj", "", ns, "task-0001", nil)
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

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes)
	got, err := cb.Build("proj", "", ns, "task-0001", cfg)
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

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes)
	got, err := cb.Build("proj", "", ns, "task-0001", cfg)
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

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes)
	got, err := cb.Build("proj/deep", "", ns, "task-0001", cfg)
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

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes)
	got, err := cb.Build("proj/node", "", ns, "task-0001", cfg)
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

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes)
	got, err := cb.Build("proj", "", ns, "task-0001", nil)
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

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes)
	_, err := cb.Build("proj/empty", "", ns, "nonexistent", nil)

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

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes)
	got, err := cb.Build("proj", "", ns, "task-0001", nil)
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
	pipeline.NewContextBuilder(nil, env.Classes)
}

func TestContextBuilder_PanicsOnNilClasses(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for nil ClassRepository")
		}
	}()
	pipeline.NewContextBuilder(env.Prompts, nil)
}

func TestContextBuilder_ClassResolveError_SkipsGuidance(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t)
	// Class configured but no prompt file exists
	env.WithClasses(map[string]config.ClassDef{"missing-class": {}})

	ns := &state.NodeState{
		Type:  state.NodeLeaf,
		State: state.StatusInProgress,
		Tasks: []state.Task{
			{ID: "task-0001", Description: "Work", State: state.StatusInProgress, Class: "missing-class"},
		},
	}

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes)
	got, err := cb.Build("proj", "", ns, "task-0001", nil)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(got, "## Class Guidance") {
		t.Error("class guidance should be silently skipped when Resolve fails")
	}
}

func TestContextBuilder_SectionOrdering(t *testing.T) {
	t.Parallel()
	env := testutil.NewEnvironment(t).
		WithClasses(map[string]config.ClassDef{"lang-go": {}}).
		WithPrompt("classes/lang-go.md", "Go guidance here.")

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

	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes)
	got, err := cb.Build("proj", "", ns, "task-0001", cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Verify ordering: node addr < node context < task < class < audit < failure
	nodeIdx := strings.Index(got, "**Node:** proj")
	typeIdx := strings.Index(got, "**Node Type:**")
	taskIdx := strings.Index(got, "**Task:** proj/task-0001")
	classIdx := strings.Index(got, "## Class Guidance")
	auditIdx := strings.Index(got, "## Recent Breadcrumbs")
	failIdx := strings.Index(got, "Failure History")

	if nodeIdx >= typeIdx {
		t.Error("node address should precede node type")
	}
	if typeIdx >= taskIdx {
		t.Error("node context should precede task context")
	}
	if taskIdx >= classIdx {
		t.Error("task context should precede class guidance")
	}
	if classIdx >= auditIdx {
		t.Error("class guidance should precede audit context")
	}
	if auditIdx >= failIdx {
		t.Error("audit context should precede failure context")
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
	cb := pipeline.NewContextBuilder(env.Prompts, env.Classes)

	got1, err := cb.Build("proj", "", ns, "task-0001", cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Mutate the failure count and build again with the same builder.
	ns.Tasks[0].FailureCount = 7
	got2, err := cb.Build("proj", "", ns, "task-0001", cfg)
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
