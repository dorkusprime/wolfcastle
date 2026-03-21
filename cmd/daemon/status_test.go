package daemon

import (
	"context"
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

	// Write archived node state with completion timestamp.
	completedAt := now.Add(-72 * time.Hour)
	archivedDir := filepath.Join(env.ProjectsDir, "old-proj")
	_ = os.MkdirAll(archivedDir, 0755)
	archivedNS := state.NewNodeState("old-proj", "Old Project", state.NodeLeaf)
	archivedNS.State = state.StatusComplete
	archivedNS.Audit.CompletedAt = &completedAt
	archivedNSData, _ := json.MarshalIndent(archivedNS, "", "  ")
	_ = os.WriteFile(filepath.Join(archivedDir, "state.json"), archivedNSData, 0644)

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

// ---------------------------------------------------------------------------
// --archived flag
// ---------------------------------------------------------------------------

func TestShowArchivedStatus_ShowsOnlyArchived(t *testing.T) {
	env, idx := setupArchivedEnv(t)
	out := captureStdout(t, func() {
		_ = showArchivedStatus(env.App, idx, "")
	})

	if !strings.Contains(out, "Old Project") {
		t.Error("--archived should show archived nodes")
	}
	if strings.Contains(out, "Active Project") {
		t.Error("--archived should not show active nodes")
	}
}

func TestShowArchivedStatus_ShowsArchivedAt(t *testing.T) {
	env, idx := setupArchivedEnv(t)
	out := captureStdout(t, func() {
		_ = showArchivedStatus(env.App, idx, "")
	})

	if !strings.Contains(out, "archived 20") {
		t.Errorf("--archived should show archive timestamp, got:\n%s", out)
	}
}

func TestShowArchivedStatus_ShowsAddress(t *testing.T) {
	env, idx := setupArchivedEnv(t)
	out := captureStdout(t, func() {
		_ = showArchivedStatus(env.App, idx, "")
	})

	if !strings.Contains(out, "old-proj") {
		t.Errorf("--archived should show original address, got:\n%s", out)
	}
}

func TestShowArchivedStatus_EmptyWhenNoneArchived(t *testing.T) {
	env := newStatusTestEnv(t)
	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	out := captureStdout(t, func() {
		_ = showArchivedStatus(env.App, idx, "")
	})

	if !strings.Contains(out, "No archived nodes") {
		t.Errorf("should show empty message when no archived nodes, got:\n%s", out)
	}
}

func TestShowArchivedStatus_JSON(t *testing.T) {
	env, idx := setupArchivedEnv(t)
	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	out := captureStdout(t, func() {
		_ = showArchivedStatus(env.App, idx, "")
	})

	var resp map[string]any
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	data := resp["data"].(map[string]any)

	total := data["total"].(float64)
	if total != 1 {
		t.Errorf("expected total=1, got %v", total)
	}

	nodes := data["nodes"].([]any)
	if len(nodes) != 1 {
		t.Fatalf("expected 1 archived node, got %d", len(nodes))
	}

	node := nodes[0].(map[string]any)
	if node["address"] != "old-proj" {
		t.Errorf("expected address=old-proj, got %v", node["address"])
	}
	if node["name"] != "Old Project" {
		t.Errorf("expected name=Old Project, got %v", node["name"])
	}
	if _, ok := node["archived_at"]; !ok {
		t.Error("JSON should include archived_at")
	}
	if _, ok := node["completed_at"]; !ok {
		t.Error("JSON should include completed_at")
	}
}

func TestShowArchivedStatus_RespectsScope(t *testing.T) {
	env := newTestEnv(t)
	now := time.Now()
	archivedAt := now.Add(-24 * time.Hour)

	idx := state.NewRootIndex()
	idx.Root = []string{"active"}
	idx.ArchivedRoot = []string{"old-a", "other-proj"}

	idx.Nodes["active"] = state.IndexEntry{
		Name: "Active", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "active",
	}
	idx.Nodes["old-a"] = state.IndexEntry{
		Name: "Old A", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "old-a", Archived: true, ArchivedAt: &archivedAt,
	}
	idx.Nodes["other-proj"] = state.IndexEntry{
		Name: "Other", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "other-proj", Archived: true, ArchivedAt: &archivedAt,
	}

	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), idxData, 0644)

	out := captureStdout(t, func() {
		_ = showArchivedStatus(env.App, idx, "old-a")
	})

	if !strings.Contains(out, "Old A") {
		t.Error("scoped --archived should show matching archived nodes")
	}
	if strings.Contains(out, "Other") {
		t.Error("scoped --archived should not show out-of-scope archived nodes")
	}
}

