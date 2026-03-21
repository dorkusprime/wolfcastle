package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dorkusprime/wolfcastle/internal/state"
	"github.com/dorkusprime/wolfcastle/internal/testutil"
)

func newStatusTestEnv(t *testing.T) *testEnv {
	t.Helper()
	env := newTestEnv(t)
	env.env.WithProject("My Project", testutil.Leaf("my-project"))
	return env
}

func TestStatusCmd_Success(t *testing.T) {
	env := newStatusTestEnv(t)
	env.RootCmd.SetArgs([]string{"status"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("status failed: %v", err)
	}
}

func TestStatusCmd_WithScope(t *testing.T) {
	env := newStatusTestEnv(t)
	env.RootCmd.SetArgs([]string{"status", "--node", "my-project"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("status --node failed: %v", err)
	}
}

func TestShowAllStatus_NoNamespaces(t *testing.T) {
	env := newTestEnv(t)
	// showAllStatus reads from projects/ dir
	err := showAllStatus(env.App)
	if err != nil {
		t.Fatalf("showAllStatus failed: %v", err)
	}
}

func TestShowTreeStatus_EmptyTree(t *testing.T) {
	env := newTestEnv(t)
	idx := state.NewRootIndex()
	err := showTreeStatus(env.App, idx, "")
	if err != nil {
		t.Fatalf("showTreeStatus failed: %v", err)
	}
}

func TestShowTreeStatus_WithNodes(t *testing.T) {
	env := newStatusTestEnv(t)
	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	err := showTreeStatus(env.App, idx, "")
	if err != nil {
		t.Fatalf("showTreeStatus failed: %v", err)
	}
}

func TestShowTreeStatus_MultipleNodeStates(t *testing.T) {
	env := newStatusTestEnv(t)

	// Add another node in different states
	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	idx.Nodes["my-project/child-a"] = state.IndexEntry{
		Name:     "Child A",
		Type:     state.NodeLeaf,
		State:    state.StatusComplete,
		Address:  "my-project/child-a",
		Parent:   "my-project",
		Children: []string{},
	}
	idx.Nodes["my-project/child-b"] = state.IndexEntry{
		Name:     "Child B",
		Type:     state.NodeLeaf,
		State:    state.StatusBlocked,
		Address:  "my-project/child-b",
		Parent:   "my-project",
		Children: []string{},
	}
	idx.Nodes["my-project/child-c"] = state.IndexEntry{
		Name:     "Child C",
		Type:     state.NodeLeaf,
		State:    state.StatusNotStarted,
		Address:  "my-project/child-c",
		Parent:   "my-project",
		Children: []string{},
	}
	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), idxData, 0644)

	// Create node dirs and states for children
	for _, name := range []string{"child-a", "child-b", "child-c"} {
		nodeDir := filepath.Join(env.ProjectsDir, "my-project", name)
		_ = os.MkdirAll(nodeDir, 0755)
		ns := state.NewNodeState(name, "Child", state.NodeLeaf)
		nsData, _ := json.MarshalIndent(ns, "", "  ")
		_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), nsData, 0644)
	}

	// Test human output with scope
	if err := showTreeStatus(env.App, idx, "my-project"); err != nil {
		t.Fatalf("showTreeStatus with multiple states and scope failed: %v", err)
	}

	// Test JSON output
	env.App.JSON = true
	defer func() { env.App.JSON = false }()
	if err := showTreeStatus(env.App, idx, "my-project"); err != nil {
		t.Fatalf("showTreeStatus JSON with multiple states failed: %v", err)
	}
}

