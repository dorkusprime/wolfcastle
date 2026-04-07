package detail

import (
	"strings"
	"testing"

	"github.com/dorkusprime/wolfcastle/internal/state"
)

func makeFullTask() *state.Task {
	return &state.Task{
		ID:                 "task-0001",
		Title:              "Implement auth handler",
		Description:        "Build the authentication handler for JWT tokens",
		State:              state.StatusInProgress,
		IsAudit:            true,
		BlockedReason:      "waiting on upstream",
		FailureCount:       3,
		NeedsDecomposition: true,
		Deliverables:       []string{"auth_handler.go", "auth_handler_test.go"},
		LastFailureType:    "test_failure",
		Body:               "Detailed implementation notes go here.",
		TaskType:           "implementation",
		Class:              "critical",
		Constraints:        []string{"Must use stdlib only", "No external deps"},
		AcceptanceCriteria: []string{"All tests pass", "Coverage > 95%"},
		References:         []string{"docs/auth-spec.md", "RFC 7519"},
	}
}

func makeMinimalTask() *state.Task {
	return &state.Task{
		ID:          "task-0002",
		Description: "A minimal task",
		State:       state.StatusNotStarted,
	}
}

func TestNewTaskDetailModel_Defaults(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	if m.addr != "" {
		t.Errorf("addr should be empty, got %q", m.addr)
	}
	if m.taskID != "" {
		t.Errorf("taskID should be empty, got %q", m.taskID)
	}
	if m.task != nil {
		t.Error("task should be nil")
	}
}

func TestLoad_FullTask(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	task := makeFullTask()
	m.Load("root/leaf", "task-0001", task)

	view := m.View()

	// Title and status
	if !strings.Contains(view, "task-0001") {
		t.Errorf("view should contain task ID, got %q", view)
	}
	if !strings.Contains(view, "Implement auth handler") {
		t.Errorf("view should contain title, got %q", view)
	}

	// Description
	if !strings.Contains(view, "JWT tokens") {
		t.Errorf("view should contain description, got %q", view)
	}

	// Body
	if !strings.Contains(view, "Body") {
		t.Errorf("view should contain Body section, got %q", view)
	}
	if !strings.Contains(view, "implementation notes") {
		t.Errorf("view should contain body text, got %q", view)
	}

	// Class and Type
	if !strings.Contains(view, "critical") {
		t.Errorf("view should contain class, got %q", view)
	}
	if !strings.Contains(view, "implementation") {
		t.Errorf("view should contain task type, got %q", view)
	}

	// Deliverables
	if !strings.Contains(view, "Deliverables") {
		t.Errorf("view should contain Deliverables section, got %q", view)
	}
	if !strings.Contains(view, "auth_handler.go") {
		t.Errorf("view should list deliverables, got %q", view)
	}

	// Acceptance Criteria
	if !strings.Contains(view, "Acceptance Criteria") {
		t.Errorf("view should contain Acceptance Criteria section, got %q", view)
	}
	if !strings.Contains(view, "Coverage > 95%") {
		t.Errorf("view should list acceptance criteria, got %q", view)
	}

	// Constraints
	if !strings.Contains(view, "Constraints") {
		t.Errorf("view should contain Constraints section, got %q", view)
	}
	if !strings.Contains(view, "No external deps") {
		t.Errorf("view should list constraints, got %q", view)
	}

	// References
	if !strings.Contains(view, "References") {
		t.Errorf("view should contain References section, got %q", view)
	}
	if !strings.Contains(view, "RFC 7519") {
		t.Errorf("view should list references, got %q", view)
	}

	// Block reason
	if !strings.Contains(view, "waiting on upstream") {
		t.Errorf("view should show block reason, got %q", view)
	}

	// Failures
	if !strings.Contains(view, "Failures: 3") {
		t.Errorf("view should show failure count, got %q", view)
	}

	// Last failure type
	if !strings.Contains(view, "test_failure") {
		t.Errorf("view should show last failure type, got %q", view)
	}

	// Needs decomposition
	if !strings.Contains(view, "Needs Decomposition: yes") {
		t.Errorf("view should show needs decomposition yes, got %q", view)
	}

	// Is audit
	if !strings.Contains(view, "Is Audit: yes") {
		t.Errorf("view should show is audit yes, got %q", view)
	}
}