func TestStatusCmd_ArchivedFlag(t *testing.T) {
	env, _ := setupArchivedEnv(t)
	env.RootCmd.SetArgs([]string{"status", "--archived"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("status --archived failed: %v", err)
	}
}

// ---------------------------------------------------------------------------
// --all flag with archived nodes
// ---------------------------------------------------------------------------

func TestShowAllStatus_IncludesArchivedCount(t *testing.T) {
	env := newTestEnv(t)
	now := time.Now()
	archivedAt := now.Add(-48 * time.Hour)

	// Write an index with both active and archived nodes.
	idx := state.NewRootIndex()
	idx.Root = []string{"proj-a"}
	idx.ArchivedRoot = []string{"proj-b"}
	idx.Nodes["proj-a"] = state.IndexEntry{
		Name: "Project A", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "proj-a",
	}
	idx.Nodes["proj-b"] = state.IndexEntry{
		Name: "Project B", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "proj-b", Archived: true, ArchivedAt: &archivedAt,
	}
	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), idxData, 0644)

	out := captureStdout(t, func() {
		_ = showAllStatus(env.App)
	})

	if !strings.Contains(out, "1 archived") {
		t.Errorf("--all should show archived count, got:\n%s", out)
	}
	// Total should exclude archived nodes.
	if !strings.Contains(out, "1 nodes") {
		t.Errorf("--all total should exclude archived nodes, got:\n%s", out)
	}
}

func TestShowAllStatus_NoArchivedLine_WhenNoneArchived(t *testing.T) {
	env := newStatusTestEnv(t)
	out := captureStdout(t, func() {
		_ = showAllStatus(env.App)
	})

	if strings.Contains(out, "archived") {
		t.Error("--all should not show archived count when no nodes are archived")
	}
}

func TestShowAllStatus_JSON_IncludesArchivedField(t *testing.T) {
	env := newTestEnv(t)
	now := time.Now()
	archivedAt := now.Add(-48 * time.Hour)

	idx := state.NewRootIndex()
	idx.Root = []string{"proj-a"}
	idx.ArchivedRoot = []string{"proj-b"}
	idx.Nodes["proj-a"] = state.IndexEntry{
		Name: "Project A", Type: state.NodeLeaf, State: state.StatusComplete, Address: "proj-a",
	}
	idx.Nodes["proj-b"] = state.IndexEntry{
		Name: "Project B", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "proj-b", Archived: true, ArchivedAt: &archivedAt,
	}
	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), idxData, 0644)

	env.App.JSON = true
	defer func() { env.App.JSON = false }()

	out := captureStdout(t, func() {
		_ = showAllStatus(env.App)
	})

	var resp map[string]any
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("invalid JSON output: %v", err)
	}
	data := resp["data"].(map[string]any)
	namespaces := data["namespaces"].([]any)
	if len(namespaces) != 1 {
		t.Fatalf("expected 1 namespace, got %d", len(namespaces))
	}
	ns := namespaces[0].(map[string]any)
	archived := ns["archived"].(float64)
	if archived != 1 {
		t.Errorf("expected archived=1, got %v", archived)
	}
	total := ns["total"].(float64)
	if total != 1 {
		t.Errorf("expected total=1 (excluding archived), got %v", total)
	}
}