func TestShowAllStatus_WithMultipleNamespaces(t *testing.T) {
	env := newStatusTestEnv(t)
	// Create a second namespace
	ns2Dir := filepath.Join(env.WolfcastleDir, "system", "projects", "other-eng")
	_ = os.MkdirAll(ns2Dir, 0755)
	idx2 := state.NewRootIndex()
	idx2.Nodes["other-proj"] = state.IndexEntry{
		Name:    "Other Project",
		Type:    state.NodeLeaf,
		State:   state.StatusComplete,
		Address: "other-proj",
	}
	data, _ := json.MarshalIndent(idx2, "", "  ")
	_ = os.WriteFile(filepath.Join(ns2Dir, "state.json"), data, 0644)

	err := showAllStatus(env.App)
	if err != nil {
		t.Fatalf("showAllStatus with multiple namespaces failed: %v", err)
	}

	// JSON mode
	env.App.JSON = true
	defer func() { env.App.JSON = false }()
	err = showAllStatus(env.App)
	if err != nil {
		t.Fatalf("showAllStatus JSON with multiple namespaces failed: %v", err)
	}
}

func TestCountDescendants(t *testing.T) {
	idx := state.NewRootIndex()
	idx.Nodes["root"] = state.IndexEntry{
		Name: "Root", Type: state.NodeOrchestrator, Children: []string{"root/a", "root/b"},
	}
	idx.Nodes["root/a"] = state.IndexEntry{
		Name: "A", Type: state.NodeOrchestrator, Parent: "root", Children: []string{"root/a/x"},
	}
	idx.Nodes["root/a/x"] = state.IndexEntry{
		Name: "X", Type: state.NodeLeaf, Parent: "root/a",
	}
	idx.Nodes["root/b"] = state.IndexEntry{
		Name: "B", Type: state.NodeLeaf, Parent: "root",
	}

	if got := countDescendants(idx, "root"); got != 3 {
		t.Errorf("expected 3 descendants, got %d", got)
	}
	if got := countDescendants(idx, "root/a"); got != 1 {
		t.Errorf("expected 1 descendant, got %d", got)
	}
	if got := countDescendants(idx, "root/b"); got != 0 {
		t.Errorf("expected 0 descendants for leaf, got %d", got)
	}
	if got := countDescendants(idx, "nonexistent"); got != 0 {
		t.Errorf("expected 0 for missing node, got %d", got)
	}
}

// captureStdout runs fn while capturing stdout, returning the output as a string.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = origStdout
	buf := make([]byte, 64*1024)
	n, _ := r.Read(buf)
	_ = r.Close()
	return string(buf[:n])
}

func TestShowTreeStatus_SubtaskIndentation(t *testing.T) {
	env := newStatusTestEnv(t)

	// Add hierarchical tasks to the leaf node
	nodeDir := filepath.Join(env.ProjectsDir, "my-project")
	ns := state.NewNodeState("my-project", "My Project", state.NodeLeaf)
	ns.State = state.StatusInProgress
	ns.Tasks = []state.Task{
		{ID: "task-0001", Description: "Parent task", State: state.StatusInProgress},
		{ID: "task-0001.0001", Description: "First subtask", State: state.StatusComplete},
		{ID: "task-0001.0002", Description: "Second subtask", State: state.StatusNotStarted},
		{ID: "task-0002", Description: "Top-level task", State: state.StatusNotStarted},
		{ID: "audit", Description: "Audit", State: state.StatusNotStarted, IsAudit: true},
	}
	nsData, _ := json.MarshalIndent(ns, "", "  ")
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), nsData, 0644)

	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	// Should not panic and should render hierarchical tasks
	if err := showTreeStatus(env.App, idx, ""); err != nil {
		t.Fatalf("showTreeStatus with subtasks failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// --detail flag: task body, failure type, deliverables, breadcrumbs
// ---------------------------------------------------------------------------

// setupDetailEnv creates a leaf node with tasks that exercise all detail fields:
// body, failure type, deliverables, and breadcrumbs.
func setupDetailEnv(t *testing.T) (*testEnv, *state.RootIndex) {
	t.Helper()
	env := newStatusTestEnv(t)

	nodeDir := filepath.Join(env.ProjectsDir, "my-project")
	ns := state.NewNodeState("my-project", "My Project", state.NodeLeaf)
	ns.State = state.StatusInProgress
	ns.Tasks = []state.Task{
		{
			ID:              "task-0001",
			Title:           "Build the widget",
			Description:     "Construct the primary widget component",
			Body:            "This is a long task body that describes the full scope of the work to be done for building the widget.",
			State:           state.StatusInProgress,
			FailureCount:    3,
			LastFailureType: "timeout",
			Deliverables:    []string{"[x] widget.go created", "[ ] widget_test.go created", "[x] docs updated"},
		},
		{
			ID:          "task-0002",
			Title:       "Simple task",
			Description: "A task with no extras",
			State:       state.StatusNotStarted,
		},
	}
	ns.Audit.Breadcrumbs = []state.Breadcrumb{
		{Timestamp: time.Now().Add(-2 * time.Minute), Task: "task-0001", Text: "Started building widget scaffold"},
		{Timestamp: time.Now().Add(-1 * time.Minute), Task: "task-0001", Text: "Completed initial file structure for the widget component"},
	}

	nsData, _ := json.MarshalIndent(ns, "", "  ")
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), nsData, 0644)

	// Update index to reflect in_progress state
	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	entry := idx.Nodes["my-project"]
	entry.State = state.StatusInProgress
	idx.Nodes["my-project"] = entry
	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), idxData, 0644)

	return env, idx
}