func TestLoad_MinimalTask(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	task := makeMinimalTask()
	m.Load("root/leaf", "task-0002", task)

	view := m.View()

	// Empty sections should be omitted
	if strings.Contains(view, "Body") {
		t.Errorf("minimal task should not show Body section, got %q", view)
	}
	if strings.Contains(view, "Deliverables") {
		t.Errorf("minimal task should not show Deliverables, got %q", view)
	}
	if strings.Contains(view, "Acceptance Criteria") {
		t.Errorf("minimal task should not show Acceptance Criteria, got %q", view)
	}
	if strings.Contains(view, "Constraints") {
		t.Errorf("minimal task should not show Constraints, got %q", view)
	}
	if strings.Contains(view, "References") {
		t.Errorf("minimal task should not show References, got %q", view)
	}

	// Empty/zero status fields should be omitted entirely.
	if strings.Contains(view, "Block Reason") {
		t.Errorf("empty block reason should be omitted, got %q", view)
	}
	if strings.Contains(view, "Failures") {
		t.Errorf("zero failures should be omitted, got %q", view)
	}
	if strings.Contains(view, "Last Failure") {
		t.Errorf("empty last failure should be omitted, got %q", view)
	}
	if strings.Contains(view, "Needs Decomposition") {
		t.Errorf("false decomposition should be omitted, got %q", view)
	}
	if strings.Contains(view, "Is Audit") {
		t.Errorf("false audit should be omitted, got %q", view)
	}
}

func TestTaskAddr(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.Load("root/leaf", "task-0001", makeFullTask())

	got := m.TaskAddr()
	if got != "root/leaf/task-0001" {
		t.Errorf("expected 'root/leaf/task-0001', got %q", got)
	}
}

func TestBlockReason_None(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	task := &state.Task{
		ID:          "t1",
		Description: "no block",
		State:       state.StatusNotStarted,
	}
	m.Load("root", "t1", task)

	view := m.View()
	if strings.Contains(view, "Block Reason") {
		t.Errorf("empty block reason should be omitted, got %q", view)
	}
}

func TestBlockReason_Present(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	task := &state.Task{
		ID:            "t1",
		Description:   "blocked task",
		State:         state.StatusBlocked,
		BlockedReason: "dependency cycle",
	}
	m.Load("root", "t1", task)

	view := m.View()
	if !strings.Contains(view, "Block Reason: dependency cycle") {
		t.Errorf("block reason should show 'dependency cycle', got %q", view)
	}
}

func TestFailureCount(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	task := &state.Task{
		ID:           "t1",
		Description:  "failing task",
		State:        state.StatusInProgress,
		FailureCount: 7,
	}
	m.Load("root", "t1", task)

	view := m.View()
	if !strings.Contains(view, "Failures: 7") {
		t.Errorf("should show failure count 7, got %q", view)
	}
}

func TestLastFailureType(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	task := &state.Task{
		ID:              "t1",
		Description:     "failing task",
		State:           state.StatusInProgress,
		LastFailureType: "compilation_error",
	}
	m.Load("root", "t1", task)

	view := m.View()
	if !strings.Contains(view, "Last Failure: compilation_error") {
		t.Errorf("should show last failure type, got %q", view)
	}
}

func TestNeedsDecomposition_Yes(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	task := &state.Task{
		ID:                 "t1",
		Description:        "decompose me",
		State:              state.StatusNotStarted,
		NeedsDecomposition: true,
	}
	m.Load("root", "t1", task)

	view := m.View()
	if !strings.Contains(view, "Needs Decomposition: yes") {
		t.Errorf("should show 'yes', got %q", view)
	}
}

func TestNeedsDecomposition_No(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	task := &state.Task{
		ID:          "t1",
		Description: "simple task",
		State:       state.StatusNotStarted,
	}
	m.Load("root", "t1", task)

	view := m.View()
	if strings.Contains(view, "Needs Decomposition") {
		t.Errorf("false decomposition should be omitted, got %q", view)
	}
}

func TestIsAudit_Yes(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	task := &state.Task{
		ID:          "t1",
		Description: "audit task",
		State:       state.StatusInProgress,
		IsAudit:     true,
	}
	m.Load("root", "t1", task)

	view := m.View()
	if !strings.Contains(view, "Is Audit: yes") {
		t.Errorf("should show 'yes', got %q", view)
	}
}

func TestIsAudit_No(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	task := &state.Task{
		ID:          "t1",
		Description: "normal task",
		State:       state.StatusInProgress,
	}
	m.Load("root", "t1", task)

	view := m.View()
	if strings.Contains(view, "Is Audit") {
		t.Errorf("false audit should be omitted, got %q", view)
	}
}

func TestDeliverables_Bulleted(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	task := &state.Task{
		ID:           "t1",
		Description:  "task with deliverables",
		State:        state.StatusInProgress,
		Deliverables: []string{"file_a.go", "file_b.go"},
	}
	m.Load("root", "t1", task)

	view := m.View()
	if !strings.Contains(view, "•") {
		t.Errorf("deliverables should use bullet points, got %q", view)
	}
	if !strings.Contains(view, "file_a.go") {
		t.Errorf("should list first deliverable, got %q", view)
	}
	if !strings.Contains(view, "file_b.go") {
		t.Errorf("should list second deliverable, got %q", view)
	}
}