func TestShowAllStatus_MultipleNamespacesWithArchived(t *testing.T) {
	env := newTestEnv(t)
	now := time.Now()
	archivedAt := now.Add(-24 * time.Hour)

	// First namespace: has archived nodes
	idx1 := state.NewRootIndex()
	idx1.Root = []string{"active-1"}
	idx1.ArchivedRoot = []string{"old-1"}
	idx1.Nodes["active-1"] = state.IndexEntry{
		Name: "Active 1", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "active-1",
	}
	idx1.Nodes["old-1"] = state.IndexEntry{
		Name: "Old 1", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "old-1", Archived: true, ArchivedAt: &archivedAt,
	}
	data1, _ := json.MarshalIndent(idx1, "", "  ")
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), data1, 0644)

	// Second namespace: no archived nodes
	ns2Dir := filepath.Join(env.WolfcastleDir, "system", "projects", "other-eng")
	_ = os.MkdirAll(ns2Dir, 0755)
	idx2 := state.NewRootIndex()
	idx2.Root = []string{"proj-x"}
	idx2.Nodes["proj-x"] = state.IndexEntry{
		Name: "Project X", Type: state.NodeLeaf, State: state.StatusComplete, Address: "proj-x",
	}
	data2, _ := json.MarshalIndent(idx2, "", "  ")
	_ = os.WriteFile(filepath.Join(ns2Dir, "state.json"), data2, 0644)

	out := captureStdout(t, func() {
		_ = showAllStatus(env.App)
	})

	// First namespace should show archived count
	if !strings.Contains(out, "1 archived") {
		t.Errorf("namespace with archived nodes should show count, got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Edge cases: all nodes archived, mixed orchestrator hierarchy
// ---------------------------------------------------------------------------

func TestShowTreeStatus_AllNodesArchived(t *testing.T) {
	env := newTestEnv(t)
	now := time.Now()
	archivedAt := now.Add(-24 * time.Hour)

	idx := state.NewRootIndex()
	idx.ArchivedRoot = []string{"proj-a", "proj-b"}

	idx.Nodes["proj-a"] = state.IndexEntry{
		Name: "Project A", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "proj-a", Archived: true, ArchivedAt: &archivedAt,
	}
	idx.Nodes["proj-b"] = state.IndexEntry{
		Name: "Project B", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "proj-b", Archived: true, ArchivedAt: &archivedAt,
	}

	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), idxData, 0644)

	out := captureStdout(t, func() {
		_ = showTreeStatus(env.App, idx, "")
	})

	// No active nodes rendered in the tree
	if strings.Contains(out, "Project A") || strings.Contains(out, "Project B") {
		t.Error("archived nodes should not appear in default tree view")
	}
	// Summary should show all as archived
	if !strings.Contains(out, "2 archived nodes (use --archived to view)") {
		t.Errorf("expected plural archived footer, got:\n%s", out)
	}
	// Total line includes archived count
	if !strings.Contains(out, "2 nodes") {
		t.Errorf("total should count archived nodes, got:\n%s", out)
	}
}