func TestShowTreeStatus_DetailFlag_ShowsTaskBody(t *testing.T) {
	env, idx := setupDetailEnv(t)
	out := captureStdout(t, func() {
		_ = showTreeStatus(env.App, idx, "", false, true) // expand=false, detail=true
	})

	if !strings.Contains(out, "This is a long task body") {
		t.Error("detail mode should show task body")
	}
}

func TestShowTreeStatus_DetailFlag_ShowsFailureType(t *testing.T) {
	env, idx := setupDetailEnv(t)
	out := captureStdout(t, func() {
		_ = showTreeStatus(env.App, idx, "", false, true)
	})

	if !strings.Contains(out, "3 failures, last: timeout") {
		t.Errorf("detail mode should show failure type inline, got:\n%s", out)
	}
}

func TestShowTreeStatus_DetailFlag_ShowsDeliverables(t *testing.T) {
	env, idx := setupDetailEnv(t)
	out := captureStdout(t, func() {
		_ = showTreeStatus(env.App, idx, "", false, true)
	})

	if !strings.Contains(out, "2/3 deliverables met") {
		t.Errorf("detail mode should show deliverable summary, got:\n%s", out)
	}
}

func TestShowTreeStatus_DetailFlag_ShowsBreadcrumb(t *testing.T) {
	env, idx := setupDetailEnv(t)
	out := captureStdout(t, func() {
		_ = showTreeStatus(env.App, idx, "", false, true)
	})

	if !strings.Contains(out, "breadcrumb:") {
		t.Error("detail mode should show most recent breadcrumb")
	}
	if !strings.Contains(out, "Completed initial file structure") {
		t.Error("detail mode should show the latest breadcrumb text")
	}
}

func TestShowTreeStatus_NoDetail_HidesExtras(t *testing.T) {
	env, idx := setupDetailEnv(t)
	out := captureStdout(t, func() {
		_ = showTreeStatus(env.App, idx, "", false, false) // expand=false, detail=false
	})

	if strings.Contains(out, "This is a long task body") {
		t.Error("without --detail, task body should not appear")
	}
	if strings.Contains(out, "deliverables met") {
		t.Error("without --detail, deliverable summary should not appear")
	}
	if strings.Contains(out, "breadcrumb:") {
		t.Error("without --detail, breadcrumb should not appear")
	}
	if strings.Contains(out, "last: timeout") {
		t.Error("without --detail, failure type should not appear")
	}
	// Basic failure count should still show
	if !strings.Contains(out, "3 failures") {
		t.Error("failure count should appear even without --detail")
	}
}

func TestShowTreeStatus_DefaultView_Unchanged(t *testing.T) {
	env, idx := setupDetailEnv(t)
	// Call with no flags at all (variadic empty)
	out := captureStdout(t, func() {
		_ = showTreeStatus(env.App, idx, "")
	})

	if strings.Contains(out, "deliverables met") {
		t.Error("default view should not show deliverable summary")
	}
	if strings.Contains(out, "breadcrumb:") {
		t.Error("default view should not show breadcrumbs")
	}
}