func TestAcceptanceCriteria_Bulleted(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	task := &state.Task{
		ID:                 "t1",
		Description:        "task with criteria",
		State:              state.StatusInProgress,
		AcceptanceCriteria: []string{"criterion one", "criterion two"},
	}
	m.Load("root", "t1", task)

	view := m.View()
	if !strings.Contains(view, "criterion one") {
		t.Errorf("should list criteria, got %q", view)
	}
}

func TestConstraints_Bulleted(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	task := &state.Task{
		ID:          "t1",
		Description: "constrained task",
		State:       state.StatusInProgress,
		Constraints: []string{"max 100 LOC", "no panics"},
	}
	m.Load("root", "t1", task)

	view := m.View()
	if !strings.Contains(view, "max 100 LOC") {
		t.Errorf("should list constraints, got %q", view)
	}
}

func TestReferences_Listed(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	task := &state.Task{
		ID:          "t1",
		Description: "referenced task",
		State:       state.StatusInProgress,
		References:  []string{"docs/spec.md", "https://example.com"},
	}
	m.Load("root", "t1", task)

	view := m.View()
	if !strings.Contains(view, "docs/spec.md") {
		t.Errorf("should list first reference, got %q", view)
	}
	if !strings.Contains(view, "https://example.com") {
		t.Errorf("should list second reference, got %q", view)
	}
}

func TestBody_Present(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	task := &state.Task{
		ID:          "t1",
		Description: "task with body",
		State:       state.StatusInProgress,
		Body:        "Here are some detailed notes about the task.",
	}
	m.Load("root", "t1", task)

	view := m.View()
	if !strings.Contains(view, "Body") {
		t.Errorf("should show Body section, got %q", view)
	}
	if !strings.Contains(view, "detailed notes") {
		t.Errorf("should show body text, got %q", view)
	}
}

func TestClassAndType(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	task := &state.Task{
		ID:          "t1",
		Description: "typed task",
		State:       state.StatusInProgress,
		Class:       "default",
		TaskType:    "documentation",
	}
	m.Load("root", "t1", task)

	view := m.View()
	if !strings.Contains(view, "Class: default") {
		t.Errorf("should show class, got %q", view)
	}
	if !strings.Contains(view, "Type: documentation") {
		t.Errorf("should show task type, got %q", view)
	}
}

func TestClassAndType_OnlyClass(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	task := &state.Task{
		ID:          "t1",
		Description: "only class",
		State:       state.StatusNotStarted,
		Class:       "priority",
	}
	m.Load("root", "t1", task)

	view := m.View()
	if !strings.Contains(view, "Class: priority") {
		t.Errorf("should show class, got %q", view)
	}
}

func TestClassAndType_Neither(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	task := &state.Task{
		ID:          "t1",
		Description: "no class or type",
		State:       state.StatusNotStarted,
	}
	m.Load("root", "t1", task)

	view := m.View()
	if strings.Contains(view, "Class:") {
		t.Errorf("should not show Class when empty, got %q", view)
	}
}

func TestSetSize_Propagates_Task(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(100, 30)

	if m.width != 100 {
		t.Errorf("expected width 100, got %d", m.width)
	}
	if m.height != 30 {
		t.Errorf("expected height 30, got %d", m.height)
	}
}

func TestSetSize_SmallWidth_Task(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(10, 30)
	task := makeFullTask()
	m.Load("root/leaf", "task-0001", task)

	// Should not panic; wrapWidth guard handles small sizes
	view := m.View()
	if view == "" {
		t.Error("view should not be empty for loaded task with small width")
	}
}

func TestUpdate_Task_PassesToViewport(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	m.Load("root", "t1", makeFullTask())

	// Key press should not panic
	m, _ = m.Update(keyPress('j'))
}

func TestUpdate_Task_NonKeyMsg_Ignored(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	type customMsg struct{}
	m, _ = m.Update(customMsg{})
}

func TestRebuildContent_NilTask(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	// rebuildContent with nil task should not panic
	m.rebuildContent()
}

func TestBoolYesNo(t *testing.T) {
	t.Parallel()
	if boolYesNo(true) != "yes" {
		t.Errorf("true should be 'yes', got %q", boolYesNo(true))
	}
	if boolYesNo(false) != "no" {
		t.Errorf("false should be 'no', got %q", boolYesNo(false))
	}
}

func TestTitle_Present(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	task := &state.Task{
		ID:          "t1",
		Title:       "My Task Title",
		Description: "description here",
		State:       state.StatusNotStarted,
	}
	m.Load("root", "t1", task)

	view := m.View()
	if !strings.Contains(view, "My Task Title") {
		t.Errorf("should show title, got %q", view)
	}
}

func TestTitle_Empty(t *testing.T) {
	t.Parallel()
	m := NewTaskDetailModel()
	m.SetSize(80, 40)
	task := &state.Task{
		ID:          "t1",
		Description: "description only",
		State:       state.StatusNotStarted,
	}
	m.Load("root", "t1", task)

	view := m.View()
	if !strings.Contains(view, "description only") {
		t.Errorf("should show description when no title, got %q", view)
	}
}