func TestShowTreeStatus_MixedOrchestratorWithArchivedChildren(t *testing.T) {
	env := newTestEnv(t)
	now := time.Now()
	archivedAt := now.Add(-48 * time.Hour)

	idx := state.NewRootIndex()
	idx.Root = []string{"parent"}

	idx.Nodes["parent"] = state.IndexEntry{
		Name: "Parent", Type: state.NodeOrchestrator, State: state.StatusInProgress,
		Address: "parent", Children: []string{"parent/active-child", "parent/archived-child"},
	}
	idx.Nodes["parent/active-child"] = state.IndexEntry{
		Name: "Active Child", Type: state.NodeLeaf, State: state.StatusInProgress,
		Address: "parent/active-child", Parent: "parent",
	}
	idx.Nodes["parent/archived-child"] = state.IndexEntry{
		Name: "Archived Child", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "parent/archived-child", Parent: "parent", Archived: true, ArchivedAt: &archivedAt,
	}

	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), idxData, 0644)

	// Write node states
	for _, addr := range []string{"parent", "parent/active-child"} {
		nodeDir := filepath.Join(env.ProjectsDir, addr)
		_ = os.MkdirAll(nodeDir, 0755)
		ntype := state.NodeLeaf
		if addr == "parent" {
			ntype = state.NodeOrchestrator
		}
		ns := state.NewNodeState(addr, "Node", ntype)
		nsData, _ := json.MarshalIndent(ns, "", "  ")
		_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), nsData, 0644)
	}

	out := captureStdout(t, func() {
		_ = showTreeStatus(env.App, idx, "")
	})

	if !strings.Contains(out, "Active Child") {
		t.Error("active child should appear in tree")
	}
	if strings.Contains(out, "Archived Child") {
		t.Error("archived child should not appear in tree")
	}
	if !strings.Contains(out, "1 archived node") {
		t.Errorf("should show archived count footer, got:\n%s", out)
	}
}

func TestShowArchivedStatus_NoArchivedAt(t *testing.T) {
	env := newTestEnv(t)

	idx := state.NewRootIndex()
	idx.ArchivedRoot = []string{"proj-notime"}
	idx.Nodes["proj-notime"] = state.IndexEntry{
		Name: "No Timestamp", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "proj-notime", Archived: true, ArchivedAt: nil,
	}
	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), idxData, 0644)

	out := captureStdout(t, func() {
		_ = showArchivedStatus(env.App, idx, "")
	})

	if !strings.Contains(out, "No Timestamp") {
		t.Error("should render archived node even without timestamp")
	}
	// Should not show "archived <date>" line
	if strings.Contains(out, "archived 20") {
		t.Error("should not show archive date when ArchivedAt is nil")
	}
}

func TestShowArchivedStatus_ReadNodeFailsGracefully(t *testing.T) {
	env := newTestEnv(t)
	now := time.Now()
	archivedAt := now.Add(-24 * time.Hour)

	idx := state.NewRootIndex()
	idx.ArchivedRoot = []string{"gone-proj"}
	idx.Nodes["gone-proj"] = state.IndexEntry{
		Name: "Gone Project", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "gone-proj", Archived: true, ArchivedAt: &archivedAt,
	}
	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), idxData, 0644)

	// Don't create node state dir; ReadNode will fail
	env.App.JSON = true
	out := captureStdout(t, func() {
		_ = showArchivedStatus(env.App, idx, "")
	})

	var resp map[string]any
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	data := resp["data"].(map[string]any)
	nodes := data["nodes"].([]any)
	node := nodes[0].(map[string]any)

	// completed_at should be omitted when ReadNode fails
	if _, ok := node["completed_at"]; ok {
		t.Error("completed_at should be absent when node state can't be read")
	}
	if _, ok := node["archived_at"]; !ok {
		t.Error("archived_at should still be present")
	}
}

func TestShowArchivedStatus_JSON_EmptyList(t *testing.T) {
	env := newStatusTestEnv(t)
	idx, _ := state.LoadRootIndex(filepath.Join(env.ProjectsDir, "state.json"))
	env.App.JSON = true

	out := captureStdout(t, func() {
		_ = showArchivedStatus(env.App, idx, "")
	})

	var resp map[string]any
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	data := resp["data"].(map[string]any)
	total := data["total"].(float64)
	if total != 0 {
		t.Errorf("expected total=0, got %v", total)
	}
}