func TestShowTreeStatus_JSON_IncludesNodeDetails(t *testing.T) {
	env, idx := setupDetailEnv(t)
	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	out := captureStdout(t, func() {
		_ = showTreeStatus(env.App, idx, "")
	})

	var resp map[string]any
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	data, ok := resp["data"].(map[string]any)
	if !ok {
		t.Fatal("expected data field in JSON response")
	}
	nodes, ok := data["nodes"].(map[string]any)
	if !ok {
		t.Fatal("expected nodes field in JSON data")
	}
	proj, ok := nodes["my-project"].(map[string]any)
	if !ok {
		t.Fatal("expected my-project node in JSON nodes")
	}
	tasks, ok := proj["tasks"].([]any)
	if !ok {
		t.Fatal("expected tasks array in node data")
	}
	if len(tasks) == 0 {
		t.Fatal("expected at least one task in JSON output")
	}
	task0 := tasks[0].(map[string]any)
	if task0["body"] != "This is a long task body that describes the full scope of the work to be done for building the widget." {
		t.Error("JSON should include task body")
	}
	if task0["last_failure_type"] != "timeout" {
		t.Error("JSON should include last_failure_type")
	}
	deliverables, ok := task0["deliverables"].([]any)
	if !ok || len(deliverables) != 3 {
		t.Errorf("JSON should include 3 deliverables, got %v", task0["deliverables"])
	}

	// Breadcrumbs at node level
	bcs, ok := proj["breadcrumbs"].([]any)
	if !ok || len(bcs) == 0 {
		t.Error("JSON should include breadcrumbs at node level")
	}
}