func TestShowArchivedStatus_MultipleNodesSortedByAddress(t *testing.T) {
	env := newTestEnv(t)
	now := time.Now()
	archivedAt := now.Add(-24 * time.Hour)

	idx := state.NewRootIndex()
	idx.ArchivedRoot = []string{"zebra-proj", "alpha-proj"}
	idx.Nodes["zebra-proj"] = state.IndexEntry{
		Name: "Zebra", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "zebra-proj", Archived: true, ArchivedAt: &archivedAt,
	}
	idx.Nodes["alpha-proj"] = state.IndexEntry{
		Name: "Alpha", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "alpha-proj", Archived: true, ArchivedAt: &archivedAt,
	}
	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), idxData, 0644)

	out := captureStdout(t, func() {
		_ = showArchivedStatus(env.App, idx, "")
	})

	alphaIdx := strings.Index(out, "Alpha")
	zebraIdx := strings.Index(out, "Zebra")
	if alphaIdx < 0 || zebraIdx < 0 {
		t.Fatalf("both nodes should appear, got:\n%s", out)
	}
	if alphaIdx > zebraIdx {
		t.Error("archived nodes should be sorted by address (alpha-proj before zebra-proj)")
	}
}

func TestShowTreeStatus_JSON_AllNodesArchived(t *testing.T) {
	env := newTestEnv(t)
	now := time.Now()
	archivedAt := now.Add(-24 * time.Hour)

	idx := state.NewRootIndex()
	idx.ArchivedRoot = []string{"only-proj"}
	idx.Nodes["only-proj"] = state.IndexEntry{
		Name: "Only Project", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "only-proj", Archived: true, ArchivedAt: &archivedAt,
	}

	env.App.JSON = true
	out := captureStdout(t, func() {
		_ = showTreeStatus(env.App, idx, "")
	})

	var resp map[string]any
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	data := resp["data"].(map[string]any)
	total := data["total"].(float64)
	archived := data["archived"].(float64)
	if total != 0 {
		t.Errorf("expected total=0 (no active nodes), got %v", total)
	}
	if archived != 1 {
		t.Errorf("expected archived=1, got %v", archived)
	}
	nodes := data["nodes"].(map[string]any)
	if len(nodes) != 0 {
		t.Error("JSON nodes map should be empty when all nodes are archived")
	}
}

func TestShowAllStatus_AllNodesArchived(t *testing.T) {
	env := newTestEnv(t)
	now := time.Now()
	archivedAt := now.Add(-24 * time.Hour)

	idx := state.NewRootIndex()
	idx.ArchivedRoot = []string{"dead-proj"}
	idx.Nodes["dead-proj"] = state.IndexEntry{
		Name: "Dead", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "dead-proj", Archived: true, ArchivedAt: &archivedAt,
	}
	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), idxData, 0644)

	out := captureStdout(t, func() {
		_ = showAllStatus(env.App)
	})

	if !strings.Contains(out, "0 nodes") {
		t.Errorf("total should be 0 when all nodes are archived, got:\n%s", out)
	}
	if !strings.Contains(out, "1 archived") {
		t.Errorf("should show archived count, got:\n%s", out)
	}
}

func TestShowTreeStatus_ScopeDoesNotCountOutOfScopeArchived(t *testing.T) {
	env := newTestEnv(t)
	now := time.Now()
	archivedAt := now.Add(-24 * time.Hour)

	idx := state.NewRootIndex()
	idx.Root = []string{"alpha", "beta"}
	idx.ArchivedRoot = []string{"gamma"}

	idx.Nodes["alpha"] = state.IndexEntry{
		Name: "Alpha", Type: state.NodeLeaf, State: state.StatusInProgress, Address: "alpha",
	}
	idx.Nodes["beta"] = state.IndexEntry{
		Name: "Beta", Type: state.NodeLeaf, State: state.StatusNotStarted, Address: "beta",
	}
	idx.Nodes["gamma"] = state.IndexEntry{
		Name: "Gamma", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "gamma", Archived: true, ArchivedAt: &archivedAt,
	}

	idxData, _ := json.MarshalIndent(idx, "", "  ")
	_ = os.WriteFile(filepath.Join(env.ProjectsDir, "state.json"), idxData, 0644)

	nodeDir := filepath.Join(env.ProjectsDir, "alpha")
	_ = os.MkdirAll(nodeDir, 0755)
	ns := state.NewNodeState("alpha", "Alpha", state.NodeLeaf)
	nsData, _ := json.MarshalIndent(ns, "", "  ")
	_ = os.WriteFile(filepath.Join(nodeDir, "state.json"), nsData, 0644)

	out := captureStdout(t, func() {
		_ = showTreeStatus(env.App, idx, "alpha")
	})

	// When scoped to alpha, the archived gamma should not contribute to archived count
	if strings.Contains(out, "archived") {
		t.Errorf("scoped view should not show out-of-scope archived nodes, got:\n%s", out)
	}
}

func TestShowArchivedStatus_ScopeToOrchestratorSubtree(t *testing.T) {
	env := newTestEnv(t)
	now := time.Now()
	archivedAt := now.Add(-24 * time.Hour)

	idx := state.NewRootIndex()
	idx.Root = []string{"parent"}
	idx.Nodes["parent"] = state.IndexEntry{
		Name: "Parent", Type: state.NodeOrchestrator, State: state.StatusInProgress,
		Address: "parent", Children: []string{"parent/child-archived"},
	}
	idx.Nodes["parent/child-archived"] = state.IndexEntry{
		Name: "Archived Child", Type: state.NodeLeaf, State: state.StatusComplete,
		Address: "parent/child-archived", Parent: "parent", Archived: true, ArchivedAt: &archivedAt,
	}

	out := captureStdout(t, func() {
		_ = showArchivedStatus(env.App, idx, "parent")
	})

	if !strings.Contains(out, "Archived Child") {
		t.Errorf("should show archived child when scoped to parent, got:\n%s", out)
	}
}

func TestStatusCmd_ArchivedWithScope(t *testing.T) {
	env, _ := setupArchivedEnv(t)
	env.RootCmd.SetArgs([]string{"status", "--archived", "--node", "old-proj"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("status --archived --node failed: %v", err)
	}
}

func TestStatusCmd_ArchivedJSON(t *testing.T) {
	env, _ := setupArchivedEnv(t)
	env.App.JSON = true
	env.RootCmd.SetArgs([]string{"status", "--archived"})
	if err := env.RootCmd.Execute(); err != nil {
		t.Fatalf("status --archived --json failed: %v", err)
	}
}

func TestWatchStatus_WithDetailFlag(t *testing.T) {
	env := newStatusTestEnv(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := watchStatus(ctx, env.App, "", false, 0.1, false, true); err != nil {
		t.Fatalf("watchStatus with detail cancelled: %v", err)
	}
}

func TestIsInSubtree_NestedHierarchy(t *testing.T) {
	idx := state.NewRootIndex()
	idx.Nodes["a"] = state.IndexEntry{Address: "a"}
	idx.Nodes["a/b"] = state.IndexEntry{Address: "a/b", Parent: "a"}
	idx.Nodes["a/b/c"] = state.IndexEntry{Address: "a/b/c", Parent: "a/b"}
	idx.Nodes["x"] = state.IndexEntry{Address: "x"}

	if !isInSubtree(idx, "a/b/c", "a") {
		t.Error("a/b/c should be in subtree of a")
	}
	if !isInSubtree(idx, "a/b", "a") {
		t.Error("a/b should be in subtree of a")
	}
	if !isInSubtree(idx, "a", "a") {
		t.Error("a should be in its own subtree")
	}
	if isInSubtree(idx, "x", "a") {
		t.Error("x should not be in subtree of a")
	}
	if isInSubtree(idx, "nonexistent", "a") {
		t.Error("nonexistent should not be in subtree of a")
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