func TestStatusCmd_DetailFlag(t *testing.T) {
	env, _ := setupDetailEnv(t)
	env.RootCmd.SetArgs([]string{"status", "--detail"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("status --detail failed: %v", err)
	}
}

func TestStatusCmd_ExpandAndDetailFlags(t *testing.T) {
	env, _ := setupDetailEnv(t)
	env.RootCmd.SetArgs([]string{"status", "--expand", "--detail"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("status --expand --detail failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Archived node filtering
// ---------------------------------------------------------------------------

func setupArchivedEnv(t *testing.T) (*testEnv, *state.RootIndex) {
	t.Helper()
	env := newTestEnv(t)

	now := time.Now()
	archivedAt := now.Add(-48 * time.Hour)

	idx := state.NewRootIndex()
	idx.Root = []string{"active-proj"}
	idx.ArchivedRoot = []string{"old-proj"}

	idx.Nodes["active-proj"] = state.IndexEntry{
		Name:    "Active Project",
		Type:    state.NodeLeaf,
		State:   state.StatusInProgress,
		Address: "active-proj",
	}
	idx.Nodes["old-proj"] = state.IndexEntry{
		Name:       "Old Project",
		Type:       state.NodeLeaf,
		State:      state.StatusComplete,
		Address:    "old-proj",
		Archived:   true,
		ArchivedAt: &archivedAt,
	}

	// Write index and active node state
	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), idxData, 0644)

	nodeDir := filepath.Join(env.ProjectsDir, "active-proj")
	_ = os.MkdirAll(nodeDir, 0755)
	ns := state.NewNodeState("active-proj", "Active Project", state.NodeLeaf)
	ns.State = state.StatusInProgress
	nsData, _ := json.MarshalIndent(ns, "", "  ")
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), nsData, 0644)

	return env, idx
}

func TestShowTreeStatus_FiltersArchivedNodes(t *testing.T) {
	env, idx := setupArchivedEnv(t)
	out := captureStdout(t, func() {
		_ = showTreeStatus(env.App, idx, "")
	})

	if strings.Contains(out, "Old Project") {
		t.Error("archived node should not appear in default tree view")
	}
	if !strings.Contains(out, "Active Project") {
		t.Error("active node should appear in default tree view")
	}
}

func TestShowTreeStatus_ShowsArchivedSummaryLine(t *testing.T) {
	env, idx := setupArchivedEnv(t)
	out := captureStdout(t, func() {
		_ = showTreeStatus(env.App, idx, "")
	})

	if !strings.Contains(out, "1 archived node (use --archived to view)") {
		t.Errorf("expected archived summary line, got:\n%s", out)
	}
}

func TestShowTreeStatus_ArchivedCountInSummary(t *testing.T) {
	env, idx := setupArchivedEnv(t)
	out := captureStdout(t, func() {
		_ = showTreeStatus(env.App, idx, "")
	})

	if !strings.Contains(out, "1 archived") {
		t.Errorf("summary line should include archived count, got:\n%s", out)
	}
	// Total should be 2 (1 active + 1 archived)
	if !strings.Contains(out, "2 nodes") {
		t.Errorf("total should include archived nodes in count, got:\n%s", out)
	}
}

func TestShowTreeStatus_NoArchivedLine_WhenNoneArchived(t *testing.T) {
	env := newStatusTestEnv(t)
	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	out := captureStdout(t, func() {
		_ = showTreeStatus(env.App, idx, "")
	})

	if strings.Contains(out, "archived") {
		t.Error("should not show archived line when no nodes are archived")
	}
}

func TestShowTreeStatus_JSON_IncludesArchivedCount(t *testing.T) {
	env, idx := setupArchivedEnv(t)
	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	out := captureStdout(t, func() {
		_ = showTreeStatus(env.App, idx, "")
	})

	var resp map[string]any
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	data := resp["data"].(map[string]any)

	archived, ok := data["archived"].(float64)
	if !ok {
		t.Fatal("expected archived field in JSON data")
	}
	if archived != 1 {
		t.Errorf("expected archived=1, got %v", archived)
	}

	// Archived nodes should not appear in nodes map
	nodes := data["nodes"].(map[string]any)
	if _, ok := nodes["old-proj"]; ok {
		t.Error("archived node should not appear in JSON nodes map")
	}
}

func TestShowTreeStatus_PluralArchivedLine(t *testing.T) {
	env := newTestEnv(t)

	now := time.Now()
	archivedAt := now.Add(-48 * time.Hour)

	idx := state.NewRootIndex()
	idx.Root = []string{"active"}
	idx.ArchivedRoot = []string{"old-a", "old-b"}

	idx.Nodes["active"] = state.IndexEntry{
		Name: "Active", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "active",
	}
	idx.Nodes["old-a"] = state.IndexEntry{
		Name: "Old A", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "old-a", Archived: true, ArchivedAt: &archivedAt,
	}
	idx.Nodes["old-b"] = state.IndexEntry{
		Name: "Old B", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "old-b", Archived: true, ArchivedAt: &archivedAt,
	}

	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), idxData, 0644)

	nodeDir := filepath.Join(env.ProjectsDir, "active")
	_ = os.MkdirAll(nodeDir, 0755)
	ns := state.NewNodeState("active", "Active", state.NodeLeaf)
	ns.State = state.StatusInProgress
	nsData, _ := json.MarshalIndent(ns, "", "  ")
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), nsData, 0644)

	out := captureStdout(t, func() {
		_ = showTreeStatus(env.App, idx, "")
	})

	if !strings.Contains(out, "2 archived nodes (use --archived to view)") {
		t.Errorf("expected plural archived summary line, got:\n%s", out)
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in     string
		maxLen int
		want   string
	}{
		{"short", 80, "short"},
		{"exactly ten", 11, "exactly ten"},
		{"this is a longer string that should be truncated", 20, "this is a longer ..."},
		{"ab", 1, "a"},
		{"line one\nline two", 80, "line one line two"},
	}
	for _, tc := range cases {
		got := truncate(tc.in, tc.maxLen)
		if got != tc.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tc.in, tc.maxLen, got, tc.want)
		}
	}
}
